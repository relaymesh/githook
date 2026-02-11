package worker

import (
	"context"
	"strings"
)

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

type tenantKey struct{}

// WithTenantID attaches a tenant ID to the context for API calls.
func WithTenantID(ctx context.Context, tenantID string) context.Context {
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		return ctx
	}
	return context.WithValue(ctx, tenantKey{}, tenantID)
}

// TenantIDFromContext returns the tenant ID stored in the context, if any.
func TenantIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if value, ok := ctx.Value(tenantKey{}).(string); ok {
		return strings.TrimSpace(value)
	}
	return ""
}
