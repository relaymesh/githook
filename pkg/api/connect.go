package api

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	"githook/pkg/auth"
	"githook/pkg/core"
	driverspkg "githook/pkg/drivers"
	cloudv1 "githook/pkg/gen/cloud/v1"
	"githook/pkg/oauth"
	"githook/pkg/providerinstance"
	"githook/pkg/storage"
)

// InstallationsService implements the Connect/GRPC InstallationsService.
type InstallationsService struct {
	Store     storage.Store
	Providers auth.Config
	Logger    *log.Logger
}

func (s *InstallationsService) ListInstallations(
	ctx context.Context,
	req *connect.Request[cloudv1.ListInstallationsRequest],
) (*connect.Response[cloudv1.ListInstallationsResponse], error) {
	if s.Store == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("storage not configured"))
	}
	stateID := strings.TrimSpace(req.Msg.GetStateId())
	provider := strings.TrimSpace(req.Msg.GetProvider())
	providers := []string{provider}
	if provider == "" {
		providers = []string{"github", "gitlab", "bitbucket"}
	}
	if s.Logger != nil {
		s.Logger.Printf("installations list provider=%s state_id=%s tenant=%s", provider, stateID, storage.TenantFromContext(ctx))
	}

	var records []storage.InstallRecord
	for _, item := range providers {
		if strings.TrimSpace(item) == "" {
			continue
		}
		items, err := s.Store.ListInstallations(ctx, item, stateID)
		if err != nil {
			logError(s.Logger, "list installations failed", err)
			return nil, connect.NewError(connect.CodeInternal, errors.New("list installations failed"))
		}
		records = append(records, items...)
	}

	resp := &cloudv1.ListInstallationsResponse{
		Installations: toProtoInstallations(records),
	}
	return connect.NewResponse(resp), nil
}

func (s *InstallationsService) GetInstallationByID(
	ctx context.Context,
	req *connect.Request[cloudv1.GetInstallationByIDRequest],
) (*connect.Response[cloudv1.GetInstallationByIDResponse], error) {
	if s.Store == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("storage not configured"))
	}
	provider := strings.TrimSpace(req.Msg.GetProvider())
	installationID := strings.TrimSpace(req.Msg.GetInstallationId())
	record, err := s.Store.GetInstallationByInstallationID(ctx, provider, installationID)
	if err != nil {
		logError(s.Logger, "get installation failed", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("get installation failed"))
	}
	if record == nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("installation not found"))
	}
	resp := &cloudv1.GetInstallationByIDResponse{
		Installation: toProtoInstallation(*record),
	}
	return connect.NewResponse(resp), nil
}

func (s *InstallationsService) UpsertInstallation(
	ctx context.Context,
	req *connect.Request[cloudv1.UpsertInstallationRequest],
) (*connect.Response[cloudv1.UpsertInstallationResponse], error) {
	if s.Store == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("storage not configured"))
	}
	install := req.Msg.GetInstallation()
	provider := strings.TrimSpace(install.GetProvider())
	record := storage.InstallRecord{
		Provider:            provider,
		AccountID:           strings.TrimSpace(install.GetAccountId()),
		AccountName:         strings.TrimSpace(install.GetAccountName()),
		InstallationID:      strings.TrimSpace(install.GetInstallationId()),
		ProviderInstanceKey: strings.TrimSpace(install.GetProviderInstanceKey()),
		AccessToken:         strings.TrimSpace(install.GetAccessToken()),
		RefreshToken:        strings.TrimSpace(install.GetRefreshToken()),
		ExpiresAt:           fromProtoTimestampPtr(install.GetExpiresAt()),
		MetadataJSON:        strings.TrimSpace(install.GetMetadataJson()),
	}
	if record.CreatedAt.IsZero() {
		record.CreatedAt = time.Now().UTC()
	}
	record.UpdatedAt = time.Now().UTC()
	if err := s.Store.UpsertInstallation(ctx, record); err != nil {
		logError(s.Logger, "upsert installation failed", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("upsert installation failed"))
	}
	resp := &cloudv1.UpsertInstallationResponse{
		Installation: toProtoInstallation(record),
	}
	return connect.NewResponse(resp), nil
}

func (s *InstallationsService) DeleteInstallation(
	ctx context.Context,
	req *connect.Request[cloudv1.DeleteInstallationRequest],
) (*connect.Response[cloudv1.DeleteInstallationResponse], error) {
	if s.Store == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("storage not configured"))
	}
	provider := strings.TrimSpace(req.Msg.GetProvider())
	accountID := strings.TrimSpace(req.Msg.GetAccountId())
	installationID := strings.TrimSpace(req.Msg.GetInstallationId())
	instanceKey := strings.TrimSpace(req.Msg.GetProviderInstanceKey())
	if err := s.Store.DeleteInstallation(ctx, provider, accountID, installationID, instanceKey); err != nil {
		logError(s.Logger, "delete installation failed", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("delete installation failed"))
	}
	return connect.NewResponse(&cloudv1.DeleteInstallationResponse{}), nil
}

// NamespacesService implements the Connect/GRPC NamespacesService.
type NamespacesService struct {
	Store                 storage.NamespaceStore
	InstallStore          storage.Store
	ProviderInstanceStore storage.ProviderInstanceStore
	ProviderInstanceCache *providerinstance.Cache
	Providers             auth.Config
	Endpoint              string
	Logger                *log.Logger
}

// RulesService implements rule matching over a payload with inline rules.
type RulesService struct {
	Store       storage.RuleStore
	DriverStore storage.DriverStore
	Engine      *core.RuleEngine
	Strict      bool
	Logger      *log.Logger
}

// DriversService handles CRUD for driver configs.
type DriversService struct {
	Store  storage.DriverStore
	Cache  *driverspkg.Cache
	Logger *log.Logger
}

// ProvidersService handles CRUD for provider instances.
type ProvidersService struct {
	Store  storage.ProviderInstanceStore
	Cache  *providerinstance.Cache
	Logger *log.Logger
}

// EventLogsService handles queries for webhook event logs and analytics.
type EventLogsService struct {
	Store  storage.EventLogStore
	Logger *log.Logger
}

func (s *RulesService) MatchRules(
	ctx context.Context,
	req *connect.Request[cloudv1.MatchRulesRequest],
) (*connect.Response[cloudv1.MatchRulesResponse], error) {
	event := req.Msg.GetEvent()
	rules := make([]core.Rule, 0, len(req.Msg.GetRules()))
	for _, rule := range req.Msg.GetRules() {
		if rule == nil {
			continue
		}
		rules = append(rules, core.Rule{
			When:    rule.GetWhen(),
			Emit:    core.EmitList(rule.GetEmit()),
			Drivers: rule.GetDrivers(),
		})
	}

	engine, err := core.NewRuleEngine(core.RulesConfig{
		Rules:  rules,
		Strict: req.Msg.GetStrict(),
		Logger: s.Logger,
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	matches := engine.EvaluateRules(core.Event{
		Provider:   event.GetProvider(),
		Name:       event.GetName(),
		RawPayload: event.GetPayload(),
	})

	resp := &cloudv1.MatchRulesResponse{
		Matches: toProtoRuleMatches(matches),
	}
	return connect.NewResponse(resp), nil
}

func (s *DriversService) ListDrivers(
	ctx context.Context,
	req *connect.Request[cloudv1.ListDriversRequest],
) (*connect.Response[cloudv1.ListDriversResponse], error) {
	_ = req
	if s.Store == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("storage not configured"))
	}
	records, err := s.Store.ListDrivers(ctx)
	if err != nil {
		logError(s.Logger, "list drivers failed", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("list drivers failed"))
	}
	resp := &cloudv1.ListDriversResponse{
		Drivers: toProtoDriverRecords(records),
	}
	return connect.NewResponse(resp), nil
}

func (s *DriversService) GetDriver(
	ctx context.Context,
	req *connect.Request[cloudv1.GetDriverRequest],
) (*connect.Response[cloudv1.GetDriverResponse], error) {
	if s.Store == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("storage not configured"))
	}
	name := strings.TrimSpace(req.Msg.GetName())
	record, err := s.Store.GetDriver(ctx, name)
	if err != nil {
		logError(s.Logger, "get driver failed", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("get driver failed"))
	}
	if record == nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("driver not found"))
	}
	resp := &cloudv1.GetDriverResponse{
		Driver: toProtoDriverRecord(record),
	}
	return connect.NewResponse(resp), nil
}

func (s *DriversService) UpsertDriver(
	ctx context.Context,
	req *connect.Request[cloudv1.UpsertDriverRequest],
) (*connect.Response[cloudv1.UpsertDriverResponse], error) {
	if s.Store == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("storage not configured"))
	}
	driver := req.Msg.GetDriver()
	record, err := s.Store.UpsertDriver(ctx, storage.DriverRecord{
		ID:         strings.TrimSpace(driver.GetId()),
		Name:       strings.TrimSpace(driver.GetName()),
		ConfigJSON: strings.TrimSpace(driver.GetConfigJson()),
		Enabled:    driver.GetEnabled(),
	})
	if err != nil {
		logError(s.Logger, "upsert driver failed", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("upsert driver failed"))
	}
	if s.Cache != nil {
		if err := s.Cache.Refresh(ctx); err != nil {
			logError(s.Logger, "driver cache refresh failed", err)
		}
	}
	resp := &cloudv1.UpsertDriverResponse{
		Driver: toProtoDriverRecord(record),
	}
	return connect.NewResponse(resp), nil
}

func (s *DriversService) DeleteDriver(
	ctx context.Context,
	req *connect.Request[cloudv1.DeleteDriverRequest],
) (*connect.Response[cloudv1.DeleteDriverResponse], error) {
	if s.Store == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("storage not configured"))
	}
	name := strings.TrimSpace(req.Msg.GetName())
	if err := s.Store.DeleteDriver(ctx, name); err != nil {
		logError(s.Logger, "delete driver failed", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("delete driver failed"))
	}
	if s.Cache != nil {
		if err := s.Cache.Refresh(ctx); err != nil {
			logError(s.Logger, "driver cache refresh failed", err)
		}
	}
	return connect.NewResponse(&cloudv1.DeleteDriverResponse{}), nil
}

func (s *RulesService) ListRules(
	ctx context.Context,
	req *connect.Request[cloudv1.ListRulesRequest],
) (*connect.Response[cloudv1.ListRulesResponse], error) {
	_ = req
	if s.Store == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("storage not configured"))
	}
	records, err := s.Store.ListRules(ctx)
	if err != nil {
		logError(s.Logger, "list rules failed", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("list rules failed"))
	}
	resp := &cloudv1.ListRulesResponse{
		Rules: toProtoRuleRecords(records),
	}
	return connect.NewResponse(resp), nil
}

func (s *RulesService) GetRule(
	ctx context.Context,
	req *connect.Request[cloudv1.GetRuleRequest],
) (*connect.Response[cloudv1.GetRuleResponse], error) {
	if s.Store == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("storage not configured"))
	}
	id := strings.TrimSpace(req.Msg.GetId())
	record, err := s.Store.GetRule(ctx, id)
	if err != nil {
		logError(s.Logger, "get rule failed", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("get rule failed"))
	}
	if record == nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("rule not found"))
	}
	resp := &cloudv1.GetRuleResponse{
		Rule: toProtoRuleRecord(*record),
	}
	return connect.NewResponse(resp), nil
}

func (s *RulesService) CreateRule(
	ctx context.Context,
	req *connect.Request[cloudv1.CreateRuleRequest],
) (*connect.Response[cloudv1.CreateRuleResponse], error) {
	if s.Store == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("storage not configured"))
	}
	incoming := req.Msg.GetRule()
	when, emit, driverIDs, err := parseRuleInput(incoming)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	driverNames, err := s.resolveDriverNames(ctx, driverIDs)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	normalized, err := normalizeCoreRule(when, emit, driverNames)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	record, err := s.Store.CreateRule(ctx, storage.RuleRecord{
		When:    normalized.When,
		Emit:    normalized.Emit.Values(),
		Drivers: driverIDs,
	})
	if err != nil {
		logError(s.Logger, "create rule failed", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("create rule failed"))
	}
	if err := s.refreshEngine(ctx); err != nil {
		logError(s.Logger, "rule engine refresh failed", err)
	}
	resp := &cloudv1.CreateRuleResponse{
		Rule: toProtoRuleRecord(*record),
	}
	return connect.NewResponse(resp), nil
}

func (s *RulesService) UpdateRule(
	ctx context.Context,
	req *connect.Request[cloudv1.UpdateRuleRequest],
) (*connect.Response[cloudv1.UpdateRuleResponse], error) {
	if s.Store == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("storage not configured"))
	}
	id := strings.TrimSpace(req.Msg.GetId())
	incoming := req.Msg.GetRule()
	existing, err := s.Store.GetRule(ctx, id)
	if err != nil {
		logError(s.Logger, "get rule failed", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("get rule failed"))
	}
	if existing == nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("rule not found"))
	}
	when, emit, driverIDs, err := parseRuleInput(incoming)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	driverNames, err := s.resolveDriverNames(ctx, driverIDs)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	normalized, err := normalizeCoreRule(when, emit, driverNames)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	record, err := s.Store.UpdateRule(ctx, storage.RuleRecord{
		ID:        id,
		When:      normalized.When,
		Emit:      normalized.Emit.Values(),
		Drivers:   driverIDs,
		CreatedAt: existing.CreatedAt,
	})
	if err != nil {
		logError(s.Logger, "update rule failed", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("update rule failed"))
	}
	if err := s.refreshEngine(ctx); err != nil {
		logError(s.Logger, "rule engine refresh failed", err)
	}
	resp := &cloudv1.UpdateRuleResponse{
		Rule: toProtoRuleRecord(*record),
	}
	return connect.NewResponse(resp), nil
}

func (s *RulesService) DeleteRule(
	ctx context.Context,
	req *connect.Request[cloudv1.DeleteRuleRequest],
) (*connect.Response[cloudv1.DeleteRuleResponse], error) {
	if s.Store == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("storage not configured"))
	}
	id := strings.TrimSpace(req.Msg.GetId())
	if err := s.Store.DeleteRule(ctx, id); err != nil {
		logError(s.Logger, "delete rule failed", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("delete rule failed"))
	}
	if err := s.refreshEngine(ctx); err != nil {
		logError(s.Logger, "rule engine refresh failed", err)
	}
	return connect.NewResponse(&cloudv1.DeleteRuleResponse{}), nil
}

func (s *ProvidersService) ListProviders(
	ctx context.Context,
	req *connect.Request[cloudv1.ListProvidersRequest],
) (*connect.Response[cloudv1.ListProvidersResponse], error) {
	_ = req
	if s.Store == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("storage not configured"))
	}
	provider := strings.TrimSpace(req.Msg.GetProvider())
	records, err := s.Store.ListProviderInstances(ctx, provider)
	if err != nil {
		logError(s.Logger, "list provider instances failed", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("list provider instances failed"))
	}
	resp := &cloudv1.ListProvidersResponse{
		Providers: toProtoProviderRecords(records),
	}
	return connect.NewResponse(resp), nil
}

func (s *ProvidersService) GetProvider(
	ctx context.Context,
	req *connect.Request[cloudv1.GetProviderRequest],
) (*connect.Response[cloudv1.GetProviderResponse], error) {
	if s.Store == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("storage not configured"))
	}
	provider := strings.TrimSpace(req.Msg.GetProvider())
	hash := strings.TrimSpace(req.Msg.GetHash())
	record, err := s.Store.GetProviderInstance(ctx, provider, hash)
	if err != nil {
		logError(s.Logger, "get provider instance failed", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("get provider instance failed"))
	}
	resp := &cloudv1.GetProviderResponse{
		Provider: toProtoProviderRecord(record),
	}
	return connect.NewResponse(resp), nil
}

func (s *ProvidersService) UpsertProvider(
	ctx context.Context,
	req *connect.Request[cloudv1.UpsertProviderRequest],
) (*connect.Response[cloudv1.UpsertProviderResponse], error) {
	if s.Store == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("storage not configured"))
	}
	provider := req.Msg.GetProvider()
	providerName := strings.TrimSpace(provider.GetProvider())
	hash := strings.TrimSpace(provider.GetHash())
	configJSON := strings.TrimSpace(provider.GetConfigJson())
	if providerName == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("provider is required"))
	}
	existing, err := s.Store.ListProviderInstances(ctx, providerName)
	if err != nil {
		logError(s.Logger, "list provider instances failed", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("list provider instances failed"))
	}
	if hash == "" {
		if len(existing) > 0 {
			return nil, connect.NewError(connect.CodeAlreadyExists, errors.New("provider instance already exists"))
		}
		var err error
		hash, err = generateProviderInstanceHash(ctx, s.Store, providerName)
		if err != nil {
			logError(s.Logger, "generate provider instance hash failed", err)
			return nil, connect.NewError(connect.CodeInternal, errors.New("generate provider instance hash failed"))
		}
	} else if len(existing) > 0 {
		found := false
		for _, item := range existing {
			if item.Key == hash {
				found = true
				break
			}
		}
		if !found {
			return nil, connect.NewError(connect.CodeAlreadyExists, errors.New("provider instance already exists"))
		}
	}
	if configJSON == "" {
		record, err := s.Store.GetProviderInstance(ctx, providerName, hash)
		if err != nil {
			logError(s.Logger, "get provider instance failed", err)
			return nil, connect.NewError(connect.CodeInternal, errors.New("get provider instance failed"))
		}
		if record != nil {
			configJSON = strings.TrimSpace(record.ConfigJSON)
		}
	}
	record, err := s.Store.UpsertProviderInstance(ctx, storage.ProviderInstanceRecord{
		Provider:   providerName,
		Key:        hash,
		ConfigJSON: configJSON,
		Enabled:    provider.GetEnabled(),
	})
	if err != nil {
		logError(s.Logger, "upsert provider instance failed", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("upsert provider instance failed"))
	}
	if s.Cache != nil {
		if err := s.Cache.Refresh(ctx); err != nil {
			logError(s.Logger, "provider instance cache refresh failed", err)
		}
	}
	resp := &cloudv1.UpsertProviderResponse{
		Provider: toProtoProviderRecord(record),
	}
	return connect.NewResponse(resp), nil
}

func (s *ProvidersService) DeleteProvider(
	ctx context.Context,
	req *connect.Request[cloudv1.DeleteProviderRequest],
) (*connect.Response[cloudv1.DeleteProviderResponse], error) {
	if s.Store == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("storage not configured"))
	}
	provider := strings.TrimSpace(req.Msg.GetProvider())
	hash := strings.TrimSpace(req.Msg.GetHash())
	if err := s.Store.DeleteProviderInstance(ctx, provider, hash); err != nil {
		logError(s.Logger, "delete provider instance failed", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("delete provider instance failed"))
	}
	if s.Cache != nil {
		if err := s.Cache.Refresh(ctx); err != nil {
			logError(s.Logger, "provider instance cache refresh failed", err)
		}
	}
	return connect.NewResponse(&cloudv1.DeleteProviderResponse{}), nil
}

func (s *EventLogsService) ListEventLogs(
	ctx context.Context,
	req *connect.Request[cloudv1.ListEventLogsRequest],
) (*connect.Response[cloudv1.ListEventLogsResponse], error) {
	if s.Store == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("storage not configured"))
	}
	pageSize := int(req.Msg.GetPageSize())
	if pageSize <= 0 {
		pageSize = defaultEventLogPageSize
	}
	if pageSize > maxEventLogPageSize {
		pageSize = maxEventLogPageSize
	}
	offset, err := decodePageToken(req.Msg.GetPageToken())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	var matched *bool
	if req.Msg.GetMatchedOnly() {
		value := true
		matched = &value
	}
	filter := storage.EventLogFilter{
		Provider:       strings.TrimSpace(req.Msg.GetProvider()),
		Name:           strings.TrimSpace(req.Msg.GetName()),
		Topic:          strings.TrimSpace(req.Msg.GetTopic()),
		RequestID:      strings.TrimSpace(req.Msg.GetRequestId()),
		StateID:        strings.TrimSpace(req.Msg.GetStateId()),
		InstallationID: strings.TrimSpace(req.Msg.GetInstallationId()),
		NamespaceID:    strings.TrimSpace(req.Msg.GetNamespaceId()),
		NamespaceName:  strings.TrimSpace(req.Msg.GetNamespaceName()),
		RuleID:         strings.TrimSpace(req.Msg.GetRuleId()),
		RuleWhen:       strings.TrimSpace(req.Msg.GetRuleWhen()),
		Matched:        matched,
		StartTime:      fromProtoTimestamp(req.Msg.GetStartTime()),
		EndTime:        fromProtoTimestamp(req.Msg.GetEndTime()),
		Limit:          pageSize + 1,
		Offset:         offset,
	}
	records, err := s.Store.ListEventLogs(ctx, filter)
	if err != nil {
		logError(s.Logger, "list event logs failed", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("list event logs failed"))
	}
	nextToken := ""
	if len(records) > pageSize {
		records = records[:pageSize]
		nextToken = encodePageToken(offset + pageSize)
	}
	resp := &cloudv1.ListEventLogsResponse{
		Logs:          toProtoEventLogRecords(records),
		NextPageToken: nextToken,
	}
	return connect.NewResponse(resp), nil
}

func (s *EventLogsService) GetEventLogAnalytics(
	ctx context.Context,
	req *connect.Request[cloudv1.GetEventLogAnalyticsRequest],
) (*connect.Response[cloudv1.GetEventLogAnalyticsResponse], error) {
	if s.Store == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("storage not configured"))
	}
	var matched *bool
	if req.Msg.GetMatchedOnly() {
		value := true
		matched = &value
	}
	filter := storage.EventLogFilter{
		Provider:       strings.TrimSpace(req.Msg.GetProvider()),
		Name:           strings.TrimSpace(req.Msg.GetName()),
		Topic:          strings.TrimSpace(req.Msg.GetTopic()),
		RequestID:      strings.TrimSpace(req.Msg.GetRequestId()),
		StateID:        strings.TrimSpace(req.Msg.GetStateId()),
		InstallationID: strings.TrimSpace(req.Msg.GetInstallationId()),
		NamespaceID:    strings.TrimSpace(req.Msg.GetNamespaceId()),
		NamespaceName:  strings.TrimSpace(req.Msg.GetNamespaceName()),
		RuleID:         strings.TrimSpace(req.Msg.GetRuleId()),
		RuleWhen:       strings.TrimSpace(req.Msg.GetRuleWhen()),
		Matched:        matched,
		StartTime:      fromProtoTimestamp(req.Msg.GetStartTime()),
		EndTime:        fromProtoTimestamp(req.Msg.GetEndTime()),
	}
	analytics, err := s.Store.GetEventLogAnalytics(ctx, filter)
	if err != nil {
		logError(s.Logger, "event log analytics failed", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("event log analytics failed"))
	}
	resp := &cloudv1.GetEventLogAnalyticsResponse{
		Analytics: toProtoEventLogAnalytics(analytics),
	}
	return connect.NewResponse(resp), nil
}

func (s *EventLogsService) GetEventLogTimeseries(
	ctx context.Context,
	req *connect.Request[cloudv1.GetEventLogTimeseriesRequest],
) (*connect.Response[cloudv1.GetEventLogTimeseriesResponse], error) {
	if s.Store == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("storage not configured"))
	}
	interval, err := eventLogIntervalFromProto(req.Msg.GetInterval())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	var matched *bool
	if req.Msg.GetMatchedOnly() {
		value := true
		matched = &value
	}
	filter := storage.EventLogFilter{
		Provider:       strings.TrimSpace(req.Msg.GetProvider()),
		Name:           strings.TrimSpace(req.Msg.GetName()),
		Topic:          strings.TrimSpace(req.Msg.GetTopic()),
		RequestID:      strings.TrimSpace(req.Msg.GetRequestId()),
		StateID:        strings.TrimSpace(req.Msg.GetStateId()),
		InstallationID: strings.TrimSpace(req.Msg.GetInstallationId()),
		NamespaceID:    strings.TrimSpace(req.Msg.GetNamespaceId()),
		NamespaceName:  strings.TrimSpace(req.Msg.GetNamespaceName()),
		RuleID:         strings.TrimSpace(req.Msg.GetRuleId()),
		RuleWhen:       strings.TrimSpace(req.Msg.GetRuleWhen()),
		Matched:        matched,
		StartTime:      fromProtoTimestamp(req.Msg.GetStartTime()),
		EndTime:        fromProtoTimestamp(req.Msg.GetEndTime()),
	}
	buckets, err := s.Store.GetEventLogTimeseries(ctx, filter, interval)
	if err != nil {
		logError(s.Logger, "event log timeseries failed", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("event log timeseries failed"))
	}
	resp := &cloudv1.GetEventLogTimeseriesResponse{
		Buckets: toProtoEventLogTimeseries(buckets),
	}
	return connect.NewResponse(resp), nil
}

func (s *EventLogsService) GetEventLogBreakdown(
	ctx context.Context,
	req *connect.Request[cloudv1.GetEventLogBreakdownRequest],
) (*connect.Response[cloudv1.GetEventLogBreakdownResponse], error) {
	if s.Store == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("storage not configured"))
	}
	groupBy, err := eventLogBreakdownGroupFromProto(req.Msg.GetGroupBy())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	sortBy := eventLogBreakdownSortFromProto(req.Msg.GetSortBy())
	pageSize := int(req.Msg.GetPageSize())
	if pageSize <= 0 {
		pageSize = defaultEventLogPageSize
	}
	if pageSize > maxEventLogPageSize {
		pageSize = maxEventLogPageSize
	}
	offset, err := decodePageToken(req.Msg.GetPageToken())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	var matched *bool
	if req.Msg.GetMatchedOnly() {
		value := true
		matched = &value
	}
	filter := storage.EventLogFilter{
		Provider:       strings.TrimSpace(req.Msg.GetProvider()),
		Name:           strings.TrimSpace(req.Msg.GetName()),
		Topic:          strings.TrimSpace(req.Msg.GetTopic()),
		RequestID:      strings.TrimSpace(req.Msg.GetRequestId()),
		StateID:        strings.TrimSpace(req.Msg.GetStateId()),
		InstallationID: strings.TrimSpace(req.Msg.GetInstallationId()),
		NamespaceID:    strings.TrimSpace(req.Msg.GetNamespaceId()),
		NamespaceName:  strings.TrimSpace(req.Msg.GetNamespaceName()),
		RuleID:         strings.TrimSpace(req.Msg.GetRuleId()),
		RuleWhen:       strings.TrimSpace(req.Msg.GetRuleWhen()),
		Matched:        matched,
		StartTime:      fromProtoTimestamp(req.Msg.GetStartTime()),
		EndTime:        fromProtoTimestamp(req.Msg.GetEndTime()),
	}
	breakdowns, nextToken, err := s.Store.GetEventLogBreakdown(
		ctx,
		filter,
		groupBy,
		sortBy,
		req.Msg.GetSortDesc(),
		pageSize,
		strconv.Itoa(offset),
		req.Msg.GetIncludeLatency(),
	)
	if err != nil {
		logError(s.Logger, "event log breakdown failed", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("event log breakdown failed"))
	}
	resp := &cloudv1.GetEventLogBreakdownResponse{
		Breakdowns:    toProtoEventLogBreakdowns(breakdowns),
		NextPageToken: encodePageTokenFromRaw(nextToken),
	}
	return connect.NewResponse(resp), nil
}

func (s *EventLogsService) UpdateEventLogStatus(
	ctx context.Context,
	req *connect.Request[cloudv1.UpdateEventLogStatusRequest],
) (*connect.Response[cloudv1.UpdateEventLogStatusResponse], error) {
	if s.Store == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("storage not configured"))
	}
	logID := strings.TrimSpace(req.Msg.GetLogId())
	status := strings.TrimSpace(req.Msg.GetStatus())
	errMsg := strings.TrimSpace(req.Msg.GetErrorMessage())
	if err := s.Store.UpdateEventLogStatus(ctx, logID, status, errMsg); err != nil {
		logError(s.Logger, "event log update failed", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("event log update failed"))
	}
	return connect.NewResponse(&cloudv1.UpdateEventLogStatusResponse{}), nil
}

func (s *NamespacesService) ListNamespaces(
	ctx context.Context,
	req *connect.Request[cloudv1.ListNamespacesRequest],
) (*connect.Response[cloudv1.ListNamespacesResponse], error) {
	if s.Store == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("storage not configured"))
	}
	stateID := strings.TrimSpace(req.Msg.GetStateId())

	provider := strings.TrimSpace(req.Msg.GetProvider())

	filter := storage.NamespaceFilter{
		Provider: provider,
		Owner:    strings.TrimSpace(req.Msg.GetOwner()),
		RepoName: strings.TrimSpace(req.Msg.GetRepo()),
		FullName: strings.TrimSpace(req.Msg.GetFullName()),
	}
	if stateID != "" {
		filter.AccountID = stateID
	}
	records, err := s.Store.ListNamespaces(ctx, filter)
	if err != nil {
		logError(s.Logger, "list namespaces failed", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("list namespaces failed"))
	}

	resp := &cloudv1.ListNamespacesResponse{
		Namespaces: toProtoNamespaces(records),
	}
	return connect.NewResponse(resp), nil
}

func (s *NamespacesService) SyncNamespaces(
	ctx context.Context,
	req *connect.Request[cloudv1.SyncNamespacesRequest],
) (*connect.Response[cloudv1.SyncNamespacesResponse], error) {
	if s.InstallStore == nil || s.Store == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("storage not configured"))
	}
	stateID := strings.TrimSpace(req.Msg.GetStateId())
	provider := strings.TrimSpace(req.Msg.GetProvider())

	installations, err := installationsForSync(ctx, s.InstallStore, provider, stateID)
	if err != nil {
		logError(s.Logger, "installation lookup failed", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("installation lookup failed"))
	}
	if len(installations) == 0 {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("installation not found"))
	}

	for i := range installations {
		record := installations[i]
		providerCfg, cfgErr := s.providerConfigFor(ctx, provider, record.ProviderInstanceKey)
		if cfgErr != nil {
			logError(s.Logger, "provider config lookup failed", cfgErr)
			return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("provider config missing"))
		}
		if provider != "github" && record.AccessToken == "" {
			return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("access token missing"))
		}
		accessToken := record.AccessToken
		if provider != "github" && shouldRefresh(record.ExpiresAt) && record.RefreshToken != "" {
			switch provider {
			case "gitlab":
				refreshed, err := oauth.RefreshGitLabToken(ctx, providerCfg, record.RefreshToken)
				if err != nil {
					logError(s.Logger, "gitlab token refresh failed", err)
					return nil, connect.NewError(connect.CodeInternal, errors.New("token refresh failed"))
				}
				accessToken = refreshed.AccessToken
				record.AccessToken = refreshed.AccessToken
				record.RefreshToken = refreshed.RefreshToken
				record.ExpiresAt = refreshed.ExpiresAt
			case "bitbucket":
				refreshed, err := oauth.RefreshBitbucketToken(ctx, providerCfg, record.RefreshToken)
				if err != nil {
					logError(s.Logger, "bitbucket token refresh failed", err)
					return nil, connect.NewError(connect.CodeInternal, errors.New("token refresh failed"))
				}
				accessToken = refreshed.AccessToken
				record.AccessToken = refreshed.AccessToken
				record.RefreshToken = refreshed.RefreshToken
				record.ExpiresAt = refreshed.ExpiresAt
			}
			if err := s.InstallStore.UpsertInstallation(ctx, record); err != nil {
				logError(s.Logger, "token refresh persist failed", err)
			}
		}

		switch provider {
		case "github":
			// No remote sync for GitHub; namespaces come from install webhooks.
		case "gitlab":
			if err := oauth.SyncGitLabNamespaces(ctx, s.Store, providerCfg, accessToken, record.AccountID, record.InstallationID, record.ProviderInstanceKey); err != nil {
				logError(s.Logger, "gitlab namespace sync failed", err)
				return nil, connect.NewError(connect.CodeInternal, errors.New("namespace sync failed"))
			}
		case "bitbucket":
			if err := oauth.SyncBitbucketNamespaces(ctx, s.Store, providerCfg, accessToken, record.AccountID, record.InstallationID, record.ProviderInstanceKey); err != nil {
				logError(s.Logger, "bitbucket namespace sync failed", err)
				return nil, connect.NewError(connect.CodeInternal, errors.New("namespace sync failed"))
			}
		}
	}

	filter := storage.NamespaceFilter{
		Provider: provider,
	}
	if stateID != "" {
		filter.AccountID = stateID
	}
	records, err := s.Store.ListNamespaces(ctx, filter)
	if err != nil {
		logError(s.Logger, "list namespaces failed", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("list namespaces failed"))
	}
	resp := &cloudv1.SyncNamespacesResponse{
		Namespaces: toProtoNamespaces(records),
	}
	return connect.NewResponse(resp), nil
}

func (s *NamespacesService) GetNamespaceWebhook(
	ctx context.Context,
	req *connect.Request[cloudv1.GetNamespaceWebhookRequest],
) (*connect.Response[cloudv1.GetNamespaceWebhookResponse], error) {
	if s.Store == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("storage not configured"))
	}
	provider := strings.TrimSpace(req.Msg.GetProvider())
	repoID := strings.TrimSpace(req.Msg.GetRepoId())
	stateID := strings.TrimSpace(req.Msg.GetStateId())

	record, err := s.Store.GetNamespace(ctx, provider, repoID, "")
	if err != nil {
		logError(s.Logger, "namespace lookup failed", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("namespace lookup failed"))
	}
	if record == nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("namespace not found"))
	}
	if stateID != "" && record.AccountID != stateID {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("state_id mismatch"))
	}
	return connect.NewResponse(&cloudv1.GetNamespaceWebhookResponse{
		Enabled: record.WebhooksEnabled,
	}), nil
}

func (s *NamespacesService) SetNamespaceWebhook(
	ctx context.Context,
	req *connect.Request[cloudv1.SetNamespaceWebhookRequest],
) (*connect.Response[cloudv1.SetNamespaceWebhookResponse], error) {
	if s.Store == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("storage not configured"))
	}
	if s.InstallStore == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("installation storage not configured"))
	}
	provider := strings.TrimSpace(req.Msg.GetProvider())
	repoID := strings.TrimSpace(req.Msg.GetRepoId())
	stateID := strings.TrimSpace(req.Msg.GetStateId())

	record, err := s.Store.GetNamespace(ctx, provider, repoID, "")
	if err != nil {
		logError(s.Logger, "namespace lookup failed", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("namespace lookup failed"))
	}
	if record == nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("namespace not found"))
	}
	if stateID != "" && record.AccountID != stateID {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("state_id mismatch"))
	}
	if provider == "github" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("github webhooks are always enabled"))
	}
	webhookURL, err := webhookURL(s.Endpoint, provider)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	install, err := installationForNamespace(ctx, s.InstallStore, provider, record, stateID)
	if err != nil {
		logError(s.Logger, "installation lookup failed", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("installation lookup failed"))
	}
	if install == nil || install.AccessToken == "" {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("access token missing"))
	}
	providerCfg, cfgErr := s.providerConfigFor(ctx, provider, install.ProviderInstanceKey)
	if cfgErr != nil {
		logError(s.Logger, "provider config lookup failed", cfgErr)
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("provider config missing"))
	}

	if req.Msg.GetEnabled() {
		if err := enableProviderWebhook(ctx, provider, providerCfg, install.AccessToken, *record, webhookURL); err != nil {
			logError(s.Logger, "webhook enable failed", err)
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("webhook enable failed"))
		}
		record.WebhooksEnabled = true
	} else {
		if err := disableProviderWebhook(ctx, provider, providerCfg, install.AccessToken, *record, webhookURL); err != nil {
			logError(s.Logger, "webhook disable failed", err)
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("webhook disable failed"))
		}
		record.WebhooksEnabled = false
	}
	if err := s.Store.UpsertNamespace(ctx, *record); err != nil {
		logError(s.Logger, "namespace update failed", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("namespace update failed"))
	}

	return connect.NewResponse(&cloudv1.SetNamespaceWebhookResponse{
		Enabled: record.WebhooksEnabled,
	}), nil
}

func (s *NamespacesService) providerConfigFor(ctx context.Context, provider, instanceKey string) (auth.ProviderConfig, error) {
	instanceKey = strings.TrimSpace(instanceKey)
	if instanceKey != "" {
		if s.ProviderInstanceCache != nil {
			if cfg, ok, err := s.ProviderInstanceCache.ConfigFor(ctx, provider, instanceKey); err == nil && ok {
				return cfg, nil
			}
		}
		if s.ProviderInstanceStore != nil {
			record, err := s.ProviderInstanceStore.GetProviderInstance(ctx, provider, instanceKey)
			if err != nil {
				return auth.ProviderConfig{}, err
			}
			if record != nil {
				return providerinstance.ProviderConfigFromRecord(*record)
			}
		}
	}
	return providerConfigFromAuthConfig(s.Providers, provider), nil
}

func logError(logger *log.Logger, message string, err error) {
	if logger == nil {
		return
	}
	logger.Printf("%s: %v", message, err)
}

const (
	defaultEventLogPageSize = 50
	maxEventLogPageSize     = 200
)

func decodePageToken(token string) (int, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return 0, nil
	}
	raw, err := base64.StdEncoding.DecodeString(token)
	if err != nil {
		return 0, errors.New("invalid page token")
	}
	offset, err := strconv.Atoi(string(raw))
	if err != nil || offset < 0 {
		return 0, errors.New("invalid page token")
	}
	return offset, nil
}

func encodePageToken(offset int) string {
	if offset <= 0 {
		return ""
	}
	return base64.StdEncoding.EncodeToString([]byte(strconv.Itoa(offset)))
}

func encodePageTokenFromRaw(token string) string {
	token = strings.TrimSpace(token)
	if token == "" {
		return ""
	}
	offset, err := strconv.Atoi(token)
	if err != nil || offset <= 0 {
		return ""
	}
	return encodePageToken(offset)
}

func eventLogIntervalFromProto(interval cloudv1.EventLogTimeseriesInterval) (storage.EventLogInterval, error) {
	switch interval {
	case cloudv1.EventLogTimeseriesInterval_EVENT_LOG_TIMESERIES_INTERVAL_HOUR:
		return storage.EventLogIntervalHour, nil
	case cloudv1.EventLogTimeseriesInterval_EVENT_LOG_TIMESERIES_INTERVAL_DAY:
		return storage.EventLogIntervalDay, nil
	case cloudv1.EventLogTimeseriesInterval_EVENT_LOG_TIMESERIES_INTERVAL_WEEK:
		return storage.EventLogIntervalWeek, nil
	default:
		return "", errors.New("invalid interval")
	}
}

func eventLogBreakdownGroupFromProto(group cloudv1.EventLogBreakdownGroup) (storage.EventLogBreakdownGroup, error) {
	switch group {
	case cloudv1.EventLogBreakdownGroup_EVENT_LOG_BREAKDOWN_GROUP_PROVIDER:
		return storage.EventLogBreakdownProvider, nil
	case cloudv1.EventLogBreakdownGroup_EVENT_LOG_BREAKDOWN_GROUP_EVENT:
		return storage.EventLogBreakdownEvent, nil
	case cloudv1.EventLogBreakdownGroup_EVENT_LOG_BREAKDOWN_GROUP_RULE_ID:
		return storage.EventLogBreakdownRuleID, nil
	case cloudv1.EventLogBreakdownGroup_EVENT_LOG_BREAKDOWN_GROUP_RULE_WHEN:
		return storage.EventLogBreakdownRuleWhen, nil
	case cloudv1.EventLogBreakdownGroup_EVENT_LOG_BREAKDOWN_GROUP_TOPIC:
		return storage.EventLogBreakdownTopic, nil
	case cloudv1.EventLogBreakdownGroup_EVENT_LOG_BREAKDOWN_GROUP_NAMESPACE_ID:
		return storage.EventLogBreakdownNamespaceID, nil
	case cloudv1.EventLogBreakdownGroup_EVENT_LOG_BREAKDOWN_GROUP_NAMESPACE_NAME:
		return storage.EventLogBreakdownNamespaceName, nil
	case cloudv1.EventLogBreakdownGroup_EVENT_LOG_BREAKDOWN_GROUP_INSTALLATION_ID:
		return storage.EventLogBreakdownInstallation, nil
	default:
		return "", errors.New("invalid group_by")
	}
}

func eventLogBreakdownSortFromProto(sortBy cloudv1.EventLogBreakdownSort) storage.EventLogBreakdownSort {
	switch sortBy {
	case cloudv1.EventLogBreakdownSort_EVENT_LOG_BREAKDOWN_SORT_MATCHED:
		return storage.EventLogBreakdownSortMatched
	case cloudv1.EventLogBreakdownSort_EVENT_LOG_BREAKDOWN_SORT_FAILED:
		return storage.EventLogBreakdownSortFailed
	default:
		return storage.EventLogBreakdownSortCount
	}
}

func toProtoInstallations(records []storage.InstallRecord) []*cloudv1.InstallRecord {
	out := make([]*cloudv1.InstallRecord, 0, len(records))
	for _, record := range records {
		out = append(out, toProtoInstallation(record))
	}
	return out
}

func toProtoInstallation(record storage.InstallRecord) *cloudv1.InstallRecord {
	return &cloudv1.InstallRecord{
		Provider:            record.Provider,
		AccountId:           record.AccountID,
		AccountName:         record.AccountName,
		InstallationId:      record.InstallationID,
		AccessToken:         record.AccessToken,
		RefreshToken:        record.RefreshToken,
		ExpiresAt:           toProtoTimestampPtr(record.ExpiresAt),
		MetadataJson:        record.MetadataJSON,
		CreatedAt:           toProtoTimestamp(record.CreatedAt),
		UpdatedAt:           toProtoTimestamp(record.UpdatedAt),
		ProviderInstanceKey: record.ProviderInstanceKey,
	}
}

func toProtoNamespaces(records []storage.NamespaceRecord) []*cloudv1.NamespaceRecord {
	out := make([]*cloudv1.NamespaceRecord, 0, len(records))
	for _, record := range records {
		out = append(out, toProtoNamespace(record))
	}
	return out
}

func toProtoNamespace(record storage.NamespaceRecord) *cloudv1.NamespaceRecord {
	return &cloudv1.NamespaceRecord{
		Provider:        record.Provider,
		RepoId:          record.RepoID,
		AccountId:       record.AccountID,
		InstallationId:  record.InstallationID,
		Owner:           record.Owner,
		RepoName:        record.RepoName,
		FullName:        record.FullName,
		Visibility:      record.Visibility,
		DefaultBranch:   record.DefaultBranch,
		HttpUrl:         record.HTTPURL,
		SshUrl:          record.SSHURL,
		WebhooksEnabled: record.WebhooksEnabled,
		CreatedAt:       toProtoTimestamp(record.CreatedAt),
		UpdatedAt:       toProtoTimestamp(record.UpdatedAt),
	}
}

func toProtoTimestamp(value time.Time) *timestamppb.Timestamp {
	if value.IsZero() {
		return nil
	}
	return timestamppb.New(value)
}

func toProtoTimestampPtr(value *time.Time) *timestamppb.Timestamp {
	if value == nil || value.IsZero() {
		return nil
	}
	return timestamppb.New(*value)
}

func fromProtoTimestamp(value *timestamppb.Timestamp) time.Time {
	if value == nil {
		return time.Time{}
	}
	parsed := value.AsTime()
	if parsed.IsZero() {
		return time.Time{}
	}
	return parsed
}

func fromProtoTimestampPtr(value *timestamppb.Timestamp) *time.Time {
	if value == nil {
		return nil
	}
	parsed := value.AsTime()
	if parsed.IsZero() {
		return nil
	}
	return &parsed
}

func toProtoRuleMatches(matches []core.MatchedRule) []*cloudv1.RuleMatch {
	out := make([]*cloudv1.RuleMatch, 0, len(matches))
	for _, match := range matches {
		out = append(out, &cloudv1.RuleMatch{
			When:    match.When,
			Emit:    append([]string(nil), match.Emit...),
			Drivers: append([]string(nil), match.Drivers...),
		})
	}
	return out
}

func toProtoDriverRecord(record *storage.DriverRecord) *cloudv1.DriverRecord {
	if record == nil {
		return nil
	}
	return &cloudv1.DriverRecord{
		Id:         record.ID,
		Name:       record.Name,
		ConfigJson: record.ConfigJSON,
		Enabled:    record.Enabled,
		CreatedAt:  toProtoTimestamp(record.CreatedAt),
		UpdatedAt:  toProtoTimestamp(record.UpdatedAt),
	}
}

func toProtoDriverRecords(records []storage.DriverRecord) []*cloudv1.DriverRecord {
	out := make([]*cloudv1.DriverRecord, 0, len(records))
	for _, record := range records {
		item := record
		out = append(out, toProtoDriverRecord(&item))
	}
	return out
}

func enabledProvidersList(cfg auth.Config) []string {
	out := make([]string, 0, 3)
	if cfg.GitHub.Enabled {
		out = append(out, "github")
	}
	if cfg.GitLab.Enabled {
		out = append(out, "gitlab")
	}
	if cfg.Bitbucket.Enabled {
		out = append(out, "bitbucket")
	}
	return out
}

func providerConfigFromAuthConfig(cfg auth.Config, provider string) auth.ProviderConfig {
	switch strings.TrimSpace(provider) {
	case "gitlab":
		return cfg.GitLab
	case "bitbucket":
		return cfg.Bitbucket
	default:
		return cfg.GitHub
	}
}

func providerEnabled(provider string, enabled []string) bool {
	for _, item := range enabled {
		if item == provider {
			return true
		}
	}
	return false
}

func providerNotEnabledMessage(provider string, enabled []string) string {
	if len(enabled) == 0 {
		return "provider not enabled (no providers enabled)"
	}
	return "provider not enabled; enabled=" + strings.Join(enabled, ",")
}

func toProtoRuleRecords(records []storage.RuleRecord) []*cloudv1.RuleRecord {
	out := make([]*cloudv1.RuleRecord, 0, len(records))
	for _, record := range records {
		out = append(out, toProtoRuleRecord(record))
	}
	return out
}

func toProtoRuleRecord(record storage.RuleRecord) *cloudv1.RuleRecord {
	return &cloudv1.RuleRecord{
		Id:        record.ID,
		When:      record.When,
		Emit:      append([]string(nil), record.Emit...),
		Drivers:   append([]string(nil), record.Drivers...),
		CreatedAt: toProtoTimestamp(record.CreatedAt),
		UpdatedAt: toProtoTimestamp(record.UpdatedAt),
	}
}

func toProtoEventLogRecords(records []storage.EventLogRecord) []*cloudv1.EventLogRecord {
	out := make([]*cloudv1.EventLogRecord, 0, len(records))
	for _, record := range records {
		out = append(out, toProtoEventLogRecord(record))
	}
	return out
}

func toProtoEventLogRecord(record storage.EventLogRecord) *cloudv1.EventLogRecord {
	return &cloudv1.EventLogRecord{
		Id:             record.ID,
		Provider:       record.Provider,
		Name:           record.Name,
		RequestId:      record.RequestID,
		StateId:        record.StateID,
		InstallationId: record.InstallationID,
		NamespaceId:    record.NamespaceID,
		NamespaceName:  record.NamespaceName,
		Topic:          record.Topic,
		RuleId:         record.RuleID,
		RuleWhen:       record.RuleWhen,
		Drivers:        append([]string(nil), record.Drivers...),
		Matched:        record.Matched,
		Status:         record.Status,
		ErrorMessage:   record.ErrorMessage,
		CreatedAt:      toProtoTimestamp(record.CreatedAt),
		UpdatedAt:      toProtoTimestamp(record.UpdatedAt),
	}
}

func toProtoEventLogAnalytics(analytics storage.EventLogAnalytics) *cloudv1.EventLogAnalytics {
	return &cloudv1.EventLogAnalytics{
		Total:            analytics.Total,
		Matched:          analytics.Matched,
		DistinctRequests: analytics.DistinctReq,
		ByProvider:       toProtoEventLogCounts(analytics.ByProvider),
		ByEvent:          toProtoEventLogCounts(analytics.ByEvent),
		ByTopic:          toProtoEventLogCounts(analytics.ByTopic),
		ByRule:           toProtoEventLogCounts(analytics.ByRule),
		ByInstallation:   toProtoEventLogCounts(analytics.ByInstall),
		ByNamespace:      toProtoEventLogCounts(analytics.ByNamespace),
	}
}

func toProtoEventLogCounts(counts []storage.EventLogCount) []*cloudv1.EventLogCount {
	out := make([]*cloudv1.EventLogCount, 0, len(counts))
	for _, count := range counts {
		out = append(out, &cloudv1.EventLogCount{
			Key:   count.Key,
			Count: count.Count,
		})
	}
	return out
}

func toProtoEventLogTimeseries(buckets []storage.EventLogTimeseriesBucket) []*cloudv1.EventLogTimeseriesBucket {
	out := make([]*cloudv1.EventLogTimeseriesBucket, 0, len(buckets))
	for _, bucket := range buckets {
		out = append(out, &cloudv1.EventLogTimeseriesBucket{
			StartTime:        toProtoTimestamp(bucket.Start),
			EndTime:          toProtoTimestamp(bucket.End),
			EventCount:       bucket.EventCount,
			MatchedCount:     bucket.MatchedCount,
			DistinctRequests: bucket.DistinctReq,
			FailedCount:      bucket.FailureCount,
		})
	}
	return out
}

func toProtoEventLogBreakdowns(breakdowns []storage.EventLogBreakdown) []*cloudv1.EventLogBreakdown {
	out := make([]*cloudv1.EventLogBreakdown, 0, len(breakdowns))
	for _, item := range breakdowns {
		out = append(out, &cloudv1.EventLogBreakdown{
			Key:          item.Key,
			EventCount:   item.EventCount,
			MatchedCount: item.MatchedCount,
			FailedCount:  item.FailureCount,
			LatencyP50Ms: item.LatencyP50MS,
			LatencyP95Ms: item.LatencyP95MS,
			LatencyP99Ms: item.LatencyP99MS,
		})
	}
	return out
}

func parseRuleInput(rule *cloudv1.Rule) (string, []string, []string, error) {
	if rule == nil {
		return "", nil, nil, errors.New("missing rule")
	}
	when := strings.TrimSpace(rule.GetWhen())
	emit := rule.GetEmit()
	driverIDs := make([]string, 0, len(rule.GetDrivers()))
	for _, value := range rule.GetDrivers() {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			driverIDs = append(driverIDs, trimmed)
		}
	}
	return when, emit, driverIDs, nil
}

func normalizeCoreRule(when string, emit []string, drivers []string) (core.Rule, error) {
	coreRule := core.Rule{
		When:    strings.TrimSpace(when),
		Emit:    core.EmitList(emit),
		Drivers: drivers,
	}
	normalized, err := core.NormalizeRules([]core.Rule{coreRule})
	if err != nil {
		return core.Rule{}, err
	}
	if len(normalized) == 0 {
		return core.Rule{}, errors.New("rule is empty")
	}
	return normalized[0], nil
}

func (s *RulesService) resolveDriverNames(ctx context.Context, driverIDs []string) ([]string, error) {
	if len(driverIDs) == 0 {
		return nil, errors.New("drivers are required")
	}
	if s.DriverStore == nil {
		return nil, errors.New("driver store not configured")
	}
	seen := make(map[string]struct{}, len(driverIDs))
	drivers := make([]string, 0, len(driverIDs))
	for _, id := range driverIDs {
		trimmed := strings.TrimSpace(id)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		record, err := s.DriverStore.GetDriverByID(ctx, trimmed)
		if err != nil {
			return nil, err
		}
		if record == nil {
			return nil, fmt.Errorf("driver not found: %s", trimmed)
		}
		name := strings.TrimSpace(record.Name)
		if name == "" {
			return nil, fmt.Errorf("driver %s has empty name", trimmed)
		}
		drivers = append(drivers, name)
	}
	if len(drivers) == 0 {
		return nil, errors.New("drivers are required")
	}
	return drivers, nil
}

func (s *RulesService) refreshEngine(ctx context.Context) error {
	if s.Store == nil || s.Engine == nil {
		return nil
	}
	records, err := s.Store.ListRules(ctx)
	if err != nil {
		return err
	}
	tenantID := storage.TenantFromContext(ctx)
	if tenantID != "" {
		loaded := make([]core.Rule, 0, len(records))
		for _, record := range records {
			drivers, err := s.resolveDriverNames(ctx, record.Drivers)
			if err != nil {
				logError(s.Logger, "rule driver resolve failed", err)
				continue
			}
			loaded = append(loaded, core.Rule{
				ID:      record.ID,
				When:    record.When,
				Emit:    core.EmitList(record.Emit),
				Drivers: drivers,
			})
		}
		normalized, err := core.NormalizeRules(loaded)
		if err != nil {
			return err
		}
		return s.Engine.Update(core.RulesConfig{
			Rules:    normalized,
			Strict:   s.Strict,
			TenantID: tenantID,
			Logger:   s.Logger,
		})
	}

	grouped := make(map[string][]core.Rule)
	for _, record := range records {
		tenantCtx := storage.WithTenant(ctx, record.TenantID)
		drivers, err := s.resolveDriverNames(tenantCtx, record.Drivers)
		if err != nil {
			logError(s.Logger, "rule driver resolve failed", err)
			continue
		}
		grouped[record.TenantID] = append(grouped[record.TenantID], core.Rule{
			ID:      record.ID,
			When:    record.When,
			Emit:    core.EmitList(record.Emit),
			Drivers: drivers,
		})
	}
	for id, rules := range grouped {
		normalized, err := core.NormalizeRules(rules)
		if err != nil {
			return err
		}
		if err := s.Engine.Update(core.RulesConfig{
			Rules:    normalized,
			Strict:   s.Strict,
			TenantID: id,
			Logger:   s.Logger,
		}); err != nil {
			return err
		}
	}
	return nil
}

func installationsForSync(ctx context.Context, store storage.Store, provider, stateID string) ([]storage.InstallRecord, error) {
	if store == nil {
		return nil, errors.New("store is not initialized")
	}
	if stateID != "" {
		record, err := latestInstallation(ctx, store, provider, stateID)
		if err != nil || record == nil {
			return nil, err
		}
		return []storage.InstallRecord{*record}, nil
	}
	return store.ListInstallations(ctx, provider, "")
}

func installationForNamespace(ctx context.Context, store storage.Store, provider string, record *storage.NamespaceRecord, stateID string) (*storage.InstallRecord, error) {
	if store == nil || record == nil {
		return nil, nil
	}
	if record.InstallationID != "" {
		found, err := store.GetInstallationByInstallationID(ctx, provider, record.InstallationID)
		if err != nil {
			return nil, err
		}
		if found != nil {
			return found, nil
		}
	}
	if record.AccountID != "" {
		return latestInstallation(ctx, store, provider, record.AccountID)
	}
	if stateID != "" {
		return latestInstallation(ctx, store, provider, stateID)
	}
	return nil, nil
}

func latestInstallation(ctx context.Context, store storage.Store, provider, accountID string) (*storage.InstallRecord, error) {
	records, err := store.ListInstallations(ctx, provider, accountID)
	if err != nil {
		return nil, err
	}
	var latest *storage.InstallRecord
	for i := range records {
		item := records[i]
		if latest == nil || item.UpdatedAt.After(latest.UpdatedAt) {
			copy := item
			latest = &copy
		}
	}
	return latest, nil
}

func shouldRefresh(expiresAt *time.Time) bool {
	if expiresAt == nil {
		return false
	}
	return time.Now().UTC().After(expiresAt.Add(-1 * time.Minute))
}

func webhookURL(endpoint, provider string) (string, error) {
	endpoint = strings.TrimSpace(endpoint)
	endpoint = strings.TrimRight(endpoint, "/")
	if endpoint == "" {
		return "", errors.New("endpoint is required for webhook management")
	}
	switch provider {
	case "gitlab":
		return endpoint + "/webhooks/gitlab", nil
	case "bitbucket":
		return endpoint + "/webhooks/bitbucket", nil
	default:
		return "", errors.New("unsupported provider for webhook management")
	}
}

const (
	providerInstanceHashBytes    = 32
	providerInstanceHashAttempts = 5
)

func generateProviderInstanceHash(ctx context.Context, store storage.ProviderInstanceStore, provider string) (string, error) {
	for i := 0; i < providerInstanceHashAttempts; i++ {
		hash, err := randomHex(providerInstanceHashBytes)
		if err != nil {
			return "", err
		}
		existing, err := store.GetProviderInstance(ctx, provider, hash)
		if err != nil {
			return "", err
		}
		if existing == nil {
			return hash, nil
		}
	}
	return "", errors.New("unable to generate unique provider instance hash")
}

func randomHex(size int) (string, error) {
	if size <= 0 {
		return "", errors.New("random hex size must be positive")
	}
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func toProtoProviderRecord(record *storage.ProviderInstanceRecord) *cloudv1.ProviderRecord {
	if record == nil {
		return nil
	}
	return &cloudv1.ProviderRecord{
		Provider:   record.Provider,
		Hash:       record.Key,
		ConfigJson: record.ConfigJSON,
		Enabled:    record.Enabled,
		CreatedAt:  timestamppb.New(record.CreatedAt),
		UpdatedAt:  timestamppb.New(record.UpdatedAt),
	}
}

func toProtoProviderRecords(records []storage.ProviderInstanceRecord) []*cloudv1.ProviderRecord {
	out := make([]*cloudv1.ProviderRecord, 0, len(records))
	for _, record := range records {
		item := record
		out = append(out, toProtoProviderRecord(&item))
	}
	return out
}
