package eventlogs

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/relaymesh/githook/pkg/storage"

	"github.com/glebarez/sqlite"
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
	Pool        storage.PoolConfig
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
	HeadersJSON    string    `gorm:"column:headers_json;type:text"`
	Body           string    `gorm:"column:body;type:text"`
	BodyHash       string    `gorm:"column:body_hash;size:64;index"`
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
	if err := storage.ApplyPoolConfig(gormDB, cfg.Pool); err != nil {
		return nil, err
	}
	table := cfg.Table
	if table == "" {
		table = "github.com/relaymesh/githook_event_logs"
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

func (s *Store) migrate() error {
	return s.tableDB().AutoMigrate(&row{})
}

func (s *Store) tableDB() *gorm.DB {
	return s.db.Table(s.table)
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
