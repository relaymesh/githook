package server

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"connectrpc.com/connect"
	"connectrpc.com/validate"
	"github.com/rs/cors"

	"github.com/relaymesh/githook/pkg/api"
	oidchelper "github.com/relaymesh/githook/pkg/auth/oidc"
	"github.com/relaymesh/githook/pkg/core"
	driverspkg "github.com/relaymesh/githook/pkg/drivers"
	cloudv1connect "github.com/relaymesh/githook/pkg/gen/cloud/v1/cloudv1connect"
	"github.com/relaymesh/githook/pkg/oauth"
	"github.com/relaymesh/githook/pkg/providerinstance"
	"github.com/relaymesh/githook/pkg/storage"
	driversstore "github.com/relaymesh/githook/pkg/storage/drivers"
	"github.com/relaymesh/githook/pkg/storage/eventlogs"
	"github.com/relaymesh/githook/pkg/storage/installations"
	"github.com/relaymesh/githook/pkg/storage/namespaces"
	providerinstancestore "github.com/relaymesh/githook/pkg/storage/provider_instances"
	"github.com/relaymesh/githook/pkg/storage/rules"
	"github.com/relaymesh/githook/pkg/webhook"

	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

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

	stores, err := openStores(config, logger, addCloser)
	if err != nil {
		return fail(err)
	}

	caches, err := buildCaches(ctx, stores, config, logger, addCloser)
	if err != nil {
		return fail(err)
	}

	publisher, err := buildPublisher(config, caches.driverCache)
	if err != nil {
		return fail(err)
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
		ProviderInstanceStore: stores.instanceStore,
		ProviderInstanceCache: caches.instanceCache,
		Registry:              oauthRegistry,
	})
	{
		installSvc := &api.InstallationsService{
			Store:     stores.installStore,
			Providers: config.Providers,
			Logger:    logger,
		}
		path, handler := cloudv1connect.NewInstallationsServiceHandler(installSvc, connectOpts...)
		mux.Handle(path, handler)
	}
	{
		namespaceSvc := &api.NamespacesService{
			Store:                 stores.namespaceStore,
			InstallStore:          stores.installStore,
			ProviderInstanceStore: stores.instanceStore,
			ProviderInstanceCache: caches.instanceCache,
			Providers:             config.Providers,
			Endpoint:              config.Endpoint,
			Logger:                logger,
		}
		path, handler := cloudv1connect.NewNamespacesServiceHandler(namespaceSvc, connectOpts...)
		mux.Handle(path, handler)
	}
	{
		rulesSvc := &api.RulesService{
			Store:       stores.ruleStore,
			DriverStore: stores.driverStore,
			Engine:      ruleEngine,
			Strict:      config.RulesStrict,
			Logger:      logger,
		}
		path, handler := cloudv1connect.NewRulesServiceHandler(rulesSvc, connectOpts...)
		mux.Handle(path, handler)
	}
	{
		driversSvc := &api.DriversService{
			Store:  stores.driverStore,
			Cache:  caches.driverCache,
			Logger: logger,
		}
		path, handler := cloudv1connect.NewDriversServiceHandler(driversSvc, connectOpts...)
		mux.Handle(path, handler)
	}
	{
		providerSvc := &api.ProvidersService{
			Store:  stores.instanceStore,
			Cache:  caches.instanceCache,
			Logger: logger,
		}
		path, handler := cloudv1connect.NewProvidersServiceHandler(providerSvc, connectOpts...)
		mux.Handle(path, handler)
	}
	{
		scmSvc := &api.SCMService{
			Store:                 stores.installStore,
			ProviderInstanceStore: stores.instanceStore,
			ProviderInstanceCache: caches.instanceCache,
			Providers:             config.Providers,
			Logger:                logger,
		}
		path, handler := cloudv1connect.NewSCMServiceHandler(scmSvc, connectOpts...)
		mux.Handle(path, handler)
	}
	{
		eventLogSvc := &api.EventLogsService{
			Store:  stores.logStore,
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
		InstallStore:       stores.installStore,
		NamespaceStore:     stores.namespaceStore,
		EventLogStore:      stores.logStore,
		RuleStore:          stores.ruleStore,
		DriverStore:        stores.driverStore,
		RulesStrict:        config.RulesStrict,
		DynamicDriverCache: caches.dynamicDriverCache,
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
		Store:                 stores.installStore,
		NamespaceStore:        stores.namespaceStore,
		ProviderInstanceStore: stores.instanceStore,
		ProviderInstanceCache: caches.instanceCache,
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

type serverStores struct {
	installStore   *installations.Store
	namespaceStore *namespaces.Store
	ruleStore      *rules.Store
	logStore       *eventlogs.Store
	driverStore    *driversstore.Store
	instanceStore  *providerinstancestore.Store
}

type serverCaches struct {
	driverCache        *driverspkg.Cache
	instanceCache      *providerinstance.Cache
	dynamicDriverCache *driverspkg.DynamicPublisherCache
}

func openStores(cfg core.Config, logger *log.Logger, addCloser func(func())) (serverStores, error) {
	var stores serverStores
	if cfg.Storage.Driver == "" || cfg.Storage.DSN == "" {
		logger.Printf("storage disabled (missing storage.driver or storage.dsn)")
		return stores, nil
	}

	pool := storage.PoolConfig{
		MaxOpenConns:      cfg.Storage.MaxOpenConns,
		MaxIdleConns:      cfg.Storage.MaxIdleConns,
		ConnMaxLifetimeMS: cfg.Storage.ConnMaxLifetimeMS,
		ConnMaxIdleTimeMS: cfg.Storage.ConnMaxIdleTimeMS,
	}
	installStore, err := installations.Open(installations.Config{
		Driver:      cfg.Storage.Driver,
		DSN:         cfg.Storage.DSN,
		Dialect:     cfg.Storage.Dialect,
		AutoMigrate: cfg.Storage.AutoMigrate,
		Pool:        pool,
	})
	if err != nil {
		return stores, fmt.Errorf("storage: %w", err)
	}
	stores.installStore = installStore
	addCloser(func() { _ = installStore.Close() })
	logger.Printf("storage enabled driver=%s dialect=%s table=githook_installations", cfg.Storage.Driver, cfg.Storage.Dialect)

	namespaceStore, err := namespaces.Open(namespaces.Config{
		Driver:      cfg.Storage.Driver,
		DSN:         cfg.Storage.DSN,
		Dialect:     cfg.Storage.Dialect,
		AutoMigrate: cfg.Storage.AutoMigrate,
		Pool:        pool,
	})
	if err != nil {
		return stores, fmt.Errorf("namespaces storage: %w", err)
	}
	stores.namespaceStore = namespaceStore
	addCloser(func() { _ = namespaceStore.Close() })
	logger.Printf("namespaces enabled driver=%s dialect=%s table=%s", cfg.Storage.Driver, cfg.Storage.Dialect, namespaceStore.TableName())

	ruleStore, err := rules.Open(rules.Config{
		Driver:      cfg.Storage.Driver,
		DSN:         cfg.Storage.DSN,
		Dialect:     cfg.Storage.Dialect,
		AutoMigrate: cfg.Storage.AutoMigrate,
		Pool:        pool,
	})
	if err != nil {
		return stores, fmt.Errorf("rules storage: %w", err)
	}
	stores.ruleStore = ruleStore
	addCloser(func() { _ = ruleStore.Close() })
	logger.Printf("rules enabled driver=%s dialect=%s table=githook_rules", cfg.Storage.Driver, cfg.Storage.Dialect)

	logStore, err := eventlogs.Open(eventlogs.Config{
		Driver:      cfg.Storage.Driver,
		DSN:         cfg.Storage.DSN,
		Dialect:     cfg.Storage.Dialect,
		AutoMigrate: cfg.Storage.AutoMigrate,
		Pool:        pool,
	})
	if err != nil {
		return stores, fmt.Errorf("event logs storage: %w", err)
	}
	stores.logStore = logStore
	addCloser(func() { _ = logStore.Close() })
	logger.Printf("event logs enabled driver=%s dialect=%s table=githook_event_logs", cfg.Storage.Driver, cfg.Storage.Dialect)

	driverStore, err := driversstore.Open(driversstore.Config{
		Driver:      cfg.Storage.Driver,
		DSN:         cfg.Storage.DSN,
		Dialect:     cfg.Storage.Dialect,
		AutoMigrate: cfg.Storage.AutoMigrate,
		Pool:        pool,
	})
	if err != nil {
		return stores, fmt.Errorf("driver storage: %w", err)
	}
	stores.driverStore = driverStore
	addCloser(func() { _ = driverStore.Close() })
	logger.Printf("drivers enabled driver=%s dialect=%s table=githook_drivers", cfg.Storage.Driver, cfg.Storage.Dialect)

	instanceStore, err := providerinstancestore.Open(providerinstancestore.Config{
		Driver:      cfg.Storage.Driver,
		DSN:         cfg.Storage.DSN,
		Dialect:     cfg.Storage.Dialect,
		AutoMigrate: cfg.Storage.AutoMigrate,
		Pool:        pool,
	})
	if err != nil {
		return stores, fmt.Errorf("provider instances storage: %w", err)
	}
	stores.instanceStore = instanceStore
	addCloser(func() { _ = instanceStore.Close() })
	logger.Printf("provider instances enabled driver=%s dialect=%s table=githook_provider_instances", cfg.Storage.Driver, cfg.Storage.Dialect)

	return stores, nil
}

func buildCaches(ctx context.Context, stores serverStores, cfg core.Config, logger *log.Logger, addCloser func(func())) (serverCaches, error) {
	caches := serverCaches{
		dynamicDriverCache: driverspkg.NewDynamicPublisherCache(),
	}
	addCloser(func() { _ = caches.dynamicDriverCache.Close() })

	if stores.driverStore != nil {
		caches.driverCache = driverspkg.NewCache(stores.driverStore, cfg.Relaybus, logger)
		if err := caches.driverCache.Refresh(ctx); err != nil {
			return caches, fmt.Errorf("drivers cache: %w", err)
		}
		addCloser(caches.driverCache.Close)
	}

	if stores.instanceStore != nil {
		caches.instanceCache = providerinstance.NewCache(stores.instanceStore, logger)
		if err := caches.instanceCache.Refresh(ctx); err != nil {
			return caches, fmt.Errorf("provider instances cache: %w", err)
		}
		addCloser(caches.instanceCache.Close)
	}

	return caches, nil
}

func buildPublisher(cfg core.Config, driverCache *driverspkg.Cache) (core.Publisher, error) {
	hasBaseDrivers := strings.TrimSpace(cfg.Relaybus.Driver) != "" || len(cfg.Relaybus.Drivers) > 0
	var basePublisher core.Publisher
	if hasBaseDrivers {
		var err error
		basePublisher, err = core.NewPublisher(cfg.Relaybus)
		if err != nil {
			return nil, fmt.Errorf("publisher: %w", err)
		}
	}

	switch {
	case driverCache != nil:
		return driverspkg.NewTenantPublisher(driverCache, basePublisher), nil
	case basePublisher != nil:
		return basePublisher, nil
	default:
		return nil, errors.New("relaybus publisher not configured")
	}
}
