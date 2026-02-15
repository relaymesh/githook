package auth

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func TestParseStringList(t *testing.T) {
	list := parseStringList("read:user, repo admin")
	if len(list) != 3 || list[0] != "read:user" || list[1] != "repo" || list[2] != "admin" {
		t.Fatalf("unexpected parsed list: %v", list)
	}
	list = parseStringList("[\"a\", \"b\"]")
	if len(list) != 2 || list[0] != "a" || list[1] != "b" {
		t.Fatalf("unexpected bracket list: %v", list)
	}
	list = parseStringList(" ")
	if list != nil {
		t.Fatalf("expected nil list, got %v", list)
	}
}

func TestStringListUnmarshalYAML(t *testing.T) {
	var value stringList
	node := &yaml.Node{Kind: yaml.ScalarNode, Value: "read, write"}
	if err := value.UnmarshalYAML(node); err != nil {
		t.Fatalf("unmarshal scalar: %v", err)
	}
	if len(value) != 2 || value[0] != "read" || value[1] != "write" {
		t.Fatalf("unexpected value: %v", value)
	}

	value = nil
	node = &yaml.Node{Kind: yaml.SequenceNode}
	node.Content = []*yaml.Node{
		{Kind: yaml.ScalarNode, Value: "a"},
		{Kind: yaml.ScalarNode, Value: "b"},
	}
	if err := value.UnmarshalYAML(node); err != nil {
		t.Fatalf("unmarshal sequence: %v", err)
	}
	if len(value) != 2 || value[0] != "a" || value[1] != "b" {
		t.Fatalf("unexpected sequence: %v", value)
	}

	value = nil
	node = &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!null"}
	if err := value.UnmarshalYAML(node); err != nil {
		t.Fatalf("unmarshal null: %v", err)
	}
	if value != nil {
		t.Fatalf("expected nil value for null, got %v", value)
	}
}

func TestOAuthConfigUnmarshalYAML(t *testing.T) {
	raw := []byte("client_id: abc\nclient_secret: def\nscopes: read, write\n")
	var cfg OAuthConfig
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		t.Fatalf("unmarshal oauth config: %v", err)
	}
	if cfg.ClientID != "abc" || cfg.ClientSecret != "def" {
		t.Fatalf("unexpected client values: %+v", cfg)
	}
	if len(cfg.Scopes) != 2 || cfg.Scopes[0] != "read" || cfg.Scopes[1] != "write" {
		t.Fatalf("unexpected scopes: %v", cfg.Scopes)
	}
}

func TestOAuth2ConfigUnmarshalYAML(t *testing.T) {
	raw := []byte(`
enabled: true
issuer: https://issuer.example.com
audience: api
required_scopes: read, write
required_roles:
  - admin
scopes: [profile, email]
`)
	var cfg OAuth2Config
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		t.Fatalf("unmarshal oauth2 config: %v", err)
	}
	if !cfg.Enabled || cfg.Issuer == "" || cfg.Audience == "" {
		t.Fatalf("unexpected oauth2 fields: %+v", cfg)
	}
	if len(cfg.RequiredScopes) != 2 || cfg.RequiredScopes[0] != "read" {
		t.Fatalf("unexpected required scopes: %v", cfg.RequiredScopes)
	}
	if len(cfg.RequiredRoles) != 1 || cfg.RequiredRoles[0] != "admin" {
		t.Fatalf("unexpected required roles: %v", cfg.RequiredRoles)
	}
	if len(cfg.Scopes) != 2 || cfg.Scopes[0] != "profile" {
		t.Fatalf("unexpected scopes: %v", cfg.Scopes)
	}
}
