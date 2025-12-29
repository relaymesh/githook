package worker

import "context"

type ProviderClients struct {
	GitHub    func(ctx context.Context, evt *Event) (interface{}, error)
	GitLab    func(ctx context.Context, evt *Event) (interface{}, error)
	Bitbucket func(ctx context.Context, evt *Event) (interface{}, error)
	Default   func(ctx context.Context, evt *Event) (interface{}, error)
}

func (p ProviderClients) Client(ctx context.Context, evt *Event) (interface{}, error) {
	switch evt.Provider {
	case "github":
		if p.GitHub != nil {
			return p.GitHub(ctx, evt)
		}
	case "gitlab":
		if p.GitLab != nil {
			return p.GitLab(ctx, evt)
		}
	case "bitbucket":
		if p.Bitbucket != nil {
			return p.Bitbucket(ctx, evt)
		}
	}
	if p.Default != nil {
		return p.Default(ctx, evt)
	}
	return nil, nil
}
