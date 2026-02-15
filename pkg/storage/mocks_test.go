package storage

import (
	"context"
	"testing"
	"time"
)

func TestMockStoreInstallations(t *testing.T) {
	store := NewMockStore()
	ctx := WithTenant(context.Background(), "tenant-a")
	record := InstallRecord{
		Provider:            "github",
		AccountID:           "acct",
		InstallationID:      "inst",
		ProviderInstanceKey: "key",
	}
	if err := store.UpsertInstallation(ctx, record); err != nil {
		t.Fatalf("upsert installation: %v", err)
	}
	got, err := store.GetInstallation(ctx, "github", "acct", "inst")
	if err != nil || got == nil {
		t.Fatalf("get installation: %v", err)
	}
	if got.TenantID != "tenant-a" {
		t.Fatalf("expected tenant id, got %q", got.TenantID)
	}

	list, err := store.ListInstallations(ctx, "github", "")
	if err != nil || len(list) != 1 {
		t.Fatalf("list installations: %v", err)
	}

	gotByID, err := store.GetInstallationByInstallationID(ctx, "github", "inst")
	if err != nil || gotByID == nil {
		t.Fatalf("get installation by id: %v", err)
	}

	if err := store.DeleteInstallation(ctx, "github", "acct", "inst", ""); err != nil {
		t.Fatalf("delete installation: %v", err)
	}
	list, _ = store.ListInstallations(ctx, "github", "")
	if len(list) != 0 {
		t.Fatalf("expected empty list after delete")
	}
}

func TestMockNamespaceStore(t *testing.T) {
	store := NewMockNamespaceStore()
	ctx := WithTenant(context.Background(), "tenant-a")
	record := NamespaceRecord{
		Provider:            "github",
		ProviderInstanceKey: "key",
		RepoID:              "1",
		Owner:               "org",
		RepoName:            "repo",
		FullName:            "org/repo",
	}
	if err := store.UpsertNamespace(ctx, record); err != nil {
		t.Fatalf("upsert namespace: %v", err)
	}
	got, err := store.GetNamespace(ctx, "github", "1", "key")
	if err != nil || got == nil {
		t.Fatalf("get namespace: %v", err)
	}
	list, err := store.ListNamespaces(ctx, NamespaceFilter{Provider: "github"})
	if err != nil || len(list) != 1 {
		t.Fatalf("list namespaces: %v", err)
	}
	if err := store.DeleteNamespace(ctx, "github", "1", "key"); err != nil {
		t.Fatalf("delete namespace: %v", err)
	}
	list, _ = store.ListNamespaces(ctx, NamespaceFilter{Provider: "github"})
	if len(list) != 0 {
		t.Fatalf("expected empty list after delete")
	}
}

func TestMockRuleStore(t *testing.T) {
	store := NewMockRuleStore()
	ctx := WithTenant(context.Background(), "tenant-a")
	created, err := store.CreateRule(ctx, RuleRecord{When: "a == 1", Emit: []string{"topic"}, DriverID: "driver"})
	if err != nil || created == nil {
		t.Fatalf("create rule: %v", err)
	}
	if created.ID == "" {
		t.Fatalf("expected rule id")
	}
	updated, err := store.UpdateRule(ctx, RuleRecord{ID: created.ID, When: "a == 2", Emit: []string{"topic"}, DriverID: "driver"})
	if err != nil || updated == nil {
		t.Fatalf("update rule: %v", err)
	}
	got, err := store.GetRule(ctx, created.ID)
	if err != nil || got == nil {
		t.Fatalf("get rule: %v", err)
	}
	list, err := store.ListRules(ctx)
	if err != nil || len(list) != 1 {
		t.Fatalf("list rules: %v", err)
	}
	if err := store.DeleteRule(ctx, created.ID); err != nil {
		t.Fatalf("delete rule: %v", err)
	}
	list, _ = store.ListRules(ctx)
	if len(list) != 0 {
		t.Fatalf("expected empty list after delete")
	}
}

func TestMockProviderInstanceStore(t *testing.T) {
	store := NewMockProviderInstanceStore()
	ctx := WithTenant(context.Background(), "tenant-a")
	record := ProviderInstanceRecord{Provider: "github", Key: "default", Enabled: true}
	created, err := store.UpsertProviderInstance(ctx, record)
	if err != nil || created == nil {
		t.Fatalf("upsert provider instance: %v", err)
	}
	got, err := store.GetProviderInstance(ctx, "github", "default")
	if err != nil || got == nil {
		t.Fatalf("get provider instance: %v", err)
	}
	list, err := store.ListProviderInstances(ctx, "github")
	if err != nil || len(list) != 1 {
		t.Fatalf("list provider instances: %v", err)
	}
	if err := store.DeleteProviderInstance(ctx, "github", "default"); err != nil {
		t.Fatalf("delete provider instance: %v", err)
	}
	list, _ = store.ListProviderInstances(ctx, "github")
	if len(list) != 0 {
		t.Fatalf("expected empty list after delete")
	}
}

func TestMockDriverStore(t *testing.T) {
	store := NewMockDriverStore()
	ctx := WithTenant(context.Background(), "tenant-a")
	created, err := store.UpsertDriver(ctx, DriverRecord{Name: "amqp", ConfigJSON: "{}"})
	if err != nil || created == nil {
		t.Fatalf("upsert driver: %v", err)
	}
	if created.ID == "" {
		t.Fatalf("expected driver id")
	}
	got, err := store.GetDriver(ctx, "amqp")
	if err != nil || got == nil {
		t.Fatalf("get driver: %v", err)
	}
	gotByID, err := store.GetDriverByID(ctx, created.ID)
	if err != nil || gotByID == nil {
		t.Fatalf("get driver by id: %v", err)
	}
	if err := store.DeleteDriver(ctx, "amqp"); err != nil {
		t.Fatalf("delete driver: %v", err)
	}
	got, _ = store.GetDriver(ctx, "amqp")
	if got != nil {
		t.Fatalf("expected driver deleted")
	}
}

func TestMockEventLogStore(t *testing.T) {
	store := NewMockEventLogStore()
	ctx := WithTenant(context.Background(), "tenant-a")
	base := time.Date(2026, 2, 14, 0, 0, 0, 0, time.UTC)
	records := []EventLogRecord{
		{
			ID:             "id-1",
			Provider:       "github",
			Name:           "push",
			RequestID:      "req-1",
			InstallationID: "inst-1",
			NamespaceName:  "org/repo",
			Topic:          "topic-1",
			RuleID:         "rule-1",
			Matched:        true,
			Status:         "queued",
			LatencyMS:      10,
			CreatedAt:      base,
		},
		{
			ID:             "id-2",
			Provider:       "gitlab",
			Name:           "merge",
			RequestID:      "req-2",
			InstallationID: "inst-2",
			NamespaceID:    "99",
			Topic:          "topic-2",
			RuleWhen:       "a == 1",
			Matched:        false,
			Status:         "failed",
			LatencyMS:      20,
			CreatedAt:      base.Add(30 * time.Minute),
		},
		{
			ID:             "id-3",
			Provider:       "github",
			Name:           "push",
			RequestID:      "req-1",
			InstallationID: "inst-1",
			NamespaceName:  "org/repo",
			Topic:          "topic-1",
			Matched:        true,
			Status:         "success",
			LatencyMS:      30,
			CreatedAt:      base.Add(90 * time.Minute),
		},
	}
	if err := store.CreateEventLogs(ctx, records); err != nil {
		t.Fatalf("create event logs: %v", err)
	}
	list, err := store.ListEventLogs(ctx, EventLogFilter{Provider: "github", Limit: 1})
	if err != nil || len(list) != 1 {
		t.Fatalf("list event logs: %v", err)
	}
	if list[0].ID != "id-3" {
		t.Fatalf("expected newest record first, got %q", list[0].ID)
	}

	analytics, err := store.GetEventLogAnalytics(ctx, EventLogFilter{Provider: "github"})
	if err != nil {
		t.Fatalf("event log analytics: %v", err)
	}
	if analytics.Total != 2 || analytics.Matched != 2 || analytics.DistinctReq != 1 {
		t.Fatalf("unexpected analytics: %+v", analytics)
	}
	if len(analytics.ByProvider) != 1 || analytics.ByProvider[0].Key != "github" {
		t.Fatalf("unexpected provider counts: %+v", analytics.ByProvider)
	}

	if err := store.UpdateEventLogStatus(ctx, "id-1", "success", ""); err != nil {
		t.Fatalf("update event log status: %v", err)
	}
	list, _ = store.ListEventLogs(ctx, EventLogFilter{RequestID: "req-1"})
	found := false
	for _, record := range list {
		if record.ID == "id-1" && record.Status == "success" && record.LatencyMS >= 0 {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected updated record")
	}

	timeseries, err := store.GetEventLogTimeseries(ctx, EventLogFilter{
		StartTime: base,
		EndTime:   base.Add(2 * time.Hour),
	}, EventLogIntervalHour)
	if err != nil {
		t.Fatalf("event log timeseries: %v", err)
	}
	if len(timeseries) != 3 {
		t.Fatalf("expected 3 buckets, got %d", len(timeseries))
	}
	if timeseries[0].EventCount != 2 || timeseries[1].EventCount != 1 {
		t.Fatalf("unexpected bucket counts: %+v", timeseries)
	}

	breakdown, token, err := store.GetEventLogBreakdown(ctx, EventLogFilter{}, EventLogBreakdownProvider, EventLogBreakdownSortCount, true, 1, "", true)
	if err != nil {
		t.Fatalf("event log breakdown: %v", err)
	}
	if len(breakdown) != 1 || breakdown[0].Key == "" {
		t.Fatalf("unexpected breakdown: %+v", breakdown)
	}
	if token == "" {
		t.Fatalf("expected next page token")
	}

	if _, err := store.GetEventLogTimeseries(ctx, EventLogFilter{}, ""); err == nil {
		t.Fatalf("expected interval error")
	}
	if _, err := store.GetEventLogTimeseries(ctx, EventLogFilter{StartTime: base}, EventLogIntervalHour); err == nil {
		t.Fatalf("expected missing end time error")
	}
}

func TestMockPageTokenHelpers(t *testing.T) {
	if _, err := mockParsePageToken("bad"); err == nil {
		t.Fatalf("expected invalid page token error")
	}
	if token := mockFormatPageToken(0); token != "" {
		t.Fatalf("expected empty token, got %q", token)
	}
}
