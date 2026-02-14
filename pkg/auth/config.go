package auth

// Config contains provider configuration for webhooks and SCM auth.
type Config struct {
	GitHub    ProviderConfig `yaml:"github"`
	GitLab    ProviderConfig `yaml:"gitlab"`
	Bitbucket ProviderConfig `yaml:"bitbucket"`
}

// ProviderConfig contains webhook and auth configuration for a provider.
type ProviderConfig struct {
	Enabled bool   `yaml:"enabled"` // Deprecated: webhooks are always enabled.
	Key     string `yaml:"key"`

	Webhook WebhookConfig `yaml:"webhook"`
	App     AppConfig     `yaml:"app"`
	OAuth   OAuthConfig   `yaml:"oauth"`
	API     APIConfig     `yaml:"api"`
}

// WebhookConfig contains inbound webhook settings.
type WebhookConfig struct {
	Path   string `yaml:"path"`
	Secret string `yaml:"secret"`
}

// AppConfig contains GitHub App settings.
type AppConfig struct {
	AppID          int64  `yaml:"app_id"`
	PrivateKeyPath string `yaml:"private_key_path"`
	PrivateKeyPEM  string `yaml:"private_key_pem" json:"PrivateKeyPEM,omitempty"`
	AppSlug        string `yaml:"app_slug"`
}

// OAuthConfig contains OAuth settings (future OAuth2 expansion).
type OAuthConfig struct {
	ClientID     string   `yaml:"client_id"`
	ClientSecret string   `yaml:"client_secret"`
	Scopes       []string `yaml:"scopes"`
}

// APIConfig contains provider API and web base URLs.
type APIConfig struct {
	BaseURL    string `yaml:"base_url"`
	WebBaseURL string `yaml:"web_base_url"`
}
