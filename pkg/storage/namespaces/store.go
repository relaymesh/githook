package namespaces

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"githook/pkg/storage"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Config mirrors the storage configuration for the namespaces table.
type Config struct {
	Driver      string
	DSN         string
	Dialect     string
	Table       string
	AutoMigrate bool
}

// Store implements storage.NamespaceStore on top of GORM.
type Store struct {
	db    *gorm.DB
	table string
}

type row struct {
	TenantID        string    `gorm:"column:tenant_id;size:64;not null;default:'';uniqueIndex:idx_namespace,priority:1"`
	Provider        string    `gorm:"column:provider;size:32;not null;uniqueIndex:idx_namespace,priority:2"`
	InstanceKey     string    `gorm:"column:provider_instance_key;size:64;uniqueIndex:idx_namespace,priority:3"`
	RepoID          string    `gorm:"column:repo_id;size:128;not null;uniqueIndex:idx_namespace,priority:4"`
	AccountID       string    `gorm:"column:account_id;size:128;not null"`
	InstallationID  string    `gorm:"column:installation_id;size:128"`
	Owner           string    `gorm:"column:owner;size:255"`
	RepoName        string    `gorm:"column:repo_name;size:255"`
	FullName        string    `gorm:"column:full_name;size:255"`
	Visibility      string    `gorm:"column:visibility;size:32"`
	DefaultBranch   string    `gorm:"column:default_branch;size:255"`
	HTTPURL         string    `gorm:"column:http_url;size:512"`
	SSHURL          string    `gorm:"column:ssh_url;size:512"`
	WebhooksEnabled bool      `gorm:"column:webhooks_enabled"`
	CreatedAt       time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt       time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

// Open creates a GORM-backed namespaces store.
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
		table = "git_namespaces"
	}
	store := &Store{
		db:    gormDB,
		table: table,
	}
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

// UpsertNamespace inserts or updates a namespace record.
func (s *Store) UpsertNamespace(ctx context.Context, record storage.NamespaceRecord) error {
	if s == nil || s.db == nil {
		return errors.New("store is not initialized")
	}
	if record.Provider == "" || record.RepoID == "" {
		return errors.New("provider and repo_id are required")
	}
	if record.TenantID == "" {
		record.TenantID = storage.TenantFromContext(ctx)
	}
	now := time.Now().UTC()
	if record.CreatedAt.IsZero() {
		record.CreatedAt = now
	}
	record.UpdatedAt = now

	data := toRow(record)
	return s.tableDB().
		WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "tenant_id"}, {Name: "provider"}, {Name: "provider_instance_key"}, {Name: "repo_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"account_id", "installation_id", "provider_instance_key", "owner", "repo_name", "full_name", "visibility", "default_branch", "http_url", "ssh_url", "webhooks_enabled", "updated_at"}),
		}).
		Create(&data).Error
}

// GetNamespace fetches a namespace by provider/repo ID.
func (s *Store) GetNamespace(ctx context.Context, provider, repoID, instanceKey string) (*storage.NamespaceRecord, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("store is not initialized")
	}
	var data row
	query := s.tableDB().
		WithContext(ctx).
		Where("provider = ? AND repo_id = ?", provider, repoID)
	if instanceKey != "" {
		query = query.Where("provider_instance_key = ?", instanceKey)
	}
	if tenantID := storage.TenantFromContext(ctx); tenantID != "" {
		query = query.Where("tenant_id = ?", tenantID)
	}
	err := query.Take(&data).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	record := fromRow(data)
	return &record, nil
}

// ListNamespaces lists namespaces by filter.
func (s *Store) ListNamespaces(ctx context.Context, filter storage.NamespaceFilter) ([]storage.NamespaceRecord, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("store is not initialized")
	}
	query := s.tableDB().WithContext(ctx)
	if filter.TenantID != "" {
		query = query.Where("tenant_id = ?", filter.TenantID)
	} else if tenantID := storage.TenantFromContext(ctx); tenantID != "" {
		query = query.Where("tenant_id = ?", tenantID)
	}
	if filter.Provider != "" {
		query = query.Where("provider = ?", filter.Provider)
	}
	if filter.ProviderInstanceKey != "" {
		query = query.Where("provider_instance_key = ?", filter.ProviderInstanceKey)
	}
	if filter.AccountID != "" {
		query = query.Where("account_id = ?", filter.AccountID)
	}
	if filter.RepoID != "" {
		query = query.Where("repo_id = ?", filter.RepoID)
	}
	if filter.Owner != "" {
		query = query.Where("owner = ?", filter.Owner)
	}
	if filter.RepoName != "" {
		query = query.Where("repo_name = ?", filter.RepoName)
	}
	if filter.FullName != "" {
		query = query.Where("full_name = ?", filter.FullName)
	}
	var data []row
	err := query.Find(&data).Error
	if err != nil {
		return nil, err
	}
	records := make([]storage.NamespaceRecord, 0, len(data))
	for _, item := range data {
		records = append(records, fromRow(item))
	}
	return records, nil
}

// UpdateProviderInstanceKey updates the provider instance key for a provider and tenant.
func (s *Store) UpdateProviderInstanceKey(ctx context.Context, provider, oldKey, newKey, tenantID string) (int64, error) {
	if s == nil || s.db == nil {
		return 0, errors.New("store is not initialized")
	}
	provider = strings.TrimSpace(provider)
	oldKey = strings.TrimSpace(oldKey)
	newKey = strings.TrimSpace(newKey)
	tenantID = strings.TrimSpace(tenantID)
	if provider == "" || oldKey == "" || newKey == "" {
		return 0, errors.New("provider and keys are required")
	}
	query := s.tableDB().
		WithContext(ctx).
		Where("provider = ? AND provider_instance_key = ?", provider, oldKey).
		Where("tenant_id = ?", tenantID)
	result := query.Updates(map[string]interface{}{
		"provider_instance_key": newKey,
		"updated_at":            time.Now().UTC(),
	})
	return result.RowsAffected, result.Error
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
		return nil, fmt.Errorf("unsupported storage driver: %s", driver)
	}
}

func toRow(record storage.NamespaceRecord) row {
	return row{
		TenantID:        record.TenantID,
		Provider:        record.Provider,
		RepoID:          record.RepoID,
		AccountID:       record.AccountID,
		InstallationID:  record.InstallationID,
		InstanceKey:     record.ProviderInstanceKey,
		Owner:           record.Owner,
		RepoName:        record.RepoName,
		FullName:        record.FullName,
		Visibility:      record.Visibility,
		DefaultBranch:   record.DefaultBranch,
		HTTPURL:         record.HTTPURL,
		SSHURL:          record.SSHURL,
		WebhooksEnabled: record.WebhooksEnabled,
		CreatedAt:       record.CreatedAt,
		UpdatedAt:       record.UpdatedAt,
	}
}

func fromRow(data row) storage.NamespaceRecord {
	return storage.NamespaceRecord{
		TenantID:            data.TenantID,
		Provider:            data.Provider,
		RepoID:              data.RepoID,
		AccountID:           data.AccountID,
		InstallationID:      data.InstallationID,
		ProviderInstanceKey: data.InstanceKey,
		Owner:               data.Owner,
		RepoName:            data.RepoName,
		FullName:            data.FullName,
		Visibility:          data.Visibility,
		DefaultBranch:       data.DefaultBranch,
		HTTPURL:             data.HTTPURL,
		SSHURL:              data.SSHURL,
		WebhooksEnabled:     data.WebhooksEnabled,
		CreatedAt:           data.CreatedAt,
		UpdatedAt:           data.UpdatedAt,
	}
}
