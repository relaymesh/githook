package drivers

import (
	"context"
	"testing"

	"githook/pkg/storage"
)

func TestDriverStoreCRUD(t *testing.T) {
	store, err := Open(Config{Driver: "sqlite", DSN: ":memory:", AutoMigrate: true})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	ctx := storage.WithTenant(context.Background(), "tenant-a")
	record, err := store.UpsertDriver(ctx, storage.DriverRecord{
		Name:       "amqp",
		ConfigJSON: "{}",
		Enabled:    true,
	})
	if err != nil {
		t.Fatalf("upsert driver: %v", err)
	}
	if record.ID == "" {
		t.Fatalf("expected driver id")
	}

	got, err := store.GetDriver(ctx, "amqp")
	if err != nil || got == nil {
		t.Fatalf("get driver: %v", err)
	}
	gotByID, err := store.GetDriverByID(ctx, record.ID)
	if err != nil || gotByID == nil {
		t.Fatalf("get driver by id: %v", err)
	}

	list, err := store.ListDrivers(ctx)
	if err != nil || len(list) != 1 {
		t.Fatalf("list drivers: %v", err)
	}

	if err := store.DeleteDriver(ctx, "amqp"); err != nil {
		t.Fatalf("delete driver: %v", err)
	}
}
