package server

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os/signal"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"connectrpc.com/connect"
	"connectrpc.com/validate"
	"github.com/rs/cors"

	"githooks/pkg/api"
	"githooks/pkg/auth"
	oidchelper "githooks/pkg/auth/oidc"
	"githooks/pkg/core"
	driverspkg "githooks/pkg/drivers"
	cloudv1connect "githooks/pkg/gen/cloud/v1/cloudv1connect"
	"githooks/pkg/oauth"
	"githooks/pkg/providerinstance"
	"githooks/pkg/storage"
	driversstore "githooks/pkg/storage/drivers"
	"githooks/pkg/storage/installations"
	"githooks/pkg/storage/namespaces"
	providerinstancestore "githooks/pkg/storage/provider_instances"
	"githooks/pkg/storage/rules"
	"githooks/pkg/webhook"

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

func bootstrapRules(ctx context.Context, store storage.RuleStore, engine *core.RuleEngine, configRules []core.Rule, strict bool, logger *log.Logger) error {
	if store == nil {
		return nil
	}
	tenantID := storage.TenantFromContext(ctx)
	records, err := store.ListRules(ctx)
	if err != nil {
		return err
	}
	if len(configRules) > 0 {
		normalizedConfig, err := core.NormalizeRules(configRules)
		if err != nil {
			return err
		}
		existing := make(map[string]struct{}, len(records))
		for _, record := range records {
			existing[ruleKey(record.When, record.Emit, record.Drivers)] = struct{}{}
		}
		added := 0
		for _, rule := range normalizedConfig {
			key := ruleKey(rule.When, rule.Emit.Values(), rule.Drivers)
			if _, ok := existing[key]; ok {
				continue
			}
			_, err := store.CreateRule(ctx, storage.RuleRecord{
				When:    rule.When,
				Emit:    rule.Emit.Values(),
				Drivers: rule.Drivers,
			})
			if err != nil {
				return err
			}
			added++
		}
		if logger != nil && added > 0 {
			logger.Printf("rules bootstrap: inserted %d rules from config", added)
		}
		records, err = store.ListRules(ctx)
		if err != nil {
			return err
		}
	}
	if len(records) == 0 {
		return nil
	}
	loaded := make([]core.Rule, 0, len(records))
	for _, record := range records {
		loaded = append(loaded, core.Rule{
			When:    record.When,
			Emit:    core.EmitList(record.Emit),
			Drivers: record.Drivers,
		})
	}
	normalized, err := core.NormalizeRules(loaded)
	if err != nil {
		return err
	}
	if logger != nil {
		logger.Printf("rules bootstrap: loaded %d rules from storage", len(normalized))
	}
	return engine.Update(core.RulesConfig{
		Rules:    normalized,
		Strict:   strict,
		TenantID: tenantID,
		Logger:   logger,
	})
}

func bootstrapDrivers(ctx context.Context, store storage.DriverStore, cfg core.WatermillConfig, logger *log.Logger) error {
	if store == nil {
		return nil
	}
	records, err := driverspkg.RecordsFromConfig(cfg)
	if err != nil {
		return err
	}
	if len(records) == 0 {
		return nil
	}
	upserted := 0
	for _, record := range records {
		if _, err := store.UpsertDriver(ctx, record); err != nil {
			return err
		}
		upserted++
	}
	if logger != nil && upserted > 0 {
		logger.Printf("drivers bootstrap: upserted %d drivers from config", upserted)
	}
	return nil
}

func bootstrapProviderInstances(ctx context.Context, store storage.ProviderInstanceStore, cfg auth.Config, logger *log.Logger) error {
	if store == nil {
		return nil
	}
	records, err := providerinstance.RecordsFromConfig(cfg)
	if err != nil {
		return err
	}
	if len(records) == 0 {
		return nil
	}
	upserted := 0
	for _, record := range records {
		if _, err := store.UpsertProviderInstance(ctx, record); err != nil {
			return err
		}
		upserted++
	}
	if logger != nil && upserted > 0 {
		logger.Printf("provider instances bootstrap: upserted %d instances from config", upserted)
	}
	return nil
}

func ruleKey(when string, emit []string, drivers []string) string {
	emitKey := normalizeRuleSlice(emit)
	driverKey := normalizeRuleSlice(drivers)
	return strings.TrimSpace(when) + "|" + emitKey + "|" + driverKey
}

func normalizeRuleSlice(values []string) string {
	clean := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		clean = append(clean, trimmed)
	}
	sort.Strings(clean)
	return strings.Join(clean, ",")
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
		installStore   *installations.Store
		namespaceStore *namespaces.Store
		ruleStore      *rules.Store
		driverStore    *driversstore.Store
		driverCache    *driverspkg.Cache
		instanceStore  *providerinstancestore.Store
		instanceCache  *providerinstance.Cache
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
		logger.Printf("storage enabled driver=%s dialect=%s table=githooks_installations", config.Storage.Driver, config.Storage.Dialect)

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
		logger.Printf("rules enabled driver=%s dialect=%s table=githooks_rules", config.Storage.Driver, config.Storage.Dialect)

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
		logger.Printf("drivers enabled driver=%s dialect=%s table=githooks_drivers", config.Storage.Driver, config.Storage.Dialect)

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
		logger.Printf("provider instances enabled driver=%s dialect=%s table=githooks_provider_instances", config.Storage.Driver, config.Storage.Dialect)
	} else {
		logger.Printf("storage disabled (missing storage.driver or storage.dsn)")
	}

	if ruleStore != nil {
		if err := bootstrapRules(ctx, ruleStore, ruleEngine, config.Rules, config.RulesStrict, logger); err != nil {
			return fmt.Errorf("rules bootstrap: %w", err)
		}
	}

	if driverStore != nil {
		if err := bootstrapDrivers(ctx, driverStore, config.Watermill, logger); err != nil {
			return fmt.Errorf("drivers bootstrap: %w", err)
		}
		driverCache = driverspkg.NewCache(driverStore, config.Watermill, logger)
		if err := driverCache.Refresh(ctx); err != nil {
			return fmt.Errorf("drivers cache: %w", err)
		}
	}

	if instanceStore != nil {
		if err := bootstrapProviderInstances(ctx, instanceStore, config.Providers, logger); err != nil {
			return fmt.Errorf("provider instances bootstrap: %w", err)
		}
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
	if config.Auth.OAuth2.Enabled {
		verifier, err := oidchelper.NewVerifier(ctx, config.Auth.OAuth2)
		if err != nil {
			return fmt.Errorf("oauth2 verifier: %w", err)
		}
		connectOpts = append(connectOpts, connect.WithInterceptors(newAuthInterceptor(verifier, logger)))
		authHandler := newOAuth2Handler(config.Auth.OAuth2, logger)
		mux.HandleFunc("/auth/login", authHandler.Login)
		mux.HandleFunc("/auth/callback", authHandler.Callback)
		logger.Printf("auth=enabled issuer=%s", config.Auth.OAuth2.Issuer)
	}
	connectOpts = append(connectOpts, connect.WithInterceptors(newTenantInterceptor()))
	mux.Handle("/", &oauth.StartHandler{
		Providers:     config.Providers,
		PublicBaseURL: config.Server.PublicBaseURL,
		Logger:        logger,
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
			Store:         namespaceStore,
			InstallStore:  installStore,
			Providers:     config.Providers,
			PublicBaseURL: config.Server.PublicBaseURL,
			Logger:        logger,
		}
		path, handler := cloudv1connect.NewNamespacesServiceHandler(namespaceSvc, connectOpts...)
		mux.Handle(path, handler)
	}
	{
		rulesSvc := &api.RulesService{
			Store:  ruleStore,
			Engine: ruleEngine,
			Strict: config.RulesStrict,
			Logger: logger,
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

	if config.Providers.GitHub.Enabled {
		ghHandler, err := webhook.NewGitHubHandler(
			config.Providers.GitHub.Webhook.Secret,
			ruleEngine,
			publisher,
			logger,
			config.Server.MaxBodyBytes,
			config.Server.DebugEvents,
			installStore,
			namespaceStore,
		)
		if err != nil {
			return fmt.Errorf("github handler: %w", err)
		}
		mux.Handle(config.Providers.GitHub.Webhook.Path, ghHandler)
		logger.Printf(
			"provider=github webhook=enabled path=%s oauth_callback=/oauth/github/callback app_id=%d",
			config.Providers.GitHub.Webhook.Path,
			config.Providers.GitHub.App.AppID,
		)
	}

	if config.Providers.GitLab.Enabled {
		glHandler, err := webhook.NewGitLabHandler(
			config.Providers.GitLab.Webhook.Secret,
			ruleEngine,
			publisher,
			logger,
			config.Server.MaxBodyBytes,
			config.Server.DebugEvents,
			namespaceStore,
		)
		if err != nil {
			return fmt.Errorf("gitlab handler: %w", err)
		}
		mux.Handle(config.Providers.GitLab.Webhook.Path, glHandler)
		logger.Printf(
			"provider=gitlab webhook=enabled path=%s oauth_callback=/oauth/gitlab/callback",
			config.Providers.GitLab.Webhook.Path,
		)
	}

	if config.Providers.Bitbucket.Enabled {
		bbHandler, err := webhook.NewBitbucketHandler(
			config.Providers.Bitbucket.Webhook.Secret,
			ruleEngine,
			publisher,
			logger,
			config.Server.MaxBodyBytes,
			config.Server.DebugEvents,
			namespaceStore,
		)
		if err != nil {
			return fmt.Errorf("bitbucket handler: %w", err)
		}
		mux.Handle(config.Providers.Bitbucket.Webhook.Path, bbHandler)
		logger.Printf(
			"provider=bitbucket webhook=enabled path=%s oauth_callback=/oauth/bitbucket/callback",
			config.Providers.Bitbucket.Webhook.Path,
		)
	}

	redirectBase := config.OAuth.RedirectBaseURL
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
			PublicBaseURL:         config.Server.PublicBaseURL,
		}
	}
	mux.Handle("/oauth/github/callback", oauthHandler("github", config.Providers.GitHub))
	mux.Handle("/oauth/gitlab/callback", oauthHandler("gitlab", config.Providers.GitLab))
	mux.Handle("/oauth/bitbucket/callback", oauthHandler("bitbucket", config.Providers.Bitbucket))

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
	if config.Server.PublicBaseURL != "" {
		logger.Printf("server public_base_url=%s", config.Server.PublicBaseURL)
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
