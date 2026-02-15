package server

import (
	"context"
	"strings"
	"testing"

	"githook/pkg/storage"
)

func TestServerHelperFunctions(t *testing.T) {
	if !isProviderInstanceHash(strings.Repeat("a", 64)) {
		t.Fatalf("expected valid hash")
	}
	if isProviderInstanceHash("bad") {
		t.Fatalf("expected invalid hash")
	}

	if _, err := randomHex(0); err == nil {
		t.Fatalf("expected size error")
	}
	value, err := randomHex(8)
	if err != nil || len(value) == 0 {
		t.Fatalf("expected random hex")
	}

	key := ruleKey(" action ", []string{"b", "a", " "}, "driver-x")
	if !strings.HasPrefix(key, "action") || !strings.Contains(key, "a,b") {
		t.Fatalf("unexpected rule key: %q", key)
	}
}

func TestResolveRuleDrivers(t *testing.T) {
	ctx := storage.WithTenant(context.Background(), "tenant-a")
	store := storage.NewMockDriverStore()
	driver, err := store.UpsertDriver(ctx, storage.DriverRecord{Name: "amqp", Enabled: true})
	if err != nil {
		t.Fatalf("upsert driver: %v", err)
	}
	name, err := resolveRuleDriverName(ctx, store, driver.ID)
	if err != nil {
		t.Fatalf("resolve name: %v", err)
	}
	if name != "amqp" {
		t.Fatalf("unexpected driver name: %q", name)
	}
}
