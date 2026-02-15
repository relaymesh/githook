package worker

import (
	"context"
	"net/http"
	"os"
	"strings"

	"githook/pkg/auth"
)

const defaultAPIEndpoint = "http://localhost:8080"

func resolveEndpoint(explicit string) string {
	if trimmed := strings.TrimSpace(explicit); trimmed != "" {
		return strings.TrimRight(trimmed, "/")
	}
	if endpoint := envEndpoint("GITHOOK_ENDPOINT"); endpoint != "" {
		return endpoint
	}
	if endpoint := envEndpoint("GITHOOK_API_BASE_URL"); endpoint != "" {
		return endpoint
	}
	return defaultAPIEndpoint
}

func envEndpoint(key string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return strings.TrimRight(value, "/")
	}
	return ""
}

func apiKeyFromEnv() string {
	return strings.TrimSpace(os.Getenv("GITHOOK_API_KEY"))
}

func (w *Worker) apiBaseURL() string {
	explicit := ""
	if w != nil {
		explicit = w.endpoint
	}
	return resolveEndpoint(explicit)
}

func (w *Worker) apiKeyValue() string {
	if w != nil {
		if key := strings.TrimSpace(w.apiKey); key != "" {
			return key
		}
	}
	return apiKeyFromEnv()
}

func (w *Worker) oauth2Value() *auth.OAuth2Config {
	if w == nil {
		return nil
	}
	return w.oauth2Config
}

func setAuthHeaders(ctx context.Context, header http.Header, apiKey string, oauth2Config *auth.OAuth2Config) {
	if header == nil {
		return
	}
	key := strings.TrimSpace(apiKey)
	if key == "" {
		key = apiKeyFromEnv()
	}
	if key != "" {
		header.Set("x-api-key", key)
		return
	}
	if oauth2Config != nil {
		if token, err := oauth2TokenFromConfig(ctx, *oauth2Config); err == nil && token != "" {
			header.Set("Authorization", "Bearer "+token)
		}
		return
	}
	if token, err := oauth2Token(ctx); err == nil && token != "" {
		header.Set("Authorization", "Bearer "+token)
	}
}

func (w *Worker) rulesClient() *RulesClient {
	return &RulesClient{
		BaseURL: w.apiBaseURL(),
		APIKey:  w.apiKeyValue(),
		OAuth2:  w.oauth2Value(),
	}
}

func (w *Worker) driversClient() *DriversClient {
	return &DriversClient{
		BaseURL: w.apiBaseURL(),
		APIKey:  w.apiKeyValue(),
		OAuth2:  w.oauth2Value(),
	}
}

func (w *Worker) eventLogsClient() *EventLogsClient {
	return &EventLogsClient{
		BaseURL: w.apiBaseURL(),
		APIKey:  w.apiKeyValue(),
		OAuth2:  w.oauth2Value(),
	}
}

func (w *Worker) installationsClient() *InstallationsClient {
	return &InstallationsClient{
		BaseURL: w.apiBaseURL(),
		APIKey:  w.apiKeyValue(),
		OAuth2:  w.oauth2Value(),
	}
}
