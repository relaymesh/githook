package worker

import (
	"context"
	"testing"
)

func TestTenantContextHelpers(t *testing.T) {
	ctx := context.Background()
	if TenantIDFromContext(ctx) != "" {
		t.Fatalf("expected empty tenant id")
	}
	ctx = WithTenantID(ctx, " tenant ")
	if TenantIDFromContext(ctx) != "tenant" {
		t.Fatalf("unexpected tenant id")
	}
	ctx = WithTenantID(ctx, " ")
	if TenantIDFromContext(ctx) != "tenant" {
		t.Fatalf("expected unchanged tenant id")
	}
}
