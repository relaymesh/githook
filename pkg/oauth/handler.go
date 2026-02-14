package oauth

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"time"

	"githook/pkg/auth"
	"githook/pkg/providerinstance"
	ghprovider "githook/pkg/providers/github"
	"githook/pkg/storage"
)

// Handler handles OAuth callbacks and persists installation data.
type Handler struct {
	Provider              string
	Config                auth.ProviderConfig
	Providers             auth.Config
	Store                 storage.Store
	NamespaceStore        storage.NamespaceStore
	ProviderInstanceStore storage.ProviderInstanceStore
	ProviderInstanceCache *providerinstance.Cache
	Logger                *log.Logger
	RedirectBase          string
	Endpoint              string
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	logger := h.Logger
	if logger == nil {
		logger = log.Default()
	}
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	provider := h.Provider
	config := h.Config
	if provider == "" {
		provider = providerFromPath(r.URL.Path)
		switch provider {
		case "github":
			config = h.Providers.GitHub
		case "gitlab":
			config = h.Providers.GitLab
		case "bitbucket":
			config = h.Providers.Bitbucket
		}
	}

	switch provider {
	case "gitlab":
		h.handleGitLab(w, r, logger, config)
	case "bitbucket":
		h.handleBitbucket(w, r, logger, config)
	case "github":
		h.handleGitHubApp(w, r, logger, config)
	default:
		http.Error(w, "unsupported provider", http.StatusBadRequest)
	}
}

func (h *Handler) handleGitHubApp(w http.ResponseWriter, r *http.Request, logger *log.Logger, cfg auth.ProviderConfig) {
	stateValue := decodeState(r.URL.Query().Get("state"))
	installationID := r.URL.Query().Get("installation_id")
	code := r.URL.Query().Get("code")
	if installationID == "" {
		http.Error(w, "missing installation_id", http.StatusBadRequest)
		return
	}
	cfg, instanceKey := h.resolveInstanceConfig(r.Context(), "github", stateValue.InstanceKey, cfg)

	var token oauthToken
	var err error
	if code != "" {
		if cfg.OAuth.ClientID == "" || cfg.OAuth.ClientSecret == "" {
			http.Error(w, "oauth client config missing", http.StatusInternalServerError)
			return
		}
		redirectURL := callbackURL(r, "github", h.Endpoint)
		token, err = exchangeGitHubToken(r.Context(), cfg, code, redirectURL)
		if err != nil {
			logger.Printf("github oauth exchange failed: %v", err)
			http.Error(w, "token exchange failed", http.StatusBadRequest)
			return
		}
	}

	accessToken := token.AccessToken
	refreshToken := token.RefreshToken
	warning := ""

	accountID, accountName, err := resolveGitHubAccount(r.Context(), cfg, installationID)
	if err != nil {
		logger.Printf("github account resolve failed: %v", err)
	}
	storeCtx := storage.WithTenant(r.Context(), stateValue.TenantID)
	if err := SyncGitHubNamespaces(storeCtx, h.NamespaceStore, cfg, installationID, accountID, instanceKey); err != nil {
		logger.Printf("github namespaces sync failed: %v", err)
	}

	record := storage.InstallRecord{
		TenantID:            stateValue.TenantID,
		Provider:            "github",
		AccountID:           accountID,
		AccountName:         accountName,
		InstallationID:      installationID,
		ProviderInstanceKey: instanceKey,
		AccessToken:         accessToken,
		RefreshToken:        refreshToken,
		ExpiresAt:           token.ExpiresAt,
		MetadataJSON:        token.MetadataJSON(),
	}
	logUpsertAttempt(logger, record, token.AccessToken)
	if !storeAvailable(h.Store) {
		if warning == "" {
			warning = "storage_not_configured"
		}
	} else if err := h.Store.UpsertInstallation(storeCtx, record); err != nil {
		logger.Printf("github install upsert failed: %v", err)
		if warning == "" {
			warning = "storage_persist_failed"
		}
	} else {
		dedupeInstallations(storeCtx, h.Store, "github", accountID, instanceKey, installationID)
	}

	params := map[string]string{
		"id":              randomID(),
		"provider":        "github",
		"installation_id": installationID,
		"state":           stateValue.State,
	}
	if warning != "" {
		params["warning"] = warning
	}
	h.redirectOrJSON(w, r, params)
}

func (h *Handler) handleGitLab(w http.ResponseWriter, r *http.Request, logger *log.Logger, cfg auth.ProviderConfig) {
	code := r.URL.Query().Get("code")
	stateValue := decodeState(r.URL.Query().Get("state"))
	if code == "" {
		http.Error(w, "missing code", http.StatusBadRequest)
		return
	}
	cfg, instanceKey := h.resolveInstanceConfig(r.Context(), "gitlab", stateValue.InstanceKey, cfg)
	if cfg.OAuth.ClientID == "" || cfg.OAuth.ClientSecret == "" {
		http.Error(w, "oauth client config missing", http.StatusInternalServerError)
		return
	}

	redirectURL := callbackURL(r, "gitlab", h.Endpoint)
	token, err := exchangeGitLabToken(r.Context(), cfg, code, redirectURL)
	if err != nil {
		logger.Printf("gitlab token exchange failed: %v", err)
		http.Error(w, "token exchange failed", http.StatusBadRequest)
		return
	}

	accessToken := token.AccessToken
	refreshToken := token.RefreshToken
	warning := ""

	accountID, accountName, err := resolveGitLabAccount(r.Context(), cfg, token.AccessToken)
	if err != nil {
		logger.Printf("gitlab account resolve failed: %v", err)
	}
	storeCtx := storage.WithTenant(r.Context(), stateValue.TenantID)
	installationID := resolveExistingInstallationID(storeCtx, h.Store, "gitlab", accountID, instanceKey)
	if installationID == "" {
		installationID = randomID()
	}
	if err := SyncGitLabNamespaces(storeCtx, h.NamespaceStore, cfg, token.AccessToken, accountID, installationID, instanceKey); err != nil {
		logger.Printf("gitlab namespaces sync failed: %v", err)
	}
	record := storage.InstallRecord{
		TenantID:            stateValue.TenantID,
		Provider:            "gitlab",
		AccountID:           accountID,
		AccountName:         accountName,
		InstallationID:      installationID,
		ProviderInstanceKey: instanceKey,
		AccessToken:         accessToken,
		RefreshToken:        refreshToken,
		ExpiresAt:           token.ExpiresAt,
		MetadataJSON:        token.MetadataJSON(),
	}
	logUpsertAttempt(logger, record, token.AccessToken)
	if !storeAvailable(h.Store) {
		if warning == "" {
			warning = "storage_not_configured"
		}
	} else if err := h.Store.UpsertInstallation(storeCtx, record); err != nil {
		logger.Printf("gitlab install upsert failed: %v", err)
		if warning == "" {
			warning = "storage_persist_failed"
		}
	} else {
		dedupeInstallations(storeCtx, h.Store, "gitlab", accountID, instanceKey, installationID)
	}

	params := map[string]string{
		"id":              randomID(),
		"provider":        "gitlab",
		"installation_id": installationID,
		"state":           stateValue.State,
	}
	if warning != "" {
		params["warning"] = warning
	}
	if warning == "storage_not_configured" {
		logger.Printf("gitlab oauth tokens access_token=%s refresh_token=%s expires_at=%v", token.AccessToken, token.RefreshToken, token.ExpiresAt)
	}
	h.redirectOrJSON(w, r, params)
}

func (h *Handler) handleBitbucket(w http.ResponseWriter, r *http.Request, logger *log.Logger, cfg auth.ProviderConfig) {
	code := r.URL.Query().Get("code")
	stateValue := decodeState(r.URL.Query().Get("state"))
	if code == "" {
		http.Error(w, "missing code", http.StatusBadRequest)
		return
	}
	cfg, instanceKey := h.resolveInstanceConfig(r.Context(), "bitbucket", stateValue.InstanceKey, cfg)
	if cfg.OAuth.ClientID == "" || cfg.OAuth.ClientSecret == "" {
		http.Error(w, "oauth client config missing", http.StatusInternalServerError)
		return
	}

	redirectURL := callbackURL(r, "bitbucket", h.Endpoint)
	token, err := exchangeBitbucketToken(r.Context(), cfg, code, redirectURL)
	if err != nil {
		logger.Printf("bitbucket token exchange failed: %v", err)
		http.Error(w, "token exchange failed", http.StatusBadRequest)
		return
	}

	accessToken := token.AccessToken
	refreshToken := token.RefreshToken
	warning := ""

	accountID, accountName, err := resolveBitbucketAccount(r.Context(), cfg, token.AccessToken)
	if err != nil {
		logger.Printf("bitbucket account resolve failed: %v", err)
	}
	storeCtx := storage.WithTenant(r.Context(), stateValue.TenantID)
	installationID := resolveExistingInstallationID(storeCtx, h.Store, "bitbucket", accountID, instanceKey)
	if installationID == "" {
		installationID = randomID()
	}
	if err := SyncBitbucketNamespaces(storeCtx, h.NamespaceStore, cfg, token.AccessToken, accountID, installationID, instanceKey); err != nil {
		logger.Printf("bitbucket namespaces sync failed: %v", err)
	}
	record := storage.InstallRecord{
		TenantID:            stateValue.TenantID,
		Provider:            "bitbucket",
		AccountID:           accountID,
		AccountName:         accountName,
		InstallationID:      installationID,
		ProviderInstanceKey: instanceKey,
		AccessToken:         accessToken,
		RefreshToken:        refreshToken,
		ExpiresAt:           token.ExpiresAt,
		MetadataJSON:        token.MetadataJSON(),
	}
	logUpsertAttempt(logger, record, token.AccessToken)
	if !storeAvailable(h.Store) {
		if warning == "" {
			warning = "storage_not_configured"
		}
	} else if err := h.Store.UpsertInstallation(storeCtx, record); err != nil {
		logger.Printf("bitbucket install upsert failed: %v", err)
		if warning == "" {
			warning = "storage_persist_failed"
		}
	} else {
		dedupeInstallations(storeCtx, h.Store, "bitbucket", accountID, instanceKey, installationID)
	}

	params := map[string]string{
		"id":              randomID(),
		"provider":        "bitbucket",
		"installation_id": installationID,
		"state":           stateValue.State,
	}
	if warning != "" {
		params["warning"] = warning
	}
	h.redirectOrJSON(w, r, params)
}

func (h *Handler) redirectOrJSON(w http.ResponseWriter, r *http.Request, params map[string]string) {
	redirect := strings.TrimSpace(h.RedirectBase)
	if redirect == "" {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(params)
		return
	}
	target, err := url.Parse(redirect)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(params)
		return
	}
	values := target.Query()
	for key, value := range params {
		if value == "" {
			continue
		}
		values.Set(key, value)
	}
	target.RawQuery = values.Encode()
	http.Redirect(w, r, target.String(), http.StatusFound)
}

func (h *Handler) resolveInstanceConfig(ctx context.Context, provider, instanceKey string, fallback auth.ProviderConfig) (auth.ProviderConfig, string) {
	provider = strings.TrimSpace(provider)
	instanceKey = strings.TrimSpace(instanceKey)
	if h.ProviderInstanceCache == nil && h.ProviderInstanceStore == nil {
		return fallback, instanceKey
	}
	if instanceKey == "" {
		if h.ProviderInstanceStore == nil {
			return fallback, instanceKey
		}
		records, err := h.ProviderInstanceStore.ListProviderInstances(ctx, provider)
		if err != nil || len(records) == 0 {
			return fallback, instanceKey
		}
		if len(records) == 1 {
			cfg, err := providerinstance.ProviderConfigFromRecord(records[0])
			if err != nil {
				return fallback, instanceKey
			}
			return cfg, records[0].Key
		}
		match, ok := matchProviderConfigRecord(records, fallback)
		if !ok {
			return fallback, instanceKey
		}
		cfg, err := providerinstance.ProviderConfigFromRecord(match)
		if err != nil {
			return fallback, instanceKey
		}
		return cfg, match.Key
	}
	if h.ProviderInstanceCache != nil {
		if cfg, ok, err := h.ProviderInstanceCache.ConfigFor(ctx, provider, instanceKey); err == nil && ok {
			return cfg, instanceKey
		}
	}
	if h.ProviderInstanceStore == nil {
		return fallback, instanceKey
	}
	record, err := h.ProviderInstanceStore.GetProviderInstance(ctx, provider, instanceKey)
	if err != nil || record == nil {
		return fallback, instanceKey
	}
	cfg, err := providerinstance.ProviderConfigFromRecord(*record)
	if err != nil {
		return fallback, instanceKey
	}
	return cfg, instanceKey
}

func resolveExistingInstallationID(ctx context.Context, store storage.Store, provider, accountID, instanceKey string) string {
	if store == nil {
		return ""
	}
	provider = strings.TrimSpace(provider)
	accountID = strings.TrimSpace(accountID)
	instanceKey = strings.TrimSpace(instanceKey)
	if provider == "" || accountID == "" {
		return ""
	}
	records, err := store.ListInstallations(ctx, provider, accountID)
	if err != nil || len(records) == 0 {
		return ""
	}
	var bestID string
	var bestTime time.Time
	for _, record := range records {
		if instanceKey != "" && record.ProviderInstanceKey != instanceKey {
			continue
		}
		if record.InstallationID == "" {
			continue
		}
		if bestID == "" || record.UpdatedAt.After(bestTime) {
			bestID = record.InstallationID
			bestTime = record.UpdatedAt
		}
	}
	return bestID
}

func dedupeInstallations(ctx context.Context, store storage.Store, provider, accountID, instanceKey, keepID string) {
	if store == nil {
		return
	}
	provider = strings.TrimSpace(provider)
	accountID = strings.TrimSpace(accountID)
	keepID = strings.TrimSpace(keepID)
	instanceKey = strings.TrimSpace(instanceKey)
	if provider == "" || accountID == "" || keepID == "" {
		return
	}
	records, err := store.ListInstallations(ctx, provider, accountID)
	if err != nil {
		return
	}
	for _, record := range records {
		if record.InstallationID == "" || record.InstallationID == keepID {
			continue
		}
		if instanceKey != "" && record.ProviderInstanceKey != "" && record.ProviderInstanceKey != instanceKey {
			continue
		}
		_ = store.DeleteInstallation(ctx, provider, accountID, record.InstallationID, record.ProviderInstanceKey)
	}
}

func matchProviderConfigRecord(records []storage.ProviderInstanceRecord, fallback auth.ProviderConfig) (storage.ProviderInstanceRecord, bool) {
	configJSON, ok := providerConfigJSON(fallback)
	if !ok {
		return storage.ProviderInstanceRecord{}, false
	}
	var match *storage.ProviderInstanceRecord
	for i := range records {
		record := records[i]
		normalized, ok := providerinstance.NormalizeProviderConfigJSON(record.ConfigJSON)
		if !ok || normalized != configJSON {
			continue
		}
		if !isProviderInstanceHash(record.Key) {
			continue
		}
		if match != nil {
			return storage.ProviderInstanceRecord{}, false
		}
		match = &record
	}
	if match == nil {
		return storage.ProviderInstanceRecord{}, false
	}
	return *match, true
}

func providerConfigJSON(cfg auth.ProviderConfig) (string, bool) {
	cfg.Key = ""
	raw, err := json.Marshal(cfg)
	if err != nil {
		return "", false
	}
	return strings.TrimSpace(string(raw)), true
}

func isProviderInstanceHash(value string) bool {
	value = strings.TrimSpace(value)
	if len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

type oauthToken struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
	TokenType    string `json:"token_type"`
	Scope        string `json:"scope"`
	CreatedAt    int64  `json:"created_at"`
	ExpiresAt    *time.Time
}

func (t oauthToken) MetadataJSON() string {
	payload := map[string]interface{}{
		"token_type": t.TokenType,
		"scope":      t.Scope,
	}
	raw, _ := json.Marshal(payload)
	return string(raw)
}

func exchangeGitLabToken(ctx context.Context, cfg auth.ProviderConfig, code, redirectURL string) (oauthToken, error) {
	baseURL := strings.TrimRight(cfg.API.BaseURL, "/")
	if baseURL == "" {
		baseURL = "https://gitlab.com/api/v4"
	}
	oauthBase := strings.TrimSuffix(baseURL, "/api/v4")
	endpoint := oauthBase + "/oauth/token"

	values := url.Values{}
	values.Set("client_id", cfg.OAuth.ClientID)
	values.Set("client_secret", cfg.OAuth.ClientSecret)
	values.Set("code", code)
	values.Set("grant_type", "authorization_code")
	if redirectURL != "" {
		values.Set("redirect_uri", redirectURL)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(values.Encode()))
	if err != nil {
		return oauthToken{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return oauthToken{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return oauthToken{}, fmt.Errorf("gitlab token exchange failed: %s", resp.Status)
	}
	var token oauthToken
	if err := json.NewDecoder(resp.Body).Decode(&token); err != nil {
		return oauthToken{}, err
	}
	token.ExpiresAt = expiryFromToken(token)
	if token.AccessToken == "" {
		return oauthToken{}, errors.New("gitlab access token missing")
	}
	return token, nil
}

func resolveGitHubAccount(ctx context.Context, cfg auth.ProviderConfig, installationID string) (string, string, error) {
	if cfg.App.AppID == 0 ||
		(cfg.App.PrivateKeyPath == "" && cfg.App.PrivateKeyPEM == "") ||
		installationID == "" {
		return "", "", errors.New("github app config missing")
	}
	id, err := strconv.ParseInt(installationID, 10, 64)
	if err != nil {
		return "", "", err
	}
	account, err := ghprovider.FetchInstallationAccount(ctx, ghprovider.AppConfig{
		AppID:          cfg.App.AppID,
		PrivateKeyPath: cfg.App.PrivateKeyPath,
		PrivateKeyPEM:  cfg.App.PrivateKeyPEM,
		BaseURL:        cfg.API.BaseURL,
	}, id)
	if err != nil {
		return "", "", err
	}
	return account.ID, account.Name, nil
}

func resolveGitLabAccount(ctx context.Context, cfg auth.ProviderConfig, accessToken string) (string, string, error) {
	if accessToken == "" {
		return "", "", errors.New("gitlab access token missing")
	}
	baseURL := strings.TrimRight(cfg.API.BaseURL, "/")
	if baseURL == "" {
		baseURL = "https://gitlab.com/api/v4"
	}
	endpoint := baseURL + "/user"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return "", "", fmt.Errorf("gitlab user lookup failed: %s body=%s", resp.Status, strings.TrimSpace(string(body)))
	}
	var payload struct {
		ID       int64  `json:"id"`
		Username string `json:"username"`
		Name     string `json:"name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", "", err
	}
	name := payload.Username
	if name == "" {
		name = payload.Name
	}
	return strconv.FormatInt(payload.ID, 10), name, nil
}

func resolveBitbucketAccount(ctx context.Context, cfg auth.ProviderConfig, accessToken string) (string, string, error) {
	if accessToken == "" {
		return "", "", errors.New("bitbucket access token missing")
	}
	baseURL := strings.TrimRight(cfg.API.BaseURL, "/")
	if baseURL == "" {
		baseURL = "https://api.bitbucket.org/2.0"
	}
	endpoint := baseURL + "/user"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return "", "", fmt.Errorf("bitbucket user lookup failed: %s body=%s", resp.Status, strings.TrimSpace(string(body)))
	}
	var payload struct {
		UUID        string `json:"uuid"`
		Username    string `json:"username"`
		DisplayName string `json:"display_name"`
		Nickname    string `json:"nickname"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", "", err
	}
	name := payload.DisplayName
	if name == "" {
		name = payload.Nickname
	}
	if name == "" {
		name = payload.Username
	}
	return payload.UUID, name, nil
}

func storeAvailable(store storage.Store) bool {
	if store == nil {
		return false
	}
	value := reflect.ValueOf(store)
	switch value.Kind() {
	case reflect.Ptr, reflect.Map, reflect.Slice, reflect.Interface, reflect.Func, reflect.Chan:
		return !value.IsNil()
	default:
		return true
	}
}

func namespaceStoreAvailable(store storage.NamespaceStore) bool {
	if store == nil {
		return false
	}
	value := reflect.ValueOf(store)
	switch value.Kind() {
	case reflect.Ptr, reflect.Map, reflect.Slice, reflect.Interface, reflect.Func, reflect.Chan:
		return !value.IsNil()
	default:
		return true
	}
}

func logUpsertAttempt(logger *log.Logger, record storage.InstallRecord, accessToken string) {
	if logger == nil {
		return
	}
	tokenState := "empty"
	if accessToken != "" {
		tokenState = "present"
	}
	logger.Printf("oauth upsert attempt provider=%s account_id=%s installation_id=%s token=%s", record.Provider, record.AccountID, record.InstallationID, tokenState)
}

func exchangeGitHubToken(ctx context.Context, cfg auth.ProviderConfig, code, redirectURL string) (oauthToken, error) {
	base := strings.TrimRight(cfg.API.BaseURL, "/")
	oauthBase := "https://github.com"
	if base != "" && base != "https://api.github.com" {
		oauthBase = strings.TrimSuffix(base, "/api/v3")
		oauthBase = strings.TrimSuffix(oauthBase, "/api")
	}
	endpoint := oauthBase + "/login/oauth/access_token"

	values := url.Values{}
	values.Set("client_id", cfg.OAuth.ClientID)
	values.Set("client_secret", cfg.OAuth.ClientSecret)
	values.Set("code", code)
	if redirectURL != "" {
		values.Set("redirect_uri", redirectURL)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(values.Encode()))
	if err != nil {
		return oauthToken{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return oauthToken{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return oauthToken{}, fmt.Errorf("github token exchange failed: %s", resp.Status)
	}
	var token oauthToken
	if err := json.NewDecoder(resp.Body).Decode(&token); err != nil {
		return oauthToken{}, err
	}
	token.ExpiresAt = expiryFromToken(token)
	if token.AccessToken == "" {
		return oauthToken{}, errors.New("github access token missing")
	}
	return token, nil
}

func exchangeBitbucketToken(ctx context.Context, cfg auth.ProviderConfig, code, redirectURL string) (oauthToken, error) {
	endpoint := "https://bitbucket.org/site/oauth2/access_token"

	values := url.Values{}
	values.Set("grant_type", "authorization_code")
	values.Set("code", code)
	if redirectURL != "" {
		values.Set("redirect_uri", redirectURL)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(values.Encode()))
	if err != nil {
		return oauthToken{}, err
	}
	req.SetBasicAuth(cfg.OAuth.ClientID, cfg.OAuth.ClientSecret)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return oauthToken{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return oauthToken{}, fmt.Errorf("bitbucket token exchange failed: %s body=%s", resp.Status, strings.TrimSpace(string(body)))
	}
	var token oauthToken
	if err := json.NewDecoder(resp.Body).Decode(&token); err != nil {
		return oauthToken{}, err
	}
	token.ExpiresAt = expiryFromToken(token)
	if token.AccessToken == "" {
		return oauthToken{}, errors.New("bitbucket access token missing")
	}
	return token, nil
}

func expiryFromToken(token oauthToken) *time.Time {
	if token.ExpiresIn <= 0 {
		return nil
	}
	expires := time.Now().UTC().Add(time.Duration(token.ExpiresIn) * time.Second)
	return &expires
}
