package auth

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

type stringList []string

func (s *stringList) UnmarshalYAML(value *yaml.Node) error {
	if value == nil {
		return nil
	}
	switch value.Kind {
	case yaml.ScalarNode:
		if value.Tag == "!!null" {
			*s = nil
			return nil
		}
		*s = parseStringList(value.Value)
		return nil
	case yaml.SequenceNode:
		var out []string
		if err := value.Decode(&out); err != nil {
			return err
		}
		*s = out
		return nil
	default:
		return fmt.Errorf("unsupported list value %q", value.Value)
	}
}

func parseStringList(value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	if strings.HasPrefix(value, "[") && strings.HasSuffix(value, "]") {
		var out []string
		if err := yaml.Unmarshal([]byte(value), &out); err == nil {
			return out
		}
	}
	fields := strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == ' ' || r == '\t' || r == '\n' || r == '\r'
	})
	out := make([]string, 0, len(fields))
	for _, field := range fields {
		trimmed := strings.TrimSpace(field)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

// UnmarshalYAML allows scopes to be specified as a list or a string.
func (c *OAuthConfig) UnmarshalYAML(value *yaml.Node) error {
	type oauthConfigYAML struct {
		ClientID     string     `yaml:"client_id"`
		ClientSecret string     `yaml:"client_secret"`
		Scopes       stringList `yaml:"scopes"`
	}
	var aux oauthConfigYAML
	if err := value.Decode(&aux); err != nil {
		return err
	}
	c.ClientID = aux.ClientID
	c.ClientSecret = aux.ClientSecret
	c.Scopes = []string(aux.Scopes)
	return nil
}

// UnmarshalYAML allows list fields to be specified as a list or a string.
func (c *OAuth2Config) UnmarshalYAML(value *yaml.Node) error {
	type oauth2ConfigYAML struct {
		Enabled        bool       `yaml:"enabled"`
		Issuer         string     `yaml:"issuer"`
		Audience       string     `yaml:"audience"`
		RequiredScopes stringList `yaml:"required_scopes"`
		RequiredRoles  stringList `yaml:"required_roles"`
		RequiredGroups stringList `yaml:"required_groups"`
		Mode           string     `yaml:"mode"`
		ClientID       string     `yaml:"client_id"`
		ClientSecret   string     `yaml:"client_secret"`
		Scopes         stringList `yaml:"scopes"`
		RedirectURL    string     `yaml:"redirect_url"`
		AuthorizeURL   string     `yaml:"authorize_url"`
		TokenURL       string     `yaml:"token_url"`
		JWKSURL        string     `yaml:"jwks_url"`
	}
	var aux oauth2ConfigYAML
	if err := value.Decode(&aux); err != nil {
		return err
	}
	*c = OAuth2Config{
		Enabled:        aux.Enabled,
		Issuer:         aux.Issuer,
		Audience:       aux.Audience,
		RequiredScopes: []string(aux.RequiredScopes),
		RequiredRoles:  []string(aux.RequiredRoles),
		RequiredGroups: []string(aux.RequiredGroups),
		Mode:           aux.Mode,
		ClientID:       aux.ClientID,
		ClientSecret:   aux.ClientSecret,
		Scopes:         []string(aux.Scopes),
		RedirectURL:    aux.RedirectURL,
		AuthorizeURL:   aux.AuthorizeURL,
		TokenURL:       aux.TokenURL,
		JWKSURL:        aux.JWKSURL,
	}
	return nil
}
