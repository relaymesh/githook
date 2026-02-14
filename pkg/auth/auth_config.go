package auth

// No auth API-key support in this server build.

// AuthConfig holds API authentication configuration.
type AuthConfig struct {
	OAuth2 OAuth2Config `yaml:"oauth2"`
}

// OAuth2Config configures JWT validation and token acquisition.
type OAuth2Config struct {
	Enabled        bool     `yaml:"enabled"`
	Issuer         string   `yaml:"issuer"`
	Audience       string   `yaml:"audience"`
	RequiredScopes []string `yaml:"required_scopes"`
	RequiredRoles  []string `yaml:"required_roles"`
	RequiredGroups []string `yaml:"required_groups"`

	Mode         string   `yaml:"mode"` // auto|client_credentials|auth_code
	ClientID     string   `yaml:"client_id"`
	ClientSecret string   `yaml:"client_secret"`
	Scopes       []string `yaml:"scopes"`

	RedirectURL  string `yaml:"redirect_url"`
	AuthorizeURL string `yaml:"authorize_url"`
	TokenURL     string `yaml:"token_url"`
	JWKSURL      string `yaml:"jwks_url"`
}
