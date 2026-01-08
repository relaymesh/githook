package storage

import (
	"context"
	"strings"
)

type tenantKey struct{}

// WithTenant attaches a tenant ID to a context.
func WithTenant(ctx context.Context, tenantID string) context.Context {
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		return ctx
	}
	return context.WithValue(ctx, tenantKey{}, tenantID)
}

// TenantFromContext returns the tenant ID stored in the context, if any.
func TenantFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if value, ok := ctx.Value(tenantKey{}).(string); ok {
		return strings.TrimSpace(value)
	}
	return ""
}
