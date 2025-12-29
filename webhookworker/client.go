package webhookworker

import "context"

type ClientProvider interface {
	Client(ctx context.Context, evt *Event) (interface{}, error)
}

type ClientProviderFunc func(ctx context.Context, evt *Event) (interface{}, error)

func (fn ClientProviderFunc) Client(ctx context.Context, evt *Event) (interface{}, error) {
	return fn(ctx, evt)
}
