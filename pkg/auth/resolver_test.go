package auth

import (
	"context"
	"testing"
)

func TestResolverGitHubInstallation(t *testing.T) {
	cfg := Config{
		GitHub: ProviderConfig{
			App: AppConfig{
				AppID:          123,
				PrivateKeyPath: "/tmp/key.pem",
			},
		},
	}
	resolver := NewResolver(cfg)

	payload := []byte(`{"installation":{"id":42}}`)
	authCtx, err := resolver.Resolve(context.Background(), EventContext{
		Provider: "github",
		Payload:  payload,
	})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if authCtx.InstallationID != 42 {
		t.Fatalf("expected installation id 42, got %d", authCtx.InstallationID)
	}
}

func TestResolverGitHubMissingInstallation(t *testing.T) {
	cfg := Config{
		GitHub: ProviderConfig{
			App: AppConfig{
				AppID:          123,
				PrivateKeyPath: "/tmp/key.pem",
			},
		},
	}
	resolver := NewResolver(cfg)

	_, err := resolver.Resolve(context.Background(), EventContext{
		Provider: "github",
		Payload:  []byte(`{"installation":{}}`),
	})
	if err == nil {
		t.Fatalf("expected error for missing installation id")
	}
}

func TestResolverUnsupportedProvider(t *testing.T) {
	cfg := Config{}
	resolver := NewResolver(cfg)

	_, err := resolver.Resolve(context.Background(), EventContext{
		Provider: "gitlab",
		Payload:  []byte(`{}`),
	})
	if err == nil {
		t.Fatalf("expected error for unsupported provider")
	}
}
