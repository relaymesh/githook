package server

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"connectrpc.com/connect"
	"connectrpc.com/validate"
	"github.com/rs/cors"

	"githook/pkg/api"
	"githook/pkg/auth"
	oidchelper "githook/pkg/auth/oidc"
	"githook/pkg/core"
	driverspkg "githook/pkg/drivers"
	cloudv1connect "githook/pkg/gen/cloud/v1/cloudv1connect"
	"githook/pkg/oauth"
	"githook/pkg/providerinstance"
	driversstore "githook/pkg/storage/drivers"
	"githook/pkg/storage/eventlogs"
	"githook/pkg/storage/installations"
	"githook/pkg/storage/namespaces"
	providerinstancestore "githook/pkg/storage/provider_instances"
	"githook/pkg/storage/rules"
	"githook/pkg/webhook"

	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

// RunConfig loads config from a path and starts the server with signal handling.
func RunConfig(configPath string) error {
	logger := core.NewLogger("server")
	config, err := core.LoadConfig(configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	return Run(ctx, config, logger)
}

// Run starts the server until the context is canceled.
func Run(ctx context.Context, config core.Config, logger *log.Logger) error {
	ruleEngine, err := core.NewRuleEngine(core.RulesConfig{
		Rules:  config.Rules,
		Strict: config.RulesStrict,
		Logger: logger,
	})
	if err != nil {
		return fmt.Errorf("compile rules: %w", err)
	}

	var (
		installStore       *installations.Store
		namespaceStore     *namespaces.Store
		ruleStore          *rules.Store
		logStore           *eventlogs.Store
		driverStore        *driversstore.Store
		driverCache        *driverspkg.Cache
		instanceStore      *providerinstancestore.Store
		instanceCache      *providerinstance.Cache
		dynamicDriverCache *driverspkg.DynamicPublisherCache
	)
	if config.Storage.Driver != "" && config.Storage.DSN != "" {
		store, err := installations.Open(installations.Config{
			Driver:      config.Storage.Driver,
			DSN:         config.Storage.DSN,
			Dialect:     config.Storage.Dialect,
			AutoMigrate: config.Storage.AutoMigrate,
		})
		if err != nil {
			return fmt.Errorf("storage: %w", err)
		}
		installStore = store
		defer installStore.Close()
		logger.Printf("storage enabled driver=%s dialect=%s table=githook_installations", config.Storage.Driver, config.Storage.Dialect)

		nsStore, err := namespaces.Open(namespaces.Config{
			Driver:      config.Storage.Driver,
			DSN:         config.Storage.DSN,
			Dialect:     config.Storage.Dialect,
			AutoMigrate: config.Storage.AutoMigrate,
		})
		if err != nil {
			return fmt.Errorf("namespaces storage: %w", err)
		}
		namespaceStore = nsStore
		defer namespaceStore.Close()
		logger.Printf("namespaces enabled driver=%s dialect=%s table=git_namespaces", config.Storage.Driver, config.Storage.Dialect)

		rsStore, err := rules.Open(rules.Config{
			Driver:      config.Storage.Driver,
			DSN:         config.Storage.DSN,
			Dialect:     config.Storage.Dialect,
			AutoMigrate: config.Storage.AutoMigrate,
		})
		if err != nil {
			return fmt.Errorf("rules storage: %w", err)
		}
		ruleStore = rsStore
		defer ruleStore.Close()
		logger.Printf("rules enabled driver=%s dialect=%s table=githook_rules", config.Storage.Driver, config.Storage.Dialect)

		elStore, err := eventlogs.Open(eventlogs.Config{
			Driver:      config.Storage.Driver,
			DSN:         config.Storage.DSN,
			Dialect:     config.Storage.Dialect,
			AutoMigrate: config.Storage.AutoMigrate,
		})
		if err != nil {
			return fmt.Errorf("event logs storage: %w", err)
		}
		logStore = elStore
		defer logStore.Close()
		logger.Printf("event logs enabled driver=%s dialect=%s table=githook_event_logs", config.Storage.Driver, config.Storage.Dialect)

		dsStore, err := driversstore.Open(driversstore.Config{
			Driver:      config.Storage.Driver,
			DSN:         config.Storage.DSN,
			Dialect:     config.Storage.Dialect,
			AutoMigrate: config.Storage.AutoMigrate,
		})
		if err != nil {
			return fmt.Errorf("drivers storage: %w", err)
		}
		driverStore = dsStore
		defer driverStore.Close()
		logger.Printf("drivers enabled driver=%s dialect=%s table=githook_drivers", config.Storage.Driver, config.Storage.Dialect)

		piStore, err := providerinstancestore.Open(providerinstancestore.Config{
			Driver:      config.Storage.Driver,
			DSN:         config.Storage.DSN,
			Dialect:     config.Storage.Dialect,
			AutoMigrate: config.Storage.AutoMigrate,
		})
		if err != nil {
			return fmt.Errorf("provider instances storage: %w", err)
		}
		instanceStore = piStore
		defer instanceStore.Close()
		logger.Printf("provider instances enabled driver=%s dialect=%s table=githook_provider_instances", config.Storage.Driver, config.Storage.Dialect)
	} else {
		logger.Printf("storage disabled (missing storage.driver or storage.dsn)")
	}

	dynamicDriverCache = driverspkg.NewDynamicPublisherCache()
	defer dynamicDriverCache.Close()

	if driverStore != nil {
		driverCache = driverspkg.NewCache(driverStore, config.Watermill, logger)
		if err := driverCache.Refresh(ctx); err != nil {
			return fmt.Errorf("drivers cache: %w", err)
		}
	}

	if instanceStore != nil {
		instanceCache = providerinstance.NewCache(instanceStore, logger)
		if err := instanceCache.Refresh(ctx); err != nil {
			return fmt.Errorf("provider instances cache: %w", err)
		}
	}

	basePublisher, err := core.NewPublisher(config.Watermill)
	if err != nil {
		return fmt.Errorf("publisher: %w", err)
	}
	publisher := core.Publisher(basePublisher)
	if driverCache != nil {
		publisher = driverspkg.NewTenantPublisher(driverCache, basePublisher)
	}
	defer publisher.Close()

	mux := http.NewServeMux()
	validationInterceptor := validate.NewInterceptor()
	connectOpts := []connect.HandlerOption{
		connect.WithInterceptors(validationInterceptor),
	}
	var verifier *oidchelper.Verifier
	if config.Auth.OAuth2.Enabled {
		created, err := oidchelper.NewVerifier(ctx, config.Auth.OAuth2)
		if err != nil {
			return fmt.Errorf("oauth2 verifier: %w", err)
		}
		verifier = created
		authHandler := newOAuth2Handler(config.Auth.OAuth2, logger)
		mux.HandleFunc("/auth/login", authHandler.Login)
		mux.HandleFunc("/auth/callback", authHandler.Callback)
		logger.Printf("auth=oauth2 enabled issuer=%s", config.Auth.OAuth2.Issuer)
	}
	if verifier != nil {
		connectOpts = append(connectOpts, connect.WithInterceptors(newAuthInterceptor(verifier, logger)))
	}
	connectOpts = append(connectOpts, connect.WithInterceptors(newTenantInterceptor()))
	mux.Handle("/", &oauth.StartHandler{
		Providers:             config.Providers,
		Endpoint:              config.Endpoint,
		Logger:                logger,
		ProviderInstanceStore: instanceStore,
		ProviderInstanceCache: instanceCache,
	})
	{
		installSvc := &api.InstallationsService{
			Store:     installStore,
			Providers: config.Providers,
			Logger:    logger,
		}
		path, handler := cloudv1connect.NewInstallationsServiceHandler(installSvc, connectOpts...)
		mux.Handle(path, handler)
	}
	{
		namespaceSvc := &api.NamespacesService{
			Store:                 namespaceStore,
			InstallStore:          installStore,
			ProviderInstanceStore: instanceStore,
			ProviderInstanceCache: instanceCache,
			Providers:             config.Providers,
			Endpoint:              config.Endpoint,
			Logger:                logger,
		}
		path, handler := cloudv1connect.NewNamespacesServiceHandler(namespaceSvc, connectOpts...)
		mux.Handle(path, handler)
	}
	{
		rulesSvc := &api.RulesService{
			Store:       ruleStore,
			DriverStore: driverStore,
			Engine:      ruleEngine,
			Strict:      config.RulesStrict,
			Logger:      logger,
		}
		path, handler := cloudv1connect.NewRulesServiceHandler(rulesSvc, connectOpts...)
		mux.Handle(path, handler)
	}
	{
		driversSvc := &api.DriversService{
			Store:  driverStore,
			Cache:  driverCache,
			Logger: logger,
		}
		path, handler := cloudv1connect.NewDriversServiceHandler(driversSvc, connectOpts...)
		mux.Handle(path, handler)
	}
	{
		providerSvc := &api.ProvidersService{
			Store:  instanceStore,
			Cache:  instanceCache,
			Logger: logger,
		}
		path, handler := cloudv1connect.NewProvidersServiceHandler(providerSvc, connectOpts...)
		mux.Handle(path, handler)
	}
	{
		eventLogSvc := &api.EventLogsService{
			Store:  logStore,
			Logger: logger,
		}
		path, handler := cloudv1connect.NewEventLogsServiceHandler(eventLogSvc, connectOpts...)
		mux.Handle(path, handler)
	}

	ghHandler, err := webhook.NewGitHubHandler(
		config.Providers.GitHub.Webhook.Secret,
		ruleEngine,
		publisher,
		logger,
		config.Server.MaxBodyBytes,
		config.Server.DebugEvents,
		installStore,
		namespaceStore,
		logStore,
		ruleStore,
		driverStore,
		config.RulesStrict,
		dynamicDriverCache,
	)
	if err != nil {
		return fmt.Errorf("github handler: %w", err)
	}
	mux.Handle(config.Providers.GitHub.Webhook.Path, ghHandler)
	logger.Printf(
		"provider=github webhook=enabled path=%s oauth_callback=/auth/github/callback app_id=%d",
		config.Providers.GitHub.Webhook.Path,
		config.Providers.GitHub.App.AppID,
	)

	glHandler, err := webhook.NewGitLabHandler(
		config.Providers.GitLab.Webhook.Secret,
		ruleEngine,
		publisher,
		logger,
		config.Server.MaxBodyBytes,
		config.Server.DebugEvents,
		namespaceStore,
		logStore,
		ruleStore,
		driverStore,
		config.RulesStrict,
		dynamicDriverCache,
	)
	if err != nil {
		return fmt.Errorf("gitlab handler: %w", err)
	}
	mux.Handle(config.Providers.GitLab.Webhook.Path, glHandler)
	logger.Printf(
		"provider=gitlab webhook=enabled path=%s oauth_callback=/auth/gitlab/callback",
		config.Providers.GitLab.Webhook.Path,
	)

	bbHandler, err := webhook.NewBitbucketHandler(
		config.Providers.Bitbucket.Webhook.Secret,
		ruleEngine,
		publisher,
		logger,
		config.Server.MaxBodyBytes,
		config.Server.DebugEvents,
		namespaceStore,
		logStore,
		ruleStore,
		driverStore,
		config.RulesStrict,
		dynamicDriverCache,
	)
	if err != nil {
		return fmt.Errorf("bitbucket handler: %w", err)
	}
	mux.Handle(config.Providers.Bitbucket.Webhook.Path, bbHandler)
	logger.Printf(
		"provider=bitbucket webhook=enabled path=%s oauth_callback=/auth/bitbucket/callback",
		config.Providers.Bitbucket.Webhook.Path,
	)

	redirectBase := config.RedirectBaseURL
	oauthHandler := func(provider string, cfg auth.ProviderConfig) *oauth.Handler {
		return &oauth.Handler{
			Provider:              provider,
			Config:                cfg,
			Providers:             config.Providers,
			Store:                 installStore,
			NamespaceStore:        namespaceStore,
			ProviderInstanceStore: instanceStore,
			ProviderInstanceCache: instanceCache,
			Logger:                logger,
			RedirectBase:          redirectBase,
			Endpoint:              config.Endpoint,
		}
	}
	mux.Handle("/auth/github/callback", oauthHandler("github", config.Providers.GitHub))
	mux.Handle("/auth/gitlab/callback", oauthHandler("gitlab", config.Providers.GitLab))
	mux.Handle("/auth/bitbucket/callback", oauthHandler("bitbucket", config.Providers.Bitbucket))

	corsHandler := cors.New(cors.Options{
		AllowedMethods: []string{
			http.MethodHead,
			http.MethodGet,
			http.MethodPost,
			http.MethodPut,
			http.MethodPatch,
			http.MethodDelete,
		},
		AllowOriginFunc: func(_ string) bool { return true },
		AllowedHeaders:  []string{"*"},
		ExposedHeaders: []string{
			"Accept",
			"Accept-Encoding",
			"Accept-Post",
			"Connect-Accept-Encoding",
			"Connect-Content-Encoding",
			"Content-Encoding",
			"Grpc-Accept-Encoding",
			"Grpc-Encoding",
			"Grpc-Message",
			"Grpc-Status",
			"Grpc-Status-Details-Bin",
		},
		MaxAge: int(2 * time.Hour / time.Second),
	})
	handler := h2c.NewHandler(corsHandler.Handler(mux), &http2.Server{})

	addr := ":" + strconv.Itoa(config.Server.Port)
	if config.Endpoint != "" {
		logger.Printf("server endpoint=%s", config.Endpoint)
	}
	server := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadTimeout:       time.Duration(config.Server.ReadTimeoutMS) * time.Millisecond,
		WriteTimeout:      time.Duration(config.Server.WriteTimeoutMS) * time.Millisecond,
		IdleTimeout:       time.Duration(config.Server.IdleTimeoutMS) * time.Millisecond,
		ReadHeaderTimeout: time.Duration(config.Server.ReadHeaderMS) * time.Millisecond,
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Printf("listening on %s", addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			logger.Printf("shutdown: %v", err)
		}
		return nil
	case err := <-errCh:
		if err != nil {
			return fmt.Errorf("listen: %w", err)
		}
		return nil
	}
}
