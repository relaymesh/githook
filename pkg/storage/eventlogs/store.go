package eventlogs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"githook/pkg/storage"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// Config mirrors the storage configuration for the event logs table.
type Config struct {
	Driver      string
	DSN         string
	Dialect     string
	Table       string
	AutoMigrate bool
}

// Store implements storage.EventLogStore on top of GORM.
type Store struct {
	db    *gorm.DB
	table string
}

type row struct {
	ID             string    `gorm:"column:id;size:64;primaryKey"`
	TenantID       string    `gorm:"column:tenant_id;size:64;not null;default:'';index"`
	Provider       string    `gorm:"column:provider;size:32;not null;index"`
	Name           string    `gorm:"column:name;size:128;not null;index"`
	RequestID      string    `gorm:"column:request_id;size:128;index"`
	StateID        string    `gorm:"column:state_id;size:128;index"`
	InstallationID string    `gorm:"column:installation_id;size:128;index"`
	NamespaceID    string    `gorm:"column:namespace_id;size:128;index"`
	NamespaceName  string    `gorm:"column:namespace_name;size:256;index"`
	Topic          string    `gorm:"column:topic;size:128;index"`
	RuleID         string    `gorm:"column:rule_id;size:64;index"`
	RuleWhen       string    `gorm:"column:rule_when;type:text"`
	DriversJSON    string    `gorm:"column:drivers_json;type:text"`
	Matched        bool      `gorm:"column:matched;not null;default:false;index"`
	Status         string    `gorm:"column:status;size:32;not null;default:'queued';index"`
	ErrorMessage   string    `gorm:"column:error_message;type:text"`
	LatencyMS      int64     `gorm:"column:latency_ms;not null;default:0;index"`
	CreatedAt      time.Time `gorm:"column:created_at;autoCreateTime;index"`
	UpdatedAt      time.Time `gorm:"column:updated_at;autoUpdateTime;index"`
}

// Open creates a GORM-backed event logs store.
func Open(cfg Config) (*Store, error) {
	if cfg.Driver == "" && cfg.Dialect == "" {
		return nil, errors.New("storage driver or dialect is required")
	}
	if cfg.DSN == "" {
		return nil, errors.New("storage dsn is required")
	}
	driver := normalizeDriver(cfg.Driver)
	if driver == "" {
		driver = normalizeDriver(cfg.Dialect)
	}
	if driver == "" {
		return nil, errors.New("unsupported storage driver")
	}
	gormDB, err := openGorm(driver, cfg.DSN)
	if err != nil {
		return nil, err
	}
	table := cfg.Table
	if table == "" {
		table = "githook_event_logs"
	}
	store := &Store{db: gormDB, table: table}
	if cfg.AutoMigrate {
		if err := store.migrate(); err != nil {
			return nil, err
		}
	}
	return store, nil
}

// Close closes the underlying DB connection.
func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	sqlDB, err := s.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// CreateEventLogs inserts event log records.
func (s *Store) CreateEventLogs(ctx context.Context, records []storage.EventLogRecord) error {
	if s == nil || s.db == nil {
		return errors.New("store is not initialized")
	}
	if len(records) == 0 {
		return nil
	}
	now := time.Now().UTC()
	rows := make([]row, 0, len(records))
	for _, record := range records {
		if record.TenantID == "" {
			record.TenantID = storage.TenantFromContext(ctx)
		}
		if record.ID == "" {
			record.ID = uuid.NewString()
		}
		if record.Status == "" {
			record.Status = "queued"
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
		data, err := toRow(record)
		if err != nil {
			return err
		}
		rows = append(rows, data)
	}
	return s.tableDB().WithContext(ctx).Create(&rows).Error
}

// ListEventLogs returns event logs matching the filter.
func (s *Store) ListEventLogs(ctx context.Context, filter storage.EventLogFilter) ([]storage.EventLogRecord, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("store is not initialized")
	}
	query := applyFilter(s.tableDB().WithContext(ctx), filter, ctx).
		Order("created_at desc")
	if filter.Limit > 0 {
		query = query.Limit(filter.Limit)
	}
	if filter.Offset > 0 {
		query = query.Offset(filter.Offset)
	}
	var data []row
	if err := query.Find(&data).Error; err != nil {
		return nil, err
	}
	out := make([]storage.EventLogRecord, 0, len(data))
	for _, item := range data {
		out = append(out, fromRow(item))
	}
	return out, nil
}

// UpdateEventLogStatus updates the status and error message of an event log.
func (s *Store) UpdateEventLogStatus(ctx context.Context, id, status, errorMessage string) error {
	if s == nil || s.db == nil {
		return errors.New("store is not initialized")
	}
	id = strings.TrimSpace(id)
	status = strings.TrimSpace(status)
	if id == "" {
		return errors.New("id is required")
	}
	if status == "" {
		return errors.New("status is required")
	}
	tenantID := storage.TenantFromContext(ctx)
	query := s.tableDB().WithContext(ctx).Where("id = ?", id)
	if tenantID != "" {
		query = query.Where("tenant_id = ?", tenantID)
	}
	updates := map[string]interface{}{
		"status":        status,
		"error_message": strings.TrimSpace(errorMessage),
		"updated_at":    time.Now().UTC(),
	}
	if status == "success" || status == "failed" {
		var existing row
		if err := query.Select("created_at").First(&existing).Error; err == nil && !existing.CreatedAt.IsZero() {
			updates["latency_ms"] = time.Since(existing.CreatedAt).Milliseconds()
		}
	}
	return query.Updates(updates).Error
}

// GetEventLogAnalytics returns aggregate analytics for event logs.
func (s *Store) GetEventLogAnalytics(ctx context.Context, filter storage.EventLogFilter) (storage.EventLogAnalytics, error) {
	if s == nil || s.db == nil {
		return storage.EventLogAnalytics{}, errors.New("store is not initialized")
	}
	base := applyFilter(s.tableDB().WithContext(ctx), filter, ctx)

	var total int64
	if err := base.Model(&row{}).Count(&total).Error; err != nil {
		return storage.EventLogAnalytics{}, err
	}

	var matched int64
	if err := base.Model(&row{}).Where("matched = ?", true).Count(&matched).Error; err != nil {
		return storage.EventLogAnalytics{}, err
	}

	var distinctReq int64
	if err := base.Model(&row{}).Where("request_id <> ''").Distinct("request_id").Count(&distinctReq).Error; err != nil {
		return storage.EventLogAnalytics{}, err
	}

	byProvider, err := aggregateCounts(base, "provider", "provider")
	if err != nil {
		return storage.EventLogAnalytics{}, err
	}
	byEvent, err := aggregateCounts(base, "name", "name")
	if err != nil {
		return storage.EventLogAnalytics{}, err
	}
	byTopic, err := aggregateCounts(base.Where("topic <> ''"), "topic", "topic")
	if err != nil {
		return storage.EventLogAnalytics{}, err
	}
	byRule, err := aggregateCounts(base, "COALESCE(NULLIF(rule_id,''), rule_when)", "COALESCE(NULLIF(rule_id,''), rule_when)")
	if err != nil {
		return storage.EventLogAnalytics{}, err
	}
	byInstall, err := aggregateCounts(base.Where("installation_id <> ''"), "installation_id", "installation_id")
	if err != nil {
		return storage.EventLogAnalytics{}, err
	}
	byNamespace, err := aggregateCounts(base, "COALESCE(NULLIF(namespace_name,''), namespace_id)", "COALESCE(NULLIF(namespace_name,''), namespace_id)")
	if err != nil {
		return storage.EventLogAnalytics{}, err
	}

	return storage.EventLogAnalytics{
		Total:       total,
		Matched:     matched,
		DistinctReq: distinctReq,
		ByProvider:  byProvider,
		ByEvent:     byEvent,
		ByTopic:     byTopic,
		ByRule:      byRule,
		ByInstall:   byInstall,
		ByNamespace: byNamespace,
	}, nil
}

// GetEventLogTimeseries returns time-series buckets grouped by interval.
func (s *Store) GetEventLogTimeseries(ctx context.Context, filter storage.EventLogFilter, interval storage.EventLogInterval) ([]storage.EventLogTimeseriesBucket, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("store is not initialized")
	}
	if interval == "" {
		return nil, errors.New("interval is required")
	}
	if filter.StartTime.IsZero() || filter.EndTime.IsZero() {
		return nil, errors.New("start_time and end_time are required")
	}
	if filter.EndTime.Before(filter.StartTime) {
		return nil, errors.New("end_time must be after start_time")
	}

	query := applyFilter(s.tableDB().WithContext(ctx), filter, ctx)
	var rows []struct {
		CreatedAt time.Time `gorm:"column:created_at"`
		Matched   bool      `gorm:"column:matched"`
		RequestID string    `gorm:"column:request_id"`
		Status    string    `gorm:"column:status"`
	}
	if err := query.Select("created_at", "matched", "request_id", "status").Order("created_at asc").Find(&rows).Error; err != nil {
		return nil, err
	}

	start := bucketStart(filter.StartTime.UTC(), interval)
	end := filter.EndTime.UTC()
	step := intervalDuration(interval)
	if step <= 0 {
		return nil, errors.New("invalid interval")
	}

	type bucketData struct {
		storage.EventLogTimeseriesBucket
		reqs map[string]struct{}
	}
	buckets := make(map[int64]*bucketData)

	for _, row := range rows {
		ts := row.CreatedAt.UTC()
		if ts.Before(start) || ts.After(end) {
			continue
		}
		bucket := bucketStart(ts, interval)
		key := bucket.Unix()
		entry := buckets[key]
		if entry == nil {
			entry = &bucketData{
				EventLogTimeseriesBucket: storage.EventLogTimeseriesBucket{
					Start: bucket,
					End:   bucket.Add(step),
				},
				reqs: make(map[string]struct{}),
			}
			buckets[key] = entry
		}
		entry.EventCount++
		if row.Matched {
			entry.MatchedCount++
		}
		if row.RequestID != "" {
			entry.reqs[row.RequestID] = struct{}{}
		}
		if strings.EqualFold(row.Status, "failed") {
			entry.FailureCount++
		}
	}

	out := make([]storage.EventLogTimeseriesBucket, 0)
	for cursor := start; cursor.Before(end) || cursor.Equal(end); cursor = cursor.Add(step) {
		key := cursor.Unix()
		if entry, ok := buckets[key]; ok {
			entry.DistinctReq = int64(len(entry.reqs))
			out = append(out, entry.EventLogTimeseriesBucket)
		} else {
			out = append(out, storage.EventLogTimeseriesBucket{
				Start: cursor,
				End:   cursor.Add(step),
			})
		}
	}
	return out, nil
}

// GetEventLogBreakdown returns grouped aggregates and an optional next page token.
func (s *Store) GetEventLogBreakdown(ctx context.Context, filter storage.EventLogFilter, groupBy storage.EventLogBreakdownGroup, sortBy storage.EventLogBreakdownSort, sortDesc bool, pageSize int, pageToken string, includeLatency bool) ([]storage.EventLogBreakdown, string, error) {
	if s == nil || s.db == nil {
		return nil, "", errors.New("store is not initialized")
	}
	groupExpr, err := breakdownGroupExpr(groupBy)
	if err != nil {
		return nil, "", err
	}
	orderExpr := breakdownSortExpr(sortBy, sortDesc)
	offset, err := parsePageToken(pageToken)
	if err != nil {
		return nil, "", err
	}
	if pageSize <= 0 {
		pageSize = 50
	}

	query := applyFilter(s.tableDB().WithContext(ctx), filter, ctx)
	selectExpr := strings.Join([]string{
		groupExpr + " as key",
		"count(*) as count",
		"sum(case when matched = true then 1 else 0 end) as matched_count",
		"sum(case when status = 'failed' then 1 else 0 end) as failed_count",
	}, ", ")

	type breakdownRow struct {
		Key          string `gorm:"column:key"`
		Count        int64  `gorm:"column:count"`
		MatchedCount int64  `gorm:"column:matched_count"`
		FailedCount  int64  `gorm:"column:failed_count"`
	}
	var rows []breakdownRow
	if err := query.Select(selectExpr).Group(groupExpr).Order(orderExpr).Limit(pageSize).Offset(offset).Find(&rows).Error; err != nil {
		return nil, "", err
	}

	out := make([]storage.EventLogBreakdown, 0, len(rows))
	keys := make([]string, 0, len(rows))
	for _, row := range rows {
		if strings.TrimSpace(row.Key) == "" {
			continue
		}
		keys = append(keys, row.Key)
		out = append(out, storage.EventLogBreakdown{
			Key:          row.Key,
			EventCount:   row.Count,
			MatchedCount: row.MatchedCount,
			FailureCount: row.FailedCount,
		})
	}

	if includeLatency && len(keys) > 0 {
		stats, err := s.fetchLatencyByGroup(ctx, filter, groupExpr, keys)
		if err != nil {
			return nil, "", err
		}
		for i := range out {
			if values, ok := stats[out[i].Key]; ok {
				out[i].LatencyP50MS = values.P50
				out[i].LatencyP95MS = values.P95
				out[i].LatencyP99MS = values.P99
			}
		}
	}

	nextToken := ""
	if len(rows) == pageSize {
		nextToken = formatPageToken(offset + pageSize)
	}
	return out, nextToken, nil
}

func (s *Store) migrate() error {
	return s.tableDB().AutoMigrate(&row{})
}

func (s *Store) tableDB() *gorm.DB {
	return s.db.Table(s.table)
}

func applyFilter(query *gorm.DB, filter storage.EventLogFilter, ctx context.Context) *gorm.DB {
	tenantID := filter.TenantID
	if tenantID == "" {
		tenantID = storage.TenantFromContext(ctx)
	}
	if tenantID != "" {
		query = query.Where("tenant_id = ?", tenantID)
	}
	if filter.Provider != "" {
		query = query.Where("provider = ?", filter.Provider)
	}
	if filter.Name != "" {
		query = query.Where("name = ?", filter.Name)
	}
	if filter.RequestID != "" {
		query = query.Where("request_id = ?", filter.RequestID)
	}
	if filter.StateID != "" {
		query = query.Where("state_id = ?", filter.StateID)
	}
	if filter.InstallationID != "" {
		query = query.Where("installation_id = ?", filter.InstallationID)
	}
	if filter.NamespaceID != "" {
		query = query.Where("namespace_id = ?", filter.NamespaceID)
	}
	if filter.NamespaceName != "" {
		query = query.Where("namespace_name = ?", filter.NamespaceName)
	}
	if filter.Topic != "" {
		query = query.Where("topic = ?", filter.Topic)
	}
	if filter.RuleID != "" {
		query = query.Where("rule_id = ?", filter.RuleID)
	}
	if filter.RuleWhen != "" {
		query = query.Where("rule_when = ?", filter.RuleWhen)
	}
	if filter.Matched != nil {
		query = query.Where("matched = ?", *filter.Matched)
	}
	if !filter.StartTime.IsZero() {
		query = query.Where("created_at >= ?", filter.StartTime)
	}
	if !filter.EndTime.IsZero() {
		query = query.Where("created_at <= ?", filter.EndTime)
	}
	return query
}

type countRow struct {
	Key   string `gorm:"column:key"`
	Count int64  `gorm:"column:count"`
}

func aggregateCounts(query *gorm.DB, selectExpr, groupExpr string) ([]storage.EventLogCount, error) {
	var rows []countRow
	if err := query.Select(selectExpr + " as key, count(*) as count").Group(groupExpr).Order("count desc").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]storage.EventLogCount, 0, len(rows))
	for _, row := range rows {
		if row.Key == "" {
			continue
		}
		out = append(out, storage.EventLogCount{Key: row.Key, Count: row.Count})
	}
	return out, nil
}

func breakdownGroupExpr(groupBy storage.EventLogBreakdownGroup) (string, error) {
	switch groupBy {
	case storage.EventLogBreakdownProvider:
		return "provider", nil
	case storage.EventLogBreakdownEvent:
		return "name", nil
	case storage.EventLogBreakdownRuleID:
		return "rule_id", nil
	case storage.EventLogBreakdownRuleWhen:
		return "rule_when", nil
	case storage.EventLogBreakdownTopic:
		return "topic", nil
	case storage.EventLogBreakdownNamespaceID:
		return "namespace_id", nil
	case storage.EventLogBreakdownNamespaceName:
		return "namespace_name", nil
	case storage.EventLogBreakdownInstallation:
		return "installation_id", nil
	default:
		return "", errors.New("unsupported group_by")
	}
}

func breakdownSortExpr(sortBy storage.EventLogBreakdownSort, sortDesc bool) string {
	column := "count"
	switch sortBy {
	case storage.EventLogBreakdownSortMatched:
		column = "matched_count"
	case storage.EventLogBreakdownSortFailed:
		column = "failed_count"
	case storage.EventLogBreakdownSortCount:
		column = "count"
	default:
		column = "count"
	}
	if sortDesc {
		return column + " desc"
	}
	return column + " asc"
}

func intervalDuration(interval storage.EventLogInterval) time.Duration {
	switch interval {
	case storage.EventLogIntervalHour:
		return time.Hour
	case storage.EventLogIntervalDay:
		return 24 * time.Hour
	case storage.EventLogIntervalWeek:
		return 7 * 24 * time.Hour
	default:
		return 0
	}
}

func bucketStart(ts time.Time, interval storage.EventLogInterval) time.Time {
	ts = ts.UTC()
	switch interval {
	case storage.EventLogIntervalHour:
		return ts.Truncate(time.Hour)
	case storage.EventLogIntervalDay:
		return time.Date(ts.Year(), ts.Month(), ts.Day(), 0, 0, 0, 0, time.UTC)
	case storage.EventLogIntervalWeek:
		day := time.Date(ts.Year(), ts.Month(), ts.Day(), 0, 0, 0, 0, time.UTC)
		weekday := int(day.Weekday())
		shift := (weekday + 6) % 7
		return day.AddDate(0, 0, -shift)
	default:
		return ts
	}
}

func parsePageToken(token string) (int, error) {
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

func formatPageToken(offset int) string {
	if offset <= 0 {
		return ""
	}
	return strconv.Itoa(offset)
}

type latencyStats struct {
	P50 float64
	P95 float64
	P99 float64
}

func (s *Store) fetchLatencyByGroup(ctx context.Context, filter storage.EventLogFilter, groupExpr string, keys []string) (map[string]latencyStats, error) {
	query := applyFilter(s.tableDB().WithContext(ctx), filter, ctx)
	type latencyRow struct {
		Key       string `gorm:"column:key"`
		LatencyMS int64  `gorm:"column:latency_ms"`
	}
	var rows []latencyRow
	if err := query.Select(groupExpr+" as key", "latency_ms").
		Where("latency_ms > 0").
		Where(groupExpr+" IN ?", keys).
		Find(&rows).Error; err != nil {
		return nil, err
	}

	grouped := make(map[string][]int64)
	for _, row := range rows {
		if strings.TrimSpace(row.Key) == "" {
			continue
		}
		grouped[row.Key] = append(grouped[row.Key], row.LatencyMS)
	}

	out := make(map[string]latencyStats, len(grouped))
	for key, values := range grouped {
		sort.Slice(values, func(i, j int) bool { return values[i] < values[j] })
		out[key] = latencyStats{
			P50: percentile(values, 0.50),
			P95: percentile(values, 0.95),
			P99: percentile(values, 0.99),
		}
	}
	return out, nil
}

func percentile(values []int64, p float64) float64 {
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

func toRow(record storage.EventLogRecord) (row, error) {
	driversJSON, err := json.Marshal(record.Drivers)
	if err != nil {
		return row{}, err
	}
	return row{
		ID:             record.ID,
		TenantID:       record.TenantID,
		Provider:       record.Provider,
		Name:           record.Name,
		RequestID:      record.RequestID,
		StateID:        record.StateID,
		InstallationID: record.InstallationID,
		NamespaceID:    record.NamespaceID,
		NamespaceName:  record.NamespaceName,
		Topic:          record.Topic,
		RuleID:         record.RuleID,
		RuleWhen:       record.RuleWhen,
		DriversJSON:    string(driversJSON),
		Matched:        record.Matched,
		Status:         record.Status,
		ErrorMessage:   record.ErrorMessage,
		LatencyMS:      record.LatencyMS,
		CreatedAt:      record.CreatedAt,
		UpdatedAt:      record.UpdatedAt,
	}, nil
}

func fromRow(data row) storage.EventLogRecord {
	record := storage.EventLogRecord{
		ID:             data.ID,
		TenantID:       data.TenantID,
		Provider:       data.Provider,
		Name:           data.Name,
		RequestID:      data.RequestID,
		StateID:        data.StateID,
		InstallationID: data.InstallationID,
		NamespaceID:    data.NamespaceID,
		NamespaceName:  data.NamespaceName,
		Topic:          data.Topic,
		RuleID:         data.RuleID,
		RuleWhen:       data.RuleWhen,
		Matched:        data.Matched,
		Status:         data.Status,
		ErrorMessage:   data.ErrorMessage,
		LatencyMS:      data.LatencyMS,
		CreatedAt:      data.CreatedAt,
		UpdatedAt:      data.UpdatedAt,
	}
	if data.DriversJSON != "" {
		_ = json.Unmarshal([]byte(data.DriversJSON), &record.Drivers)
	}
	return record
}

func normalizeDriver(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	switch value {
	case "postgres", "postgresql", "pgx":
		return "postgres"
	case "mysql":
		return "mysql"
	case "sqlite", "sqlite3":
		return "sqlite"
	default:
		return ""
	}
}

func openGorm(driver, dsn string) (*gorm.DB, error) {
	switch driver {
	case "postgres":
		return gorm.Open(postgres.Open(dsn), &gorm.Config{})
	case "mysql":
		return gorm.Open(mysql.Open(dsn), &gorm.Config{})
	case "sqlite":
		return gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	default:
		return nil, fmt.Errorf("unsupported driver %q", driver)
	}
}
