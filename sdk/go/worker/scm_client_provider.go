package worker

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"githook/pkg/auth"
	"githook/pkg/scm"
)

// SCMClientProvider resolves SCM clients from webhook events.
type SCMClientProvider struct {
	resolver               auth.Resolver
	factory                *scm.Factory
	installationsClient    *InstallationsClient
	installationsClientSet bool // true when explicitly provided via WithSCMInstallationsClient
}

// SCMClientProviderOption configures an SCMClientProvider.
type SCMClientProviderOption func(*SCMClientProvider)

// WithSCMResolver sets a custom resolver and factory on the provider.
func WithSCMResolver(r auth.Resolver, f *scm.Factory) SCMClientProviderOption {
	return func(p *SCMClientProvider) {
		p.resolver = r
		p.factory = f
	}
}

// WithSCMInstallationsClient sets a custom installations client on the provider.
func WithSCMInstallationsClient(c *InstallationsClient) SCMClientProviderOption {
	return func(p *SCMClientProvider) {
		p.installationsClient = c
		p.installationsClientSet = true
	}
}

// NewSCMClientProvider creates a provider that resolves auth and builds SCM clients.
// The installations client is not created here — it is bound automatically by the
// worker (via bindInstallationsClient) so that WithEndpoint / WithAPIKey /
// WithOAuth2Config are respected without duplication.
func NewSCMClientProvider(cfg auth.Config, opts ...SCMClientProviderOption) *SCMClientProvider {
	p := &SCMClientProvider{
		resolver: auth.NewResolver(cfg),
		factory:  scm.NewFactory(cfg),
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// bindInstallationsClient sets the installations client unless one was
// explicitly provided via WithSCMInstallationsClient.
func (p *SCMClientProvider) bindInstallationsClient(c *InstallationsClient) {
	if p == nil || p.installationsClientSet {
		return
	}
	p.installationsClient = c
}

// Client resolves a provider-specific SCM client for the given event.
func (p *SCMClientProvider) Client(ctx context.Context, evt *Event) (interface{}, error) {
	if p == nil || p.resolver == nil || p.factory == nil {
		return nil, errors.New("scm client provider is not configured")
	}
	if evt == nil {
		return nil, errors.New("event is required")
	}
	switch evt.Provider {
	case "github":
		if record, err := p.resolveInstallation(ctx, evt); err == nil && record != nil {
			client, err := p.githubClient(ctx, record.InstallationID)
			if err == nil {
				return client, nil
			}
		}
		authCtx, err := p.resolver.Resolve(ctx, auth.EventContext{
			Provider: evt.Provider,
			Payload:  evt.Payload,
		})
		if err != nil {
			return nil, err
		}
		return p.factory.NewClient(ctx, authCtx)
	case "gitlab", "bitbucket":
		client := p.installationsClient
		if client == nil || client.BaseURL == "" {
			client = &InstallationsClient{
				BaseURL: resolveEndpoint(""),
				APIKey:  apiKeyFromEnv(),
			}
		}
		record, err := ResolveInstallation(ctx, evt, client)
		if err != nil {
			return nil, err
		}
		if record == nil || record.AccessToken == "" {
			return nil, errors.New("access token missing for provider")
		}
		return p.factory.NewClient(ctx, auth.AuthContext{
			Provider: evt.Provider,
			Token:    record.AccessToken,
		})
	default:
		authCtx, err := p.resolver.Resolve(ctx, auth.EventContext{
			Provider: evt.Provider,
			Payload:  evt.Payload,
		})
		if err != nil {
			return nil, err
		}
		return p.factory.NewClient(ctx, authCtx)
	}
}

func (p *SCMClientProvider) resolveInstallation(ctx context.Context, evt *Event) (*InstallationRecord, error) {
	client := p.installationsClient
	if client == nil {
		return nil, nil
	}
	return ResolveInstallation(ctx, evt, client)
}

func (p *SCMClientProvider) githubClient(ctx context.Context, installationID string) (interface{}, error) {
	trimmed := strings.TrimSpace(installationID)
	if trimmed == "" {
		return nil, errors.New("installation id is required")
	}
	id, err := strconv.ParseInt(trimmed, 10, 64)
	if err != nil {
		return nil, err
	}
	return p.factory.NewClient(ctx, auth.AuthContext{
		Provider:       auth.ProviderGitHub,
		InstallationID: id,
	})
}
