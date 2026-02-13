package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
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

	"githook/pkg/api"
	"githook/pkg/auth"
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
	if tenantID != "" {
		loaded := make([]core.Rule, 0, len(records))
		for _, record := range records {
			loaded = append(loaded, core.Rule{
				ID:      record.ID,
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

	grouped := make(map[string][]core.Rule)
	for _, record := range records {
		grouped[record.TenantID] = append(grouped[record.TenantID], core.Rule{
			ID:      record.ID,
			When:    record.When,
			Emit:    core.EmitList(record.Emit),
			Drivers: record.Drivers,
		})
	}
	for id, rules := range grouped {
		normalized, err := core.NormalizeRules(rules)
		if err != nil {
			return err
		}
		if logger != nil {
			logger.Printf("rules bootstrap: loaded %d rules from storage tenant=%s", len(normalized), id)
		}
		if err := engine.Update(core.RulesConfig{
			Rules:    normalized,
			Strict:   strict,
			TenantID: id,
			Logger:   logger,
		}); err != nil {
			return err
		}
	}
	return nil
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

func bootstrapProviderInstances(
	ctx context.Context,
	store *providerinstancestore.Store,
	installStore *installations.Store,
	namespaceStore *namespaces.Store,
	cfg auth.Config,
	logger *log.Logger,
) error {
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
		hash, legacyKey, err := resolveProviderInstanceHash(ctx, store, record)
		if err != nil {
			return err
		}
		if legacyKey != "" {
			if err := migrateProviderInstanceKey(ctx, store, installStore, namespaceStore, record.Provider, legacyKey, hash, record.TenantID); err != nil {
				return err
			}
		}
		record.Key = hash
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

func resolveProviderInstanceHash(
	ctx context.Context,
	store *providerinstancestore.Store,
	record storage.ProviderInstanceRecord,
) (string, string, error) {
	provider := strings.TrimSpace(record.Provider)
	if provider == "" {
		return "", "", errors.New("provider is required")
	}
	tenantID := strings.TrimSpace(record.TenantID)
	records, err := store.ListProviderInstances(ctx, provider)
	if err != nil {
		return "", "", err
	}
	configJSON := strings.TrimSpace(record.ConfigJSON)
	legacyKey := ""
	for _, existing := range records {
		if strings.TrimSpace(existing.TenantID) != tenantID {
			continue
		}
		if strings.TrimSpace(existing.ConfigJSON) != configJSON {
			continue
		}
		key := strings.TrimSpace(existing.Key)
		if key == "" {
			continue
		}
		if isProviderInstanceHash(key) {
			return key, "", nil
		}
		if legacyKey != "" && legacyKey != key {
			return "", "", errors.New("multiple legacy provider instance keys match config")
		}
		legacyKey = key
	}
	hash, err := randomHex(32)
	if err != nil {
		return "", "", err
	}
	return hash, legacyKey, nil
}

func migrateProviderInstanceKey(
	ctx context.Context,
	instanceStore *providerinstancestore.Store,
	installStore *installations.Store,
	namespaceStore *namespaces.Store,
	provider string,
	oldKey string,
	newKey string,
	tenantID string,
) error {
	provider = strings.TrimSpace(provider)
	oldKey = strings.TrimSpace(oldKey)
	newKey = strings.TrimSpace(newKey)
	tenantID = strings.TrimSpace(tenantID)
	if provider == "" || oldKey == "" || newKey == "" {
		return errors.New("provider and keys are required")
	}
	if oldKey == newKey {
		return nil
	}
	if instanceStore == nil {
		return errors.New("provider instance store not configured")
	}
	records, err := instanceStore.ListProviderInstances(ctx, provider)
	if err != nil {
		return err
	}
	hasNewKey := false
	for _, record := range records {
		if strings.TrimSpace(record.TenantID) != tenantID {
			continue
		}
		if strings.TrimSpace(record.Key) == newKey {
			hasNewKey = true
			break
		}
	}
	if installStore != nil {
		if _, err := installStore.UpdateProviderInstanceKey(ctx, provider, oldKey, newKey, tenantID); err != nil {
			return err
		}
	}
	if namespaceStore != nil {
		if _, err := namespaceStore.UpdateProviderInstanceKey(ctx, provider, oldKey, newKey, tenantID); err != nil {
			return err
		}
	}
	if hasNewKey {
		return instanceStore.DeleteProviderInstanceForTenant(ctx, provider, oldKey, tenantID)
	}
	if _, err := instanceStore.UpdateProviderInstanceKey(ctx, provider, oldKey, newKey, tenantID); err != nil {
		return err
	}
	return nil
}

func isProviderInstanceHash(value string) bool {
	value = strings.TrimSpace(value)
	if len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func randomHex(size int) (string, error) {
	if size <= 0 {
		return "", errors.New("random hex size must be positive")
	}
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
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
		logStore       *eventlogs.Store
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
		if err := bootstrapProviderInstances(ctx, instanceStore, installStore, namespaceStore, config.Providers, logger); err != nil {
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
		Providers:             config.Providers,
		PublicBaseURL:         config.Server.PublicBaseURL,
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
			PublicBaseURL:         config.Server.PublicBaseURL,
			Logger:                logger,
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
	{
		eventLogSvc := &api.EventLogsService{
			Store:  logStore,
			Logger: logger,
		}
		path, handler := cloudv1connect.NewEventLogsServiceHandler(eventLogSvc, connectOpts...)
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
			logStore,
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
			logStore,
		)
		if err != nil {
			return fmt.Errorf("gitlab handler: %w", err)
		}
		mux.Handle(config.Providers.GitLab.Webhook.Path, glHandler)
		logger.Printf(
			"provider=gitlab webhook=enabled path=%s oauth_callback=/auth/gitlab/callback",
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
			logStore,
		)
		if err != nil {
			return fmt.Errorf("bitbucket handler: %w", err)
		}
		mux.Handle(config.Providers.Bitbucket.Webhook.Path, bbHandler)
		logger.Printf(
			"provider=bitbucket webhook=enabled path=%s oauth_callback=/auth/bitbucket/callback",
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
