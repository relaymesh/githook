package storage

import (
	"context"
	"time"
)

// InstallRecord stores SCM installation or token metadata.
type InstallRecord struct {
	TenantID            string
	Provider            string
	AccountID           string
	AccountName         string
	InstallationID      string
	ProviderInstanceKey string
	AccessToken         string
	RefreshToken        string
	ExpiresAt           *time.Time
	MetadataJSON        string
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

// NamespaceRecord stores provider repository metadata.
type NamespaceRecord struct {
	TenantID            string
	Provider            string
	AccountID           string
	InstallationID      string
	ProviderInstanceKey string
	RepoID              string
	Owner               string
	RepoName            string
	FullName            string
	Visibility          string
	DefaultBranch       string
	HTTPURL             string
	SSHURL              string
	WebhooksEnabled     bool
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

// RuleRecord stores rule metadata.
type RuleRecord struct {
	TenantID  string
	ID        string
	When      string
	Emit      []string
	Drivers   []string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// DriverRecord stores Watermill driver config (per-tenant).
type DriverRecord struct {
	TenantID   string
	Name       string
	ConfigJSON string
	Enabled    bool
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// EventLogRecord stores metadata about webhook events and rule matches.
type EventLogRecord struct {
	TenantID       string
	ID             string
	Provider       string
	Name           string
	RequestID      string
	StateID        string
	InstallationID string
	NamespaceID    string
	NamespaceName  string
	Topic          string
	RuleID         string
	RuleWhen       string
	Drivers        []string
	Matched        bool
	CreatedAt      time.Time
	UpdatedAt      time.Time
	Status         string
	ErrorMessage   string
}

// EventLogFilter selects event log rows.
type EventLogFilter struct {
	TenantID       string
	Provider       string
	Name           string
	RequestID      string
	StateID        string
	InstallationID string
	NamespaceID    string
	NamespaceName  string
	Topic          string
	RuleID         string
	RuleWhen       string
	Matched        *bool
	StartTime      time.Time
	EndTime        time.Time
	Limit          int
	Offset         int
}

// EventLogCount represents an aggregate count bucket.
type EventLogCount struct {
	Key   string
	Count int64
}

// EventLogAnalytics contains aggregate data for event logs.
type EventLogAnalytics struct {
	Total       int64
	Matched     int64
	DistinctReq int64
	ByProvider  []EventLogCount
	ByEvent     []EventLogCount
	ByTopic     []EventLogCount
	ByRule      []EventLogCount
	ByInstall   []EventLogCount
	ByNamespace []EventLogCount
}

// ProviderInstanceRecord stores per-tenant provider instance config.
type ProviderInstanceRecord struct {
	TenantID   string
	Provider   string
	Key        string
	ConfigJSON string
	Enabled    bool
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// NamespaceFilter selects namespace rows.
type NamespaceFilter struct {
	TenantID            string
	Provider            string
	AccountID           string
	InstallationID      string
	ProviderInstanceKey string
	RepoID              string
	Owner               string
	RepoName            string
	FullName            string
}

// Store defines the persistence interface for installation records.
type Store interface {
	UpsertInstallation(ctx context.Context, record InstallRecord) error
	GetInstallation(ctx context.Context, provider, accountID, installationID string) (*InstallRecord, error)
	GetInstallationByInstallationID(ctx context.Context, provider, installationID string) (*InstallRecord, error)
	// ListInstallations lists installations for a provider, optionally filtered by accountID.
	ListInstallations(ctx context.Context, provider, accountID string) ([]InstallRecord, error)
	DeleteInstallation(ctx context.Context, provider, accountID, installationID, instanceKey string) error
	Close() error
}

// NamespaceStore defines persistence for provider repository metadata.
type NamespaceStore interface {
	UpsertNamespace(ctx context.Context, record NamespaceRecord) error
	GetNamespace(ctx context.Context, provider, repoID, instanceKey string) (*NamespaceRecord, error)
	ListNamespaces(ctx context.Context, filter NamespaceFilter) ([]NamespaceRecord, error)
	DeleteNamespace(ctx context.Context, provider, repoID, instanceKey string) error
	Close() error
}

// RuleStore defines persistence for rules.
type RuleStore interface {
	ListRules(ctx context.Context) ([]RuleRecord, error)
	GetRule(ctx context.Context, id string) (*RuleRecord, error)
	CreateRule(ctx context.Context, record RuleRecord) (*RuleRecord, error)
	UpdateRule(ctx context.Context, record RuleRecord) (*RuleRecord, error)
	DeleteRule(ctx context.Context, id string) error
	Close() error
}

// ProviderInstanceStore defines persistence for provider instance configs.
type ProviderInstanceStore interface {
	ListProviderInstances(ctx context.Context, provider string) ([]ProviderInstanceRecord, error)
	GetProviderInstance(ctx context.Context, provider, key string) (*ProviderInstanceRecord, error)
	UpsertProviderInstance(ctx context.Context, record ProviderInstanceRecord) (*ProviderInstanceRecord, error)
	DeleteProviderInstance(ctx context.Context, provider, key string) error
	Close() error
}

// DriverStore defines persistence for driver configs.
type DriverStore interface {
	ListDrivers(ctx context.Context) ([]DriverRecord, error)
	GetDriver(ctx context.Context, name string) (*DriverRecord, error)
	UpsertDriver(ctx context.Context, record DriverRecord) (*DriverRecord, error)
	DeleteDriver(ctx context.Context, name string) error
	Close() error
}

// EventLogStore defines persistence for webhook event logs.
type EventLogStore interface {
	CreateEventLogs(ctx context.Context, records []EventLogRecord) error
	ListEventLogs(ctx context.Context, filter EventLogFilter) ([]EventLogRecord, error)
	GetEventLogAnalytics(ctx context.Context, filter EventLogFilter) (EventLogAnalytics, error)
	UpdateEventLogStatus(ctx context.Context, id, status, errorMessage string) error
	Close() error
}
