package api

import (
	"context"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	cloudv1 "githook/pkg/gen/cloud/v1"
	"githook/pkg/storage"
)

func TestInstallationsServiceLifecycle(t *testing.T) {
	store := storage.NewMockStore()
	service := &InstallationsService{Store: store}
	ctx := storage.WithTenant(context.Background(), "tenant-a")

	upsertReq := connect.NewRequest(&cloudv1.UpsertInstallationRequest{
		Installation: &cloudv1.InstallRecord{
			Provider:       "github",
			AccountId:      "acct",
			AccountName:    "account",
			InstallationId: "inst",
		},
	})
	if _, err := service.UpsertInstallation(ctx, upsertReq); err != nil {
		t.Fatalf("upsert installation: %v", err)
	}

	listResp, err := service.ListInstallations(ctx, connect.NewRequest(&cloudv1.ListInstallationsRequest{
		Provider: "github",
	}))
	if err != nil {
		t.Fatalf("list installations: %v", err)
	}
	if len(listResp.Msg.GetInstallations()) != 1 {
		t.Fatalf("expected 1 installation")
	}

	if _, err := service.GetInstallationByID(ctx, connect.NewRequest(&cloudv1.GetInstallationByIDRequest{
		Provider:       "github",
		InstallationId: "missing",
	})); connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("expected not found error")
	}

	if _, err := service.DeleteInstallation(ctx, connect.NewRequest(&cloudv1.DeleteInstallationRequest{
		Provider:       "github",
		AccountId:      "acct",
		InstallationId: "inst",
	})); err != nil {
		t.Fatalf("delete installation: %v", err)
	}
}

func TestDriversServiceLifecycle(t *testing.T) {
	store := storage.NewMockDriverStore()
	service := &DriversService{Store: store}
	ctx := storage.WithTenant(context.Background(), "tenant-a")

	if _, err := service.UpsertDriver(ctx, connect.NewRequest(&cloudv1.UpsertDriverRequest{
		Driver: &cloudv1.DriverRecord{Name: "gochannel", ConfigJson: "{}", Enabled: true},
	})); err != nil {
		t.Fatalf("upsert driver: %v", err)
	}

	listResp, err := service.ListDrivers(ctx, connect.NewRequest(&cloudv1.ListDriversRequest{}))
	if err != nil {
		t.Fatalf("list drivers: %v", err)
	}
	if len(listResp.Msg.GetDrivers()) != 1 {
		t.Fatalf("expected 1 driver")
	}

	if _, err := service.GetDriver(ctx, connect.NewRequest(&cloudv1.GetDriverRequest{Name: "missing"})); connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("expected not found error")
	}

	if _, err := service.DeleteDriver(ctx, connect.NewRequest(&cloudv1.DeleteDriverRequest{Name: "amqp"})); err != nil {
		t.Fatalf("delete driver: %v", err)
	}
}

func TestRulesServiceCreateUpdateMatch(t *testing.T) {
	ruleStore := storage.NewMockRuleStore()
	driverStore := storage.NewMockDriverStore()
	ctx := storage.WithTenant(context.Background(), "tenant-a")
	driver, err := driverStore.UpsertDriver(ctx, storage.DriverRecord{Name: "gochannel", Enabled: true})
	if err != nil {
		t.Fatalf("upsert driver: %v", err)
	}

	service := &RulesService{Store: ruleStore, DriverStore: driverStore}
	createResp, err := service.CreateRule(ctx, connect.NewRequest(&cloudv1.CreateRuleRequest{
		Rule: &cloudv1.Rule{
			When:     "action == \"opened\"",
			Emit:     []string{"pr.opened"},
			DriverId: driver.ID,
		},
	}))
	if err != nil {
		t.Fatalf("create rule: %v", err)
	}

	ruleID := createResp.Msg.GetRule().GetId()
	if ruleID == "" {
		t.Fatalf("expected rule id")
	}

	if _, err := service.UpdateRule(ctx, connect.NewRequest(&cloudv1.UpdateRuleRequest{
		Id: ruleID,
		Rule: &cloudv1.Rule{
			When:     "action == \"closed\"",
			Emit:     []string{"pr.closed"},
			DriverId: driver.ID,
		},
	})); err != nil {
		t.Fatalf("update rule: %v", err)
	}

	matchResp, err := service.MatchRules(ctx, connect.NewRequest(&cloudv1.MatchRulesRequest{
		Event: &cloudv1.EventPayload{
			Provider: "github",
			Name:     "pull_request",
			Payload:  []byte(`{"action":"closed"}`),
		},
		Rules: []*cloudv1.Rule{
			{When: "action == \"closed\"", Emit: []string{"pr.closed"}, DriverId: driver.ID},
		},
	}))
	if err != nil {
		t.Fatalf("match rules: %v", err)
	}
	if len(matchResp.Msg.GetMatches()) != 1 {
		t.Fatalf("expected 1 match")
	}
}

func TestProvidersServiceLifecycle(t *testing.T) {
	store := storage.NewMockProviderInstanceStore()
	service := &ProvidersService{Store: store}
	ctx := storage.WithTenant(context.Background(), "tenant-a")

	resp, err := service.UpsertProvider(ctx, connect.NewRequest(&cloudv1.UpsertProviderRequest{
		Provider: &cloudv1.ProviderRecord{
			Provider:   "github",
			ConfigJson: "{}",
			Enabled:    true,
		},
	}))
	if err != nil {
		t.Fatalf("upsert provider: %v", err)
	}
	hash := resp.Msg.GetProvider().GetHash()
	if len(hash) != 64 {
		t.Fatalf("expected hash to be generated")
	}

	listResp, err := service.ListProviders(ctx, connect.NewRequest(&cloudv1.ListProvidersRequest{
		Provider: "github",
	}))
	if err != nil {
		t.Fatalf("list providers: %v", err)
	}
	if len(listResp.Msg.GetProviders()) != 1 {
		t.Fatalf("expected 1 provider")
	}

	if _, err := service.DeleteProvider(ctx, connect.NewRequest(&cloudv1.DeleteProviderRequest{
		Provider: "github",
		Hash:     hash,
	})); err != nil {
		t.Fatalf("delete provider: %v", err)
	}
}

func TestEventLogsServiceLifecycle(t *testing.T) {
	store := storage.NewMockEventLogStore()
	service := &EventLogsService{Store: store}
	ctx := storage.WithTenant(context.Background(), "tenant-a")
	now := time.Now().UTC()
	if err := store.CreateEventLogs(ctx, []storage.EventLogRecord{
		{ID: "id-1", Provider: "github", Name: "push", RequestID: "req-1", CreatedAt: now, Matched: true},
		{ID: "id-2", Provider: "gitlab", Name: "merge", RequestID: "req-2", CreatedAt: now.Add(time.Minute)},
	}); err != nil {
		t.Fatalf("create event logs: %v", err)
	}

	listResp, err := service.ListEventLogs(ctx, connect.NewRequest(&cloudv1.ListEventLogsRequest{
		PageSize: 1,
	}))
	if err != nil {
		t.Fatalf("list event logs: %v", err)
	}
	if len(listResp.Msg.GetLogs()) != 1 || listResp.Msg.GetNextPageToken() == "" {
		t.Fatalf("expected paginated results")
	}

	analyticsResp, err := service.GetEventLogAnalytics(ctx, connect.NewRequest(&cloudv1.GetEventLogAnalyticsRequest{}))
	if err != nil {
		t.Fatalf("get analytics: %v", err)
	}
	if analyticsResp.Msg.GetAnalytics().GetTotal() == 0 {
		t.Fatalf("expected analytics totals")
	}

	_, err = service.GetEventLogTimeseries(ctx, connect.NewRequest(&cloudv1.GetEventLogTimeseriesRequest{
		StartTime: timestamppb.New(now.Add(-time.Hour)),
		EndTime:   timestamppb.New(now.Add(time.Hour)),
		Interval:  cloudv1.EventLogTimeseriesInterval_EVENT_LOG_TIMESERIES_INTERVAL_HOUR,
	}))
	if err != nil {
		t.Fatalf("get timeseries: %v", err)
	}

	_, err = service.GetEventLogBreakdown(ctx, connect.NewRequest(&cloudv1.GetEventLogBreakdownRequest{
		GroupBy: cloudv1.EventLogBreakdownGroup_EVENT_LOG_BREAKDOWN_GROUP_PROVIDER,
	}))
	if err != nil {
		t.Fatalf("get breakdown: %v", err)
	}

	if _, err := service.UpdateEventLogStatus(ctx, connect.NewRequest(&cloudv1.UpdateEventLogStatusRequest{
		LogId:  "id-1",
		Status: "success",
	})); err != nil {
		t.Fatalf("update status: %v", err)
	}
}

func TestNamespacesServiceList(t *testing.T) {
	store := storage.NewMockNamespaceStore()
	service := &NamespacesService{Store: store}
	ctx := storage.WithTenant(context.Background(), "tenant-a")
	if err := store.UpsertNamespace(ctx, storage.NamespaceRecord{
		Provider: "github",
		RepoID:   "1",
		Owner:    "org",
		RepoName: "repo",
		FullName: "org/repo",
	}); err != nil {
		t.Fatalf("upsert namespace: %v", err)
	}
	resp, err := service.ListNamespaces(ctx, connect.NewRequest(&cloudv1.ListNamespacesRequest{
		Provider: "github",
		Owner:    "org",
		Repo:     "repo",
	}))
	if err != nil {
		t.Fatalf("list namespaces: %v", err)
	}
	if len(resp.Msg.GetNamespaces()) != 1 {
		t.Fatalf("expected 1 namespace")
	}
}

func TestRuleHelpersAndPagination(t *testing.T) {
	if _, _, _, err := parseRuleInput(nil); err == nil {
		t.Fatalf("expected missing rule error")
	}
	when, emit, driverID, err := parseRuleInput(&cloudv1.Rule{
		When:     " when ",
		Emit:     []string{"topic"},
		DriverId: " driver ",
	})
	if err != nil || when != "when" || emit[0] != "topic" || driverID != "driver" {
		t.Fatalf("unexpected parse output")
	}

	normalized, err := normalizeCoreRule("  action == \"opened\"  ", []string{"topic"}, "amqp", "amqp")
	if err != nil || strings.TrimSpace(normalized.When) == "" {
		t.Fatalf("expected normalized rule")
	}

	if _, err := decodePageToken("bad"); err == nil {
		t.Fatalf("expected invalid page token error")
	}
	if token := encodePageToken(0); token != "" {
		t.Fatalf("expected empty token")
	}
	if token := encodePageTokenFromRaw("5"); token == "" {
		t.Fatalf("expected encoded token")
	}
	if _, _, _, err := parseRuleInput(&cloudv1.Rule{
		When:     "when",
		Emit:     []string{"one", "two"},
		DriverId: "driver",
	}); err == nil {
		t.Fatalf("expected emit count validation error")
	}
}
