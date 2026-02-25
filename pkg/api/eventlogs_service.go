package api

import (
	"context"
	"encoding/base64"
	"errors"
	"log"
	"strconv"
	"strings"

	"connectrpc.com/connect"

	cloudv1 "github.com/relaymesh/githook/pkg/gen/cloud/v1"
	"github.com/relaymesh/githook/pkg/storage"
)

const (
	defaultEventLogPageSize = 50
	maxEventLogPageSize     = 200
)

// EventLogsService handles queries for webhook event logs and analytics.
type EventLogsService struct {
	Store  storage.EventLogStore
	Logger *log.Logger
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
