package auth

import (
	"reflect"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestOAuthConfigScopesString(t *testing.T) {
	input := "client_id: test\nclient_secret: secret\nscopes: read:user,repo\n"
	var cfg OAuthConfig
	if err := yaml.Unmarshal([]byte(input), &cfg); err != nil {
		t.Fatalf("unmarshal oauth config: %v", err)
	}
	want := []string{"read:user", "repo"}
	if !reflect.DeepEqual(cfg.Scopes, want) {
		t.Fatalf("expected scopes %v, got %v", want, cfg.Scopes)
	}
}

func TestOAuth2ConfigScopesString(t *testing.T) {
	input := "enabled: true\nrequired_scopes: openid profile\nscopes: read:user\n"
	var cfg OAuth2Config
	if err := yaml.Unmarshal([]byte(input), &cfg); err != nil {
		t.Fatalf("unmarshal oauth2 config: %v", err)
	}
	wantRequired := []string{"openid", "profile"}
	if !reflect.DeepEqual(cfg.RequiredScopes, wantRequired) {
		t.Fatalf("expected required scopes %v, got %v", wantRequired, cfg.RequiredScopes)
	}
	wantScopes := []string{"read:user"}
	if !reflect.DeepEqual(cfg.Scopes, wantScopes) {
		t.Fatalf("expected scopes %v, got %v", wantScopes, cfg.Scopes)
	}
}
