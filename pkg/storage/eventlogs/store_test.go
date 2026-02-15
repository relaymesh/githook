package eventlogs

import (
	"context"
	"testing"
	"time"

	"githook/pkg/storage"
)

func TestEventLogsStoreCRUD(t *testing.T) {
	store, err := Open(Config{Driver: "sqlite", DSN: ":memory:", AutoMigrate: true})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	ctx := storage.WithTenant(context.Background(), "tenant-a")
	now := time.Now().UTC()
	if err := store.CreateEventLogs(ctx, []storage.EventLogRecord{
		{ID: "id-1", Provider: "github", Name: "push", RequestID: "req-1", CreatedAt: now, Matched: true},
		{ID: "id-2", Provider: "gitlab", Name: "merge", RequestID: "req-2", CreatedAt: now.Add(time.Minute)},
	}); err != nil {
		t.Fatalf("create event logs: %v", err)
	}

	list, err := store.ListEventLogs(ctx, storage.EventLogFilter{Provider: "github"})
	if err != nil || len(list) != 1 {
		t.Fatalf("list event logs: %v", err)
	}

	if err := store.UpdateEventLogStatus(ctx, "id-1", "success", ""); err != nil {
		t.Fatalf("update status: %v", err)
	}

	analytics, err := store.GetEventLogAnalytics(ctx, storage.EventLogFilter{})
	if err != nil || analytics.Total == 0 {
		t.Fatalf("analytics: %v", err)
	}

	_, err = store.GetEventLogTimeseries(ctx, storage.EventLogFilter{
		StartTime: now.Add(-time.Hour),
		EndTime:   now.Add(time.Hour),
	}, storage.EventLogIntervalHour)
	if err != nil {
		t.Fatalf("timeseries: %v", err)
	}

	_, _, err = store.GetEventLogBreakdown(ctx, storage.EventLogFilter{}, storage.EventLogBreakdownProvider, storage.EventLogBreakdownSortCount, true, 10, "", false)
	if err != nil {
		t.Fatalf("breakdown: %v", err)
	}
}
