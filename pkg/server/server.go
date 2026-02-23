package server

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"connectrpc.com/connect"
	"connectrpc.com/validate"
	"github.com/rs/cors"

	"githook/pkg/api"
	oidchelper "githook/pkg/auth/oidc"
	"githook/pkg/core"
	driverspkg "githook/pkg/drivers"
	cloudv1connect "githook/pkg/gen/cloud/v1/cloudv1connect"
	"githook/pkg/oauth"
	"githook/pkg/providerinstance"
	"githook/pkg/storage"
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
	return RunWithMiddleware(ctx, config, logger)
}

// RunWithMiddleware starts the server with HTTP middleware applied to all handlers.
func RunWithMiddleware(ctx context.Context, config core.Config, logger *log.Logger, middlewares ...Middleware) error {
	if logger == nil {
		logger = core.NewLogger("server")
	}
	handler, cleanup, err := BuildHandler(ctx, config, logger, middlewares...)
	if err != nil {
		return err
	}
	defer cleanup()

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

// BuildHandler constructs the HTTP handler and returns a cleanup function.
func BuildHandler(ctx context.Context, config core.Config, logger *log.Logger, middlewares ...Middleware) (http.Handler, func(), error) {
	if logger == nil {
		logger = core.NewLogger("server")
	}
	var closers []func()
	addCloser := func(fn func()) {
		if fn != nil {
			closers = append(closers, fn)
		}
	}
	cleanup := func() {
		for i := len(closers) - 1; i >= 0; i-- {
			closers[i]()
		}
	}
	fail := func(err error) (http.Handler, func(), error) {
		cleanup()
		return nil, nil, err
	}

	ruleEngine, err := core.NewRuleEngine(core.RulesConfig{
		Rules:  config.Rules,
		Strict: config.RulesStrict,
		Logger: logger,
	})
	if err != nil {
		return fail(fmt.Errorf("compile rules: %w", err))
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
		pool := storage.PoolConfig{
			MaxOpenConns:      config.Storage.MaxOpenConns,
			MaxIdleConns:      config.Storage.MaxIdleConns,
			ConnMaxLifetimeMS: config.Storage.ConnMaxLifetimeMS,
			ConnMaxIdleTimeMS: config.Storage.ConnMaxIdleTimeMS,
		}
		store, err := installations.Open(installations.Config{
			Driver:      config.Storage.Driver,
			DSN:         config.Storage.DSN,
			Dialect:     config.Storage.Dialect,
			AutoMigrate: config.Storage.AutoMigrate,
			Pool:        pool,
		})
		if err != nil {
			return fail(fmt.Errorf("storage: %w", err))
		}
		installStore = store
		addCloser(func() { _ = installStore.Close() })
		logger.Printf("storage enabled driver=%s dialect=%s table=githook_installations", config.Storage.Driver, config.Storage.Dialect)

		nsStore, err := namespaces.Open(namespaces.Config{
			Driver:      config.Storage.Driver,
			DSN:         config.Storage.DSN,
			Dialect:     config.Storage.Dialect,
			AutoMigrate: config.Storage.AutoMigrate,
			Pool:        pool,
		})
		if err != nil {
			return fail(fmt.Errorf("namespaces storage: %w", err))
		}
		namespaceStore = nsStore
		addCloser(func() { _ = namespaceStore.Close() })
		logger.Printf("namespaces enabled driver=%s dialect=%s table=git_namespaces", config.Storage.Driver, config.Storage.Dialect)

		rsStore, err := rules.Open(rules.Config{
			Driver:      config.Storage.Driver,
			DSN:         config.Storage.DSN,
			Dialect:     config.Storage.Dialect,
			AutoMigrate: config.Storage.AutoMigrate,
			Pool:        pool,
		})
		if err != nil {
			return fail(fmt.Errorf("rules storage: %w", err))
		}
		ruleStore = rsStore
		addCloser(func() { _ = ruleStore.Close() })
		logger.Printf("rules enabled driver=%s dialect=%s table=githook_rules", config.Storage.Driver, config.Storage.Dialect)

		elStore, err := eventlogs.Open(eventlogs.Config{
			Driver:      config.Storage.Driver,
			DSN:         config.Storage.DSN,
			Dialect:     config.Storage.Dialect,
			AutoMigrate: config.Storage.AutoMigrate,
			Pool:        pool,
		})
		if err != nil {
			return fail(fmt.Errorf("event logs storage: %w", err))
		}
		logStore = elStore
		addCloser(func() { _ = logStore.Close() })
		logger.Printf("event logs enabled driver=%s dialect=%s table=githook_event_logs", config.Storage.Driver, config.Storage.Dialect)

		dsStore, err := driversstore.Open(driversstore.Config{
			Driver:      config.Storage.Driver,
			DSN:         config.Storage.DSN,
			Dialect:     config.Storage.Dialect,
			AutoMigrate: config.Storage.AutoMigrate,
			Pool:        pool,
		})
		if err != nil {
			return fail(fmt.Errorf("driver storage: %w", err))
		}
		driverStore = dsStore
		addCloser(func() { _ = driverStore.Close() })
		logger.Printf("drivers enabled driver=%s dialect=%s table=githook_drivers", config.Storage.Driver, config.Storage.Dialect)

		piStore, err := providerinstancestore.Open(providerinstancestore.Config{
			Driver:      config.Storage.Driver,
			DSN:         config.Storage.DSN,
			Dialect:     config.Storage.Dialect,
			AutoMigrate: config.Storage.AutoMigrate,
			Pool:        pool,
		})
		if err != nil {
			return fail(fmt.Errorf("provider instances storage: %w", err))
		}
		instanceStore = piStore
		addCloser(func() { _ = instanceStore.Close() })
		logger.Printf("provider instances enabled driver=%s dialect=%s table=githook_provider_instances", config.Storage.Driver, config.Storage.Dialect)
	} else {
		logger.Printf("storage disabled (missing storage.driver or storage.dsn)")
	}

	dynamicDriverCache = driverspkg.NewDynamicPublisherCache()
	addCloser(func() { _ = dynamicDriverCache.Close() })

	if driverStore != nil {
		driverCache = driverspkg.NewCache(driverStore, config.Relaybus, logger)
		if err := driverCache.Refresh(ctx); err != nil {
			return fail(fmt.Errorf("drivers cache: %w", err))
		}
		addCloser(driverCache.Close)
	}

	if instanceStore != nil {
		instanceCache = providerinstance.NewCache(instanceStore, logger)
		if err := instanceCache.Refresh(ctx); err != nil {
			return fail(fmt.Errorf("provider instances cache: %w", err))
		}
		addCloser(instanceCache.Close)
	}

	hasBaseDrivers := strings.TrimSpace(config.Relaybus.Driver) != "" || len(config.Relaybus.Drivers) > 0
	var basePublisher core.Publisher
	if hasBaseDrivers {
		var err error
		basePublisher, err = core.NewPublisher(config.Relaybus)
		if err != nil {
			return fail(fmt.Errorf("publisher: %w", err))
		}
	}

	var publisher core.Publisher
	switch {
	case driverCache != nil:
		publisher = driverspkg.NewTenantPublisher(driverCache, basePublisher)
	case basePublisher != nil:
		publisher = basePublisher
	default:
		return fail(errors.New("relaybus publisher not configured"))
	}
	addCloser(func() { _ = publisher.Close() })

	mux := http.NewServeMux()
	validationInterceptor := validate.NewInterceptor()
	connectOpts := []connect.HandlerOption{
		connect.WithInterceptors(validationInterceptor),
	}
	var verifier *oidchelper.Verifier
	if config.Auth.OAuth2.Enabled {
		created, err := oidchelper.NewVerifier(ctx, config.Auth.OAuth2)
		if err != nil {
			return fail(fmt.Errorf("oauth2 verifier: %w", err))
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
	webhookRegistry := webhook.DefaultRegistry()
	oauthRegistry := oauth.DefaultRegistry()
	mux.Handle("/", &oauth.StartHandler{
		Providers:             config.Providers,
		Endpoint:              config.Endpoint,
		Logger:                logger,
		ProviderInstanceStore: instanceStore,
		ProviderInstanceCache: instanceCache,
		Registry:              oauthRegistry,
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
		scmSvc := &api.SCMService{
			Store:                 installStore,
			ProviderInstanceStore: instanceStore,
			ProviderInstanceCache: instanceCache,
			Providers:             config.Providers,
			Logger:                logger,
		}
		path, handler := cloudv1connect.NewSCMServiceHandler(scmSvc, connectOpts...)
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

	webhookOpts := webhook.HandlerOptions{
		Rules:              ruleEngine,
		Publisher:          publisher,
		Logger:             logger,
		MaxBodyBytes:       config.Server.MaxBodyBytes,
		DebugEvents:        config.Server.DebugEvents,
		InstallStore:       installStore,
		NamespaceStore:     namespaceStore,
		EventLogStore:      logStore,
		RuleStore:          ruleStore,
		DriverStore:        driverStore,
		RulesStrict:        config.RulesStrict,
		DynamicDriverCache: dynamicDriverCache,
	}

	for _, provider := range webhookRegistry.Providers() {
		providerCfg, ok := config.Providers.ProviderConfigFor(provider.Name())
		if !ok {
			continue
		}
		handler, err := provider.NewHandler(providerCfg, webhookOpts)
		if err != nil {
			return fail(fmt.Errorf("%s handler: %w", provider.Name(), err))
		}
		path := provider.WebhookPath(providerCfg)
		mux.Handle(path, handler)
		oauthCallback := ""
		if oauthProvider, ok := oauthRegistry.Provider(provider.Name()); ok {
			oauthCallback = oauthProvider.CallbackPath()
		}
		extra := strings.TrimSpace(provider.WebhookLogFields(providerCfg))
		if extra != "" {
			extra = " " + extra
		}
		logger.Printf("provider=%s webhook=enabled path=%s oauth_callback=%s%s", provider.Name(), path, oauthCallback, extra)
	}

	redirectBase := config.RedirectBaseURL
	oauthOpts := oauth.HandlerOptions{
		Providers:             config.Providers,
		Store:                 installStore,
		NamespaceStore:        namespaceStore,
		ProviderInstanceStore: instanceStore,
		ProviderInstanceCache: instanceCache,
		Logger:                logger,
		RedirectBase:          redirectBase,
		Endpoint:              config.Endpoint,
	}
	for _, provider := range oauthRegistry.Providers() {
		providerCfg, ok := config.Providers.ProviderConfigFor(provider.Name())
		if !ok {
			continue
		}
		mux.Handle(provider.CallbackPath(), provider.NewHandler(providerCfg, oauthOpts))
	}

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
	appHandler := applyMiddlewares(mux, middlewares)
	handler := h2c.NewHandler(corsHandler.Handler(appHandler), &http2.Server{})

	return handler, cleanup, nil
}
