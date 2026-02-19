package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// MockEventLogStore is an in-memory implementation of EventLogStore for tests.
type MockEventLogStore struct {
	mu     sync.RWMutex
	values []EventLogRecord
}

// NewMockEventLogStore returns a new in-memory EventLogStore.
func NewMockEventLogStore() *MockEventLogStore {
	return &MockEventLogStore{values: make([]EventLogRecord, 0)}
}

func (m *MockEventLogStore) CreateEventLogs(ctx context.Context, records []EventLogRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(records) == 0 {
		return nil
	}
	now := time.Now().UTC()
	for _, record := range records {
		if record.TenantID == "" {
			record.TenantID = TenantFromContext(ctx)
		}
		if record.ID == "" {
			record.ID = uuid.NewString()
		}
		if record.Status == "" {
			record.Status = "queued"
		}
		if record.BodyHash == "" {
			record.BodyHash = mockBodyHash(record.Body)
		}
		if record.CreatedAt.IsZero() {
			record.CreatedAt = now
		}
		if record.UpdatedAt.IsZero() {
			record.UpdatedAt = record.CreatedAt
		}
		if record.LatencyMS < 0 {
			record.LatencyMS = 0
		}
		m.values = append(m.values, record)
	}
	return nil
}

func (m *MockEventLogStore) ListEventLogs(ctx context.Context, filter EventLogFilter) ([]EventLogRecord, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	tenantID := filter.TenantID
	if tenantID == "" {
		tenantID = TenantFromContext(ctx)
	}
	filtered := make([]EventLogRecord, 0)
	for _, record := range m.values {
		if tenantID != "" && record.TenantID != tenantID {
			continue
		}
		if filter.Provider != "" && record.Provider != filter.Provider {
			continue
		}
		if filter.Name != "" && record.Name != filter.Name {
			continue
		}
		if filter.RequestID != "" && record.RequestID != filter.RequestID {
			continue
		}
		if filter.StateID != "" && record.StateID != filter.StateID {
			continue
		}
		if filter.InstallationID != "" && record.InstallationID != filter.InstallationID {
			continue
		}
		if filter.NamespaceID != "" && record.NamespaceID != filter.NamespaceID {
			continue
		}
		if filter.NamespaceName != "" && record.NamespaceName != filter.NamespaceName {
			continue
		}
		if filter.Topic != "" && record.Topic != filter.Topic {
			continue
		}
		if filter.RuleID != "" && record.RuleID != filter.RuleID {
			continue
		}
		if filter.RuleWhen != "" && record.RuleWhen != filter.RuleWhen {
			continue
		}
		if filter.Matched != nil && record.Matched != *filter.Matched {
			continue
		}
		if !filter.StartTime.IsZero() && record.CreatedAt.Before(filter.StartTime) {
			continue
		}
		if !filter.EndTime.IsZero() && record.CreatedAt.After(filter.EndTime) {
			continue
		}
		filtered = append(filtered, record)
	}
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].CreatedAt.After(filtered[j].CreatedAt)
	})
	start := filter.Offset
	if start < 0 || start > len(filtered) {
		start = len(filtered)
	}
	end := len(filtered)
	if filter.Limit > 0 && start+filter.Limit < end {
		end = start + filter.Limit
	}
	return append([]EventLogRecord(nil), filtered[start:end]...), nil
}

func (m *MockEventLogStore) GetEventLogAnalytics(ctx context.Context, filter EventLogFilter) (EventLogAnalytics, error) {
	records, err := m.ListEventLogs(ctx, EventLogFilter{
		TenantID:       filter.TenantID,
		Provider:       filter.Provider,
		Name:           filter.Name,
		RequestID:      filter.RequestID,
		StateID:        filter.StateID,
		InstallationID: filter.InstallationID,
		Topic:          filter.Topic,
		Matched:        filter.Matched,
		StartTime:      filter.StartTime,
		EndTime:        filter.EndTime,
	})
	if err != nil {
		return EventLogAnalytics{}, err
	}
	total := int64(len(records))
	matched := int64(0)
	reqs := make(map[string]struct{})
	byProvider := make(map[string]int64)
	byEvent := make(map[string]int64)
	byTopic := make(map[string]int64)
	byRule := make(map[string]int64)
	byInstall := make(map[string]int64)
	byNamespace := make(map[string]int64)
	for _, record := range records {
		if record.Matched {
			matched++
		}
		if record.RequestID != "" {
			reqs[record.RequestID] = struct{}{}
		}
		if record.Provider != "" {
			byProvider[record.Provider]++
		}
		if record.Name != "" {
			byEvent[record.Name]++
		}
		if record.Topic != "" {
			byTopic[record.Topic]++
		}
		if record.RuleID != "" {
			byRule[record.RuleID]++
		} else if record.RuleWhen != "" {
			byRule[record.RuleWhen]++
		}
		if record.InstallationID != "" {
			byInstall[record.InstallationID]++
		}
		if record.NamespaceName != "" {
			byNamespace[record.NamespaceName]++
		} else if record.NamespaceID != "" {
			byNamespace[record.NamespaceID]++
		}
	}
	return EventLogAnalytics{
		Total:       total,
		Matched:     matched,
		DistinctReq: int64(len(reqs)),
		ByProvider:  mapCounts(byProvider),
		ByEvent:     mapCounts(byEvent),
		ByTopic:     mapCounts(byTopic),
		ByRule:      mapCounts(byRule),
		ByInstall:   mapCounts(byInstall),
		ByNamespace: mapCounts(byNamespace),
	}, nil
}

func (m *MockEventLogStore) UpdateEventLogStatus(ctx context.Context, id, status, errorMessage string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	tenantID := TenantFromContext(ctx)
	for i, record := range m.values {
		if record.ID != id {
			continue
		}
		if tenantID != "" && record.TenantID != tenantID {
			continue
		}
		record.Status = status
		record.ErrorMessage = errorMessage
		record.UpdatedAt = time.Now().UTC()
		if status == "success" || status == "failed" {
			record.LatencyMS = record.UpdatedAt.Sub(record.CreatedAt).Milliseconds()
		}
		m.values[i] = record
	}
	return nil
}

func (m *MockEventLogStore) GetEventLogTimeseries(ctx context.Context, filter EventLogFilter, interval EventLogInterval) ([]EventLogTimeseriesBucket, error) {
	if interval == "" {
		return nil, errors.New("interval is required")
	}
	if filter.StartTime.IsZero() || filter.EndTime.IsZero() {
		return nil, errors.New("start_time and end_time are required")
	}
	if filter.EndTime.Before(filter.StartTime) {
		return nil, errors.New("end_time must be after start_time")
	}
	filtered, err := m.ListEventLogs(ctx, EventLogFilter{
		TenantID:       filter.TenantID,
		Provider:       filter.Provider,
		Name:           filter.Name,
		RequestID:      filter.RequestID,
		StateID:        filter.StateID,
		InstallationID: filter.InstallationID,
		NamespaceID:    filter.NamespaceID,
		NamespaceName:  filter.NamespaceName,
		Topic:          filter.Topic,
		RuleID:         filter.RuleID,
		RuleWhen:       filter.RuleWhen,
		Matched:        filter.Matched,
		StartTime:      filter.StartTime,
		EndTime:        filter.EndTime,
	})
	if err != nil {
		return nil, err
	}

	start := mockBucketStart(filter.StartTime.UTC(), interval)
	end := filter.EndTime.UTC()
	step := mockIntervalDuration(interval)
	if step <= 0 {
		return nil, errors.New("invalid interval")
	}

	type bucketData struct {
		EventLogTimeseriesBucket
		reqs map[string]struct{}
	}
	buckets := make(map[int64]*bucketData)
	for _, record := range filtered {
		ts := record.CreatedAt.UTC()
		if ts.Before(start) || ts.After(end) {
			continue
		}
		bucket := mockBucketStart(ts, interval)
		key := bucket.Unix()
		entry := buckets[key]
		if entry == nil {
			entry = &bucketData{
				EventLogTimeseriesBucket: EventLogTimeseriesBucket{
					Start: bucket,
					End:   bucket.Add(step),
				},
				reqs: make(map[string]struct{}),
			}
			buckets[key] = entry
		}
		entry.EventCount++
		if record.Matched {
			entry.MatchedCount++
		}
		if record.RequestID != "" {
			entry.reqs[record.RequestID] = struct{}{}
		}
		if strings.EqualFold(record.Status, "failed") {
			entry.FailureCount++
		}
	}

	out := make([]EventLogTimeseriesBucket, 0)
	for cursor := start; cursor.Before(end) || cursor.Equal(end); cursor = cursor.Add(step) {
		key := cursor.Unix()
		if entry, ok := buckets[key]; ok {
			entry.DistinctReq = int64(len(entry.reqs))
			out = append(out, entry.EventLogTimeseriesBucket)
		} else {
			out = append(out, EventLogTimeseriesBucket{
				Start: cursor,
				End:   cursor.Add(step),
			})
		}
	}
	return out, nil
}

func (m *MockEventLogStore) GetEventLogBreakdown(ctx context.Context, filter EventLogFilter, groupBy EventLogBreakdownGroup, sortBy EventLogBreakdownSort, sortDesc bool, pageSize int, pageToken string, includeLatency bool) ([]EventLogBreakdown, string, error) {
	filtered, err := m.ListEventLogs(ctx, EventLogFilter{
		TenantID:       filter.TenantID,
		Provider:       filter.Provider,
		Name:           filter.Name,
		RequestID:      filter.RequestID,
		StateID:        filter.StateID,
		InstallationID: filter.InstallationID,
		NamespaceID:    filter.NamespaceID,
		NamespaceName:  filter.NamespaceName,
		Topic:          filter.Topic,
		RuleID:         filter.RuleID,
		RuleWhen:       filter.RuleWhen,
		Matched:        filter.Matched,
		StartTime:      filter.StartTime,
		EndTime:        filter.EndTime,
	})
	if err != nil {
		return nil, "", err
	}
	grouped := make(map[string]*EventLogBreakdown)
	latencies := make(map[string][]int64)
	for _, record := range filtered {
		key := mockBreakdownKey(record, groupBy)
		if strings.TrimSpace(key) == "" {
			continue
		}
		entry := grouped[key]
		if entry == nil {
			entry = &EventLogBreakdown{Key: key}
			grouped[key] = entry
		}
		entry.EventCount++
		if record.Matched {
			entry.MatchedCount++
		}
		if strings.EqualFold(record.Status, "failed") {
			entry.FailureCount++
		}
		if includeLatency && record.LatencyMS > 0 {
			latencies[key] = append(latencies[key], record.LatencyMS)
		}
	}
	out := make([]EventLogBreakdown, 0, len(grouped))
	for _, entry := range grouped {
		if includeLatency {
			if values := latencies[entry.Key]; len(values) > 0 {
				sort.Slice(values, func(i, j int) bool { return values[i] < values[j] })
				entry.LatencyP50MS = mockPercentile(values, 0.50)
				entry.LatencyP95MS = mockPercentile(values, 0.95)
				entry.LatencyP99MS = mockPercentile(values, 0.99)
			}
		}
		out = append(out, *entry)
	}
	sort.Slice(out, func(i, j int) bool {
		ai := mockSortValue(out[i], sortBy)
		aj := mockSortValue(out[j], sortBy)
		if ai == aj {
			if sortDesc {
				return out[i].Key < out[j].Key
			}
			return out[i].Key > out[j].Key
		}
		if sortDesc {
			return ai > aj
		}
		return ai < aj
	})

	offset, err := mockParsePageToken(pageToken)
	if err != nil {
		return nil, "", err
	}
	if pageSize <= 0 {
		pageSize = 50
	}
	if offset > len(out) {
		offset = len(out)
	}
	end := offset + pageSize
	if end > len(out) {
		end = len(out)
	}
	nextToken := ""
	if end < len(out) {
		nextToken = mockFormatPageToken(end)
	}
	return append([]EventLogBreakdown(nil), out[offset:end]...), nextToken, nil
}

func (m *MockEventLogStore) Close() error {
	return nil
}

func mapCounts(input map[string]int64) []EventLogCount {
	out := make([]EventLogCount, 0, len(input))
	for key, count := range input {
		out = append(out, EventLogCount{Key: key, Count: count})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count == out[j].Count {
			return out[i].Key < out[j].Key
		}
		return out[i].Count > out[j].Count
	})
	return out
}

func mockIntervalDuration(interval EventLogInterval) time.Duration {
	switch interval {
	case EventLogIntervalHour:
		return time.Hour
	case EventLogIntervalDay:
		return 24 * time.Hour
	case EventLogIntervalWeek:
		return 7 * 24 * time.Hour
	default:
		return 0
	}
}

func mockBucketStart(ts time.Time, interval EventLogInterval) time.Time {
	ts = ts.UTC()
	switch interval {
	case EventLogIntervalHour:
		return ts.Truncate(time.Hour)
	case EventLogIntervalDay:
		return time.Date(ts.Year(), ts.Month(), ts.Day(), 0, 0, 0, 0, time.UTC)
	case EventLogIntervalWeek:
		day := time.Date(ts.Year(), ts.Month(), ts.Day(), 0, 0, 0, 0, time.UTC)
		weekday := int(day.Weekday())
		shift := (weekday + 6) % 7
		return day.AddDate(0, 0, -shift)
	default:
		return ts
	}
}

func mockBreakdownKey(record EventLogRecord, groupBy EventLogBreakdownGroup) string {
	switch groupBy {
	case EventLogBreakdownProvider:
		return record.Provider
	case EventLogBreakdownEvent:
		return record.Name
	case EventLogBreakdownRuleID:
		return record.RuleID
	case EventLogBreakdownRuleWhen:
		return record.RuleWhen
	case EventLogBreakdownTopic:
		return record.Topic
	case EventLogBreakdownNamespaceID:
		return record.NamespaceID
	case EventLogBreakdownNamespaceName:
		return record.NamespaceName
	case EventLogBreakdownInstallation:
		return record.InstallationID
	default:
		return ""
	}
}

func mockSortValue(entry EventLogBreakdown, sortBy EventLogBreakdownSort) int64 {
	switch sortBy {
	case EventLogBreakdownSortMatched:
		return entry.MatchedCount
	case EventLogBreakdownSortFailed:
		return entry.FailureCount
	default:
		return entry.EventCount
	}
}

func mockPercentile(values []int64, p float64) float64 {
	if len(values) == 0 {
		return 0
	}
	if p <= 0 {
		return float64(values[0])
	}
	if p >= 1 {
		return float64(values[len(values)-1])
	}
	index := int(float64(len(values)-1) * p)
	if index < 0 {
		index = 0
	}
	if index >= len(values) {
		index = len(values) - 1
	}
	return float64(values[index])
}

func mockParsePageToken(token string) (int, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return 0, nil
	}
	offset, err := strconv.Atoi(token)
	if err != nil || offset < 0 {
		return 0, errors.New("invalid page_token")
	}
	return offset, nil
}

func mockFormatPageToken(offset int) string {
	if offset <= 0 {
		return ""
	}
	return strconv.Itoa(offset)
}

func mockBodyHash(body []byte) string {
	if len(body) == 0 {
		return ""
	}
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}
