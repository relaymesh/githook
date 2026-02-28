package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log"
	"strconv"
	"strings"

	"connectrpc.com/connect"
	"github.com/dop251/goja"

	"github.com/relaymesh/githook/pkg/core"
	driverspkg "github.com/relaymesh/githook/pkg/drivers"
	cloudv1 "github.com/relaymesh/githook/pkg/gen/cloud/v1"
	"github.com/relaymesh/githook/pkg/storage"
)

const (
	defaultEventLogPageSize = 50
	maxEventLogPageSize     = 200
)

// EventLogsService handles queries for webhook event logs and analytics.
type EventLogsService struct {
	Store       storage.EventLogStore
	RuleStore   storage.RuleStore
	DriverStore storage.DriverStore
	Publisher   core.Publisher
	RulesStrict bool
	Logger      *log.Logger
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
	logger := s.Logger
	if logger == nil {
		logger = log.Default()
	}
	headerTenant := ""
	if req != nil {
		header := req.Header()
		if header != nil {
			headerTenant = strings.TrimSpace(header.Get("X-Tenant-ID"))
			if headerTenant == "" {
				headerTenant = strings.TrimSpace(header.Get("X-Githooks-Tenant-ID"))
			}
		}
	}
	resolvedTenant := storage.TenantFromContext(ctx)
	if len(records) > 0 {
		logger.Printf("event log list tenant=%s header_tenant=%s request_id=%s count=%d first_log_id=%s first_status=%s", resolvedTenant, headerTenant, filter.RequestID, len(records), records[0].ID, records[0].Status)
	} else {
		logger.Printf("event log list tenant=%s header_tenant=%s request_id=%s count=0", resolvedTenant, headerTenant, filter.RequestID)
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
	if req == nil || req.Msg == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("request is required"))
	}
	msg := req.Msg
	logID := strings.TrimSpace(msg.GetLogId())
	status := strings.TrimSpace(msg.GetStatus())
	errMsg := strings.TrimSpace(msg.GetErrorMessage())
	tenantID := storage.TenantFromContext(ctx)
	logger := s.Logger
	if logger == nil {
		logger = log.Default()
	}
	var headerTenant string
	header := req.Header()
	if header != nil {
		headerTenant = strings.TrimSpace(header.Get("X-Tenant-ID"))
		if headerTenant == "" {
			headerTenant = strings.TrimSpace(header.Get("X-Githooks-Tenant-ID"))
		}
	}
	logger.Printf("event log update request log_id=%s status=%s tenant=%s header_tenant=%s err_len=%d", logID, status, tenantID, headerTenant, len(errMsg))
	if err := s.Store.UpdateEventLogStatus(ctx, logID, status, errMsg); err != nil {
		logError(s.Logger, "event log update failed", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("event log update failed"))
	}
	logger.Printf("event log update ok log_id=%s status=%s tenant=%s", logID, status, tenantID)
	return connect.NewResponse(&cloudv1.UpdateEventLogStatusResponse{}), nil
}

func (s *EventLogsService) ReplayEventLog(
	ctx context.Context,
	req *connect.Request[cloudv1.ReplayEventLogRequest],
) (*connect.Response[cloudv1.ReplayEventLogResponse], error) {
	if s.Store == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("storage not configured"))
	}
	if s.DriverStore == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("driver storage not configured"))
	}
	if s.RuleStore == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("rule storage not configured"))
	}
	if req == nil || req.Msg == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("request is required"))
	}
	logID := strings.TrimSpace(req.Msg.GetLogId())
	driverName := strings.TrimSpace(req.Msg.GetDriverName())
	if logID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("log_id is required"))
	}
	if driverName == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("driver_name is required"))
	}

	record, err := s.Store.GetEventLog(ctx, logID)
	if err != nil {
		logError(s.Logger, "get event log failed", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("get event log failed"))
	}
	if record == nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("event log not found"))
	}
	topic := strings.TrimSpace(record.Topic)
	if topic == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("event log topic is empty"))
	}

	driverRecord, err := s.DriverStore.GetDriver(ctx, driverName)
	if err != nil {
		logError(s.Logger, "get driver failed", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("get driver failed"))
	}
	if driverRecord == nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("driver not found"))
	}
	if !driverRecord.Enabled {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("driver is disabled"))
	}

	publisher := s.Publisher
	if publisher == nil {
		cfg, err := driverspkg.ConfigFromDriver(driverRecord.Name, driverRecord.ConfigJSON)
		if err != nil {
			logError(s.Logger, "driver config parse failed", err)
			return nil, connect.NewError(connect.CodeInternal, errors.New("driver config parse failed"))
		}
		publisher, err = core.NewPublisher(cfg)
		if err != nil {
			logError(s.Logger, "publisher init failed", err)
			return nil, connect.NewError(connect.CodeInternal, errors.New("publisher init failed"))
		}
		defer func() { _ = publisher.Close() }()
	}

	event, err := replayEventFromLog(*record)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	rules, err := s.matchReplayRules(ctx, event)
	if err != nil {
		logError(s.Logger, "replay match failed", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("replay match failed"))
	}
	if len(rules) == 0 {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("no matching rules for event log"))
	}

	published := 0
	for _, rule := range rules {
		transformed, err := replayApplyTransform(event, rule.TransformJS)
		if err != nil {
			logError(s.Logger, "replay transform failed", err)
			return nil, connect.NewError(connect.CodeInternal, errors.New("replay transform failed"))
		}
		if err := publisher.PublishForDrivers(ctx, rule.Topic, transformed, []string{driverRecord.Name}); err != nil {
			logError(s.Logger, "replay publish failed", err)
			return nil, connect.NewError(connect.CodeInternal, errors.New("replay publish failed"))
		}
		published++
	}
	if published == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("event log payload is empty"))
	}

	return connect.NewResponse(&cloudv1.ReplayEventLogResponse{
		LogId:      record.ID,
		Topic:      topic,
		DriverName: driverRecord.Name,
	}), nil
}

func replayEventFromLog(record storage.EventLogRecord) (core.Event, error) {
	body := append([]byte(nil), record.Body...)
	if len(body) == 0 {
		if len(record.TransformedBody) > 0 {
			body = append([]byte(nil), record.TransformedBody...)
		} else {
			return core.Event{}, errors.New("event log payload is empty")
		}
	}
	var rawObj map[string]interface{}
	if err := json.Unmarshal(body, &rawObj); err != nil {
		return core.Event{}, errors.New("event log payload is invalid json")
	}
	return core.Event{
		Provider:       record.Provider,
		Name:           record.Name,
		RequestID:      record.RequestID,
		StateID:        record.StateID,
		TenantID:       record.TenantID,
		InstallationID: record.InstallationID,
		NamespaceID:    record.NamespaceID,
		NamespaceName:  record.NamespaceName,
		RawPayload:     body,
		RawObject:      rawObj,
		Data:           core.Flatten(rawObj),
		LogID:          record.ID,
	}, nil
}

func (s *EventLogsService) matchReplayRules(ctx context.Context, event core.Event) ([]core.RuleMatch, error) {
	records, err := s.RuleStore.ListRules(ctx)
	if err != nil {
		return nil, err
	}
	rules := make([]core.Rule, 0, len(records))
	for _, record := range records {
		if strings.TrimSpace(record.When) == "" || len(record.Emit) == 0 {
			continue
		}
		driverID := strings.TrimSpace(record.DriverID)
		if driverID == "" {
			continue
		}
		driverName := strings.TrimSpace(record.DriverName)
		if driverName == "" {
			driver, derr := s.DriverStore.GetDriverByID(ctx, driverID)
			if derr != nil || driver == nil || strings.TrimSpace(driver.Name) == "" {
				continue
			}
			driverName = strings.TrimSpace(driver.Name)
		}
		rules = append(rules, core.Rule{
			ID:          record.ID,
			When:        record.When,
			Emit:        core.EmitList(record.Emit),
			DriverID:    driverID,
			TransformJS: strings.TrimSpace(record.TransformJS),
			DriverName:  driverName,
		})
	}
	if len(rules) == 0 {
		return nil, nil
	}
	engine, err := core.NewRuleEngine(core.RulesConfig{Rules: rules, Strict: s.RulesStrict, Logger: s.Logger})
	if err != nil {
		return nil, err
	}
	matched := engine.EvaluateRulesWithLogger(event, s.Logger)
	out := make([]core.RuleMatch, 0, len(matched))
	for _, mr := range matched {
		for _, topic := range mr.Emit {
			out = append(out, core.RuleMatch{Topic: topic, DriverID: mr.DriverID, DriverName: mr.DriverName, RuleID: mr.ID, RuleWhen: mr.When, TransformJS: mr.TransformJS})
		}
	}
	return out, nil
}

func replayApplyTransform(event core.Event, transformJS string) (core.Event, error) {
	transformJS = strings.TrimSpace(transformJS)
	if transformJS == "" {
		return event, nil
	}
	vm := goja.New()
	if _, err := vm.RunString("var transform = " + transformJS); err != nil {
		return event, err
	}
	transform, ok := goja.AssertFunction(vm.Get("transform"))
	if !ok {
		return event, errors.New("transform_js must define a function")
	}
	var payload any
	if err := json.Unmarshal(event.RawPayload, &payload); err != nil {
		return event, err
	}
	ctx := map[string]any{
		"provider":        event.Provider,
		"name":            event.Name,
		"request_id":      event.RequestID,
		"state_id":        event.StateID,
		"tenant_id":       event.TenantID,
		"installation_id": event.InstallationID,
		"namespace_id":    event.NamespaceID,
		"namespace_name":  event.NamespaceName,
		"data":            event.Data,
		"payload":         payload,
	}
	if err := vm.Set("event", ctx); err != nil {
		return event, err
	}
	result, err := transform(goja.Undefined(), vm.ToValue(payload), vm.ToValue(ctx))
	if err != nil {
		return event, err
	}
	exported := result.Export()
	if m, ok := exported.(map[string]any); ok {
		if p, exists := m["payload"]; exists {
			exported = p
		}
	}
	out, err := json.Marshal(exported)
	if err != nil {
		return event, err
	}
	event.RawPayload = out
	event.RawObject = nil
	return event, nil
}

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
