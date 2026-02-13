package eventlogs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
	return query.Updates(map[string]interface{}{
		"status":        status,
		"error_message": strings.TrimSpace(errorMessage),
		"updated_at":    time.Now().UTC(),
	}).Error
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
