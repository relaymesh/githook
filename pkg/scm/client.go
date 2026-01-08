package scm

import (
	"context"
	"errors"

	"githooks/pkg/auth"
	"githooks/pkg/providers/bitbucket"
	"githooks/pkg/providers/github"
	"githooks/pkg/providers/gitlab"
)

// Client is a provider-specific API client instance.
// It is returned as an interface so callers can type-assert to the provider client
// without constructing it themselves.
type Client interface{}

// Factory builds SCM clients using resolved auth contexts.
type Factory struct {
	cfg auth.Config
}

// NewFactory creates a new Factory.
func NewFactory(cfg auth.Config) *Factory {
	return &Factory{cfg: cfg}
}

// NewClient creates a provider-specific client from an AuthContext.
func (f *Factory) NewClient(ctx context.Context, authCtx auth.AuthContext) (Client, error) {
	switch authCtx.Provider {
	case "github":
		return github.NewAppClient(ctx, github.AppConfig{
			AppID:          f.cfg.GitHub.App.AppID,
			PrivateKeyPath: f.cfg.GitHub.App.PrivateKeyPath,
			BaseURL:        f.cfg.GitHub.API.BaseURL,
		}, authCtx.InstallationID)
	case "gitlab":
		return gitlab.NewTokenClient(f.cfg.GitLab, authCtx.Token)
	case "bitbucket":
		return bitbucket.NewTokenClient(f.cfg.Bitbucket, authCtx.Token)
	default:
		return nil, errors.New("unsupported provider for scm client")
	}
}
