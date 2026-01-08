package oauth

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"

	"githooks/pkg/auth"
)

// StartHandler redirects users into provider install/authorize flows.
type StartHandler struct {
	Providers     auth.Config
	PublicBaseURL string
	Logger        *log.Logger
}

func (h *StartHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	logger := h.Logger
	if logger == nil {
		logger = log.Default()
	}
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	provider := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("provider")))
	if provider == "" {
		http.Error(w, "missing provider", http.StatusBadRequest)
		return
	}

	state := strings.TrimSpace(r.URL.Query().Get("state"))
	if state == "" {
		state = randomState()
	}
	tenantID := strings.TrimSpace(r.URL.Query().Get("tenant_id"))
	instanceKey := strings.TrimSpace(r.URL.Query().Get("instance"))
	state = encodeState(state, tenantID, instanceKey)

	switch provider {
	case "github":
		target, err := githubInstallURL(h.Providers.GitHub, state)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		http.Redirect(w, r, target, http.StatusFound)
	case "gitlab":
		redirectURL := callbackURL(r, "gitlab", h.PublicBaseURL)
		target, err := gitlabAuthorizeURL(h.Providers.GitLab, state, redirectURL)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		http.Redirect(w, r, target, http.StatusFound)
	case "bitbucket":
		redirectURL := callbackURL(r, "bitbucket", h.PublicBaseURL)
		target, err := bitbucketAuthorizeURL(h.Providers.Bitbucket, state, redirectURL)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		http.Redirect(w, r, target, http.StatusFound)
	default:
		http.Error(w, "unsupported provider", http.StatusBadRequest)
	}
}

func githubInstallURL(cfg auth.ProviderConfig, state string) (string, error) {
	appSlug := strings.TrimSpace(cfg.App.AppSlug)
	if appSlug == "" {
		return "", fmt.Errorf("github app_slug is required")
	}
	webBase := githubWebBase(cfg)
	return addQueryParam(fmt.Sprintf("%s/apps/%s/installations/new", webBase, appSlug), "state", state)
}

func gitlabAuthorizeURL(cfg auth.ProviderConfig, state, redirectURL string) (string, error) {
	if cfg.OAuth.ClientID == "" {
		return "", fmt.Errorf("gitlab oauth_client_id is required")
	}
	webBase := gitlabWebBase(cfg)
	u, err := url.Parse(webBase + "/oauth/authorize")
	if err != nil {
		return "", err
	}
	q := u.Query()
	q.Set("client_id", cfg.OAuth.ClientID)
	q.Set("response_type", "code")
	if redirectURL != "" {
		q.Set("redirect_uri", redirectURL)
	}
	if len(cfg.OAuth.Scopes) > 0 {
		q.Set("scope", strings.Join(cfg.OAuth.Scopes, " "))
	}
	if state != "" {
		q.Set("state", state)
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func bitbucketAuthorizeURL(cfg auth.ProviderConfig, state, redirectURL string) (string, error) {
	if cfg.OAuth.ClientID == "" {
		return "", fmt.Errorf("bitbucket oauth_client_id is required")
	}
	webBase := strings.TrimRight(cfg.API.WebBaseURL, "/")
	if webBase == "" {
		webBase = "https://bitbucket.org"
	}
	u, err := url.Parse(webBase + "/site/oauth2/authorize")
	if err != nil {
		return "", err
	}
	q := u.Query()
	q.Set("client_id", cfg.OAuth.ClientID)
	q.Set("response_type", "code")
	if redirectURL != "" {
		q.Set("redirect_uri", redirectURL)
	}
	if len(cfg.OAuth.Scopes) > 0 {
		q.Set("scope", strings.Join(cfg.OAuth.Scopes, " "))
	}
	if state != "" {
		q.Set("state", state)
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func githubWebBase(cfg auth.ProviderConfig) string {
	webBase := strings.TrimRight(cfg.API.WebBaseURL, "/")
	if webBase != "" {
		return webBase
	}
	base := strings.TrimRight(cfg.API.BaseURL, "/")
	if base == "" || base == "https://api.github.com" {
		return "https://github.com"
	}
	webBase = strings.TrimSuffix(base, "/api/v3")
	webBase = strings.TrimSuffix(webBase, "/api")
	if webBase == "" {
		return "https://github.com"
	}
	return webBase
}

func gitlabWebBase(cfg auth.ProviderConfig) string {
	webBase := strings.TrimRight(cfg.API.WebBaseURL, "/")
	if webBase != "" {
		return webBase
	}
	base := strings.TrimRight(cfg.API.BaseURL, "/")
	if base == "" {
		return "https://gitlab.com"
	}
	webBase = strings.TrimSuffix(base, "/api/v4")
	if webBase == "" {
		return "https://gitlab.com"
	}
	return webBase
}

func addQueryParam(rawURL, key, value string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}
	if value == "" {
		return u.String(), nil
	}
	q := u.Query()
	q.Set(key, value)
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func randomState() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return ""
	}
	return hex.EncodeToString(buf)
}
