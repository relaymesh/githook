package api

import (
	"context"
	"encoding/base64"
	"errors"
	"log"
	"strconv"
	"strings"

	"connectrpc.com/connect"

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
	DriverStore storage.DriverStore
	Publisher   core.Publisher
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

	payload := record.Body
	if len(record.TransformedBody) > 0 {
		payload = record.TransformedBody
	}
	if len(payload) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("event log payload is empty"))
	}

	event := core.Event{
		Provider:       record.Provider,
		Name:           record.Name,
		RequestID:      record.RequestID,
		StateID:        record.StateID,
		InstallationID: record.InstallationID,
		NamespaceID:    record.NamespaceID,
		NamespaceName:  record.NamespaceName,
		RawPayload:     append([]byte(nil), payload...),
		LogID:          record.ID,
	}
	if err := publisher.PublishForDrivers(ctx, topic, event, []string{driverRecord.Name}); err != nil {
		logError(s.Logger, "replay publish failed", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("replay publish failed"))
	}

	return connect.NewResponse(&cloudv1.ReplayEventLogResponse{
		LogId:      record.ID,
		Topic:      topic,
		DriverName: driverRecord.Name,
	}), nil
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
