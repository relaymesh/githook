package worker

import "context"

// ClientProvider is an interface for creating API clients.
// This allows handlers to interact with the provider's API.
type ClientProvider interface {
	// Client returns a new API client for the given event.
	Client(ctx context.Context, evt *Event) (interface{}, error)
}

// ClientProviderFunc is a function that implements the ClientProvider interface.
type ClientProviderFunc func(ctx context.Context, evt *Event) (interface{}, error)

// Client returns a new API client by calling the underlying function.
func (fn ClientProviderFunc) Client(ctx context.Context, evt *Event) (interface{}, error) {
	return fn(ctx, evt)
}
