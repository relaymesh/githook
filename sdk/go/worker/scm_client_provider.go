package worker

import (
	"context"
	"errors"

	"githooks/pkg/auth"
	"githooks/pkg/scm"
)

// SCMClientProvider resolves SCM clients from webhook events.
type SCMClientProvider struct {
	resolver            auth.Resolver
	factory             *scm.Factory
	installationsClient *InstallationsClient
}

// NewSCMClientProvider creates a provider that resolves auth and builds SCM clients.
func NewSCMClientProvider(cfg auth.Config) *SCMClientProvider {
	return &SCMClientProvider{
		resolver: auth.NewResolver(cfg),
		factory:  scm.NewFactory(cfg),
		installationsClient: &InstallationsClient{
			BaseURL: installationsBaseURL(),
		},
	}
}

// NewSCMClientProviderWithResolver creates a provider with custom resolver/factory.
func NewSCMClientProviderWithResolver(resolver auth.Resolver, factory *scm.Factory) *SCMClientProvider {
	return &SCMClientProvider{
		resolver: resolver,
		factory:  factory,
		installationsClient: &InstallationsClient{
			BaseURL: installationsBaseURL(),
		},
	}
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
	case "gitlab", "bitbucket":
		client := p.installationsClient
		if client == nil || client.BaseURL == "" {
			client = &InstallationsClient{BaseURL: installationsBaseURL()}
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
