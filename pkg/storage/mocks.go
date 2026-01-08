package storage

import (
	"context"
	"sync"
	"time"
)

// MockStore is an in-memory implementation of Store for tests.
type MockStore struct {
	mu     sync.RWMutex
	values map[string]InstallRecord
}

// NewMockStore returns a new in-memory Store.
func NewMockStore() *MockStore {
	return &MockStore{values: make(map[string]InstallRecord)}
}

func (m *MockStore) UpsertInstallation(ctx context.Context, record InstallRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if record.TenantID == "" {
		record.TenantID = TenantFromContext(ctx)
	}
	if record.CreatedAt.IsZero() {
		record.CreatedAt = time.Now().UTC()
	}
	record.UpdatedAt = time.Now().UTC()
	m.values[m.installKey(record.TenantID, record.Provider, record.AccountID, record.InstallationID)] = record
	return nil
}

func (m *MockStore) GetInstallation(ctx context.Context, provider, accountID, installationID string) (*InstallRecord, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	tenantID := TenantFromContext(ctx)
	record, ok := m.values[m.installKey(tenantID, provider, accountID, installationID)]
	if !ok {
		return nil, nil
	}
	copied := record
	return &copied, nil
}

func (m *MockStore) GetInstallationByInstallationID(ctx context.Context, provider, installationID string) (*InstallRecord, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var (
		found  bool
		latest InstallRecord
	)
	tenantID := TenantFromContext(ctx)
	for _, record := range m.values {
		if record.Provider != provider || record.InstallationID != installationID {
			continue
		}
		if tenantID != "" && record.TenantID != tenantID {
			continue
		}
		if !found || record.UpdatedAt.After(latest.UpdatedAt) {
			latest = record
			found = true
		}
	}
	if !found {
		return nil, nil
	}
	copied := latest
	return &copied, nil
}

func (m *MockStore) ListInstallations(ctx context.Context, provider, accountID string) ([]InstallRecord, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	results := make([]InstallRecord, 0)
	tenantID := TenantFromContext(ctx)
	for _, record := range m.values {
		if record.Provider == provider && record.AccountID == accountID {
			if tenantID != "" && record.TenantID != tenantID {
				continue
			}
			results = append(results, record)
		}
	}
	return results, nil
}

func (m *MockStore) Close() error {
	return nil
}

func (m *MockStore) installKey(tenantID, provider, accountID, installationID string) string {
	return tenantID + "|" + provider + "|" + accountID + "|" + installationID
}

// MockNamespaceStore is an in-memory implementation of NamespaceStore for tests.
type MockNamespaceStore struct {
	mu     sync.RWMutex
	values map[string]NamespaceRecord
}

// NewMockNamespaceStore returns a new in-memory NamespaceStore.
func NewMockNamespaceStore() *MockNamespaceStore {
	return &MockNamespaceStore{values: make(map[string]NamespaceRecord)}
}

func (m *MockNamespaceStore) UpsertNamespace(ctx context.Context, record NamespaceRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if record.TenantID == "" {
		record.TenantID = TenantFromContext(ctx)
	}
	if record.CreatedAt.IsZero() {
		record.CreatedAt = time.Now().UTC()
	}
	record.UpdatedAt = time.Now().UTC()
	m.values[m.namespaceKey(record.TenantID, record.Provider, record.ProviderInstanceKey, record.RepoID)] = record
	return nil
}

func (m *MockNamespaceStore) GetNamespace(ctx context.Context, provider, repoID, instanceKey string) (*NamespaceRecord, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	tenantID := TenantFromContext(ctx)
	record, ok := m.values[m.namespaceKey(tenantID, provider, instanceKey, repoID)]
	if !ok {
		return nil, nil
	}
	copied := record
	return &copied, nil
}

func (m *MockNamespaceStore) ListNamespaces(ctx context.Context, filter NamespaceFilter) ([]NamespaceRecord, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	results := make([]NamespaceRecord, 0)
	tenantID := filter.TenantID
	if tenantID == "" {
		tenantID = TenantFromContext(ctx)
	}
	for _, record := range m.values {
		if tenantID != "" && record.TenantID != tenantID {
			continue
		}
		if filter.Provider != "" && record.Provider != filter.Provider {
			continue
		}
		if filter.ProviderInstanceKey != "" && record.ProviderInstanceKey != filter.ProviderInstanceKey {
			continue
		}
		if filter.AccountID != "" && record.AccountID != filter.AccountID {
			continue
		}
		if filter.RepoID != "" && record.RepoID != filter.RepoID {
			continue
		}
		if filter.Owner != "" && record.Owner != filter.Owner {
			continue
		}
		if filter.RepoName != "" && record.RepoName != filter.RepoName {
			continue
		}
		if filter.FullName != "" && record.FullName != filter.FullName {
			continue
		}
		results = append(results, record)
	}
	return results, nil
}

func (m *MockNamespaceStore) Close() error {
	return nil
}

func (m *MockNamespaceStore) namespaceKey(tenantID, provider, instanceKey, repoID string) string {
	return tenantID + "|" + provider + "|" + instanceKey + "|" + repoID
}

// MockRuleStore is an in-memory implementation of RuleStore for tests.
type MockRuleStore struct {
	mu     sync.RWMutex
	values map[string]RuleRecord
}

// NewMockRuleStore returns a new in-memory RuleStore.
func NewMockRuleStore() *MockRuleStore {
	return &MockRuleStore{values: make(map[string]RuleRecord)}
}

func (m *MockRuleStore) ListRules(ctx context.Context) ([]RuleRecord, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	results := make([]RuleRecord, 0, len(m.values))
	tenantID := TenantFromContext(ctx)
	for _, record := range m.values {
		if tenantID != "" && record.TenantID != tenantID {
			continue
		}
		results = append(results, record)
	}
	return results, nil
}

func (m *MockRuleStore) GetRule(ctx context.Context, id string) (*RuleRecord, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	record, ok := m.values[id]
	if !ok {
		return nil, nil
	}
	tenantID := TenantFromContext(ctx)
	if tenantID != "" && record.TenantID != tenantID {
		return nil, nil
	}
	copied := record
	return &copied, nil
}

func (m *MockRuleStore) CreateRule(ctx context.Context, record RuleRecord) (*RuleRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if record.TenantID == "" {
		record.TenantID = TenantFromContext(ctx)
	}
	if record.ID == "" {
		record.ID = time.Now().UTC().Format("20060102150405.000000000")
	}
	if record.CreatedAt.IsZero() {
		record.CreatedAt = time.Now().UTC()
	}
	record.UpdatedAt = time.Now().UTC()
	m.values[record.ID] = record
	copied := record
	return &copied, nil
}

func (m *MockRuleStore) UpdateRule(ctx context.Context, record RuleRecord) (*RuleRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if record.TenantID == "" {
		record.TenantID = TenantFromContext(ctx)
	}
	if record.CreatedAt.IsZero() {
		record.CreatedAt = time.Now().UTC()
	}
	record.UpdatedAt = time.Now().UTC()
	m.values[record.ID] = record
	copied := record
	return &copied, nil
}

func (m *MockRuleStore) DeleteRule(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.values, id)
	return nil
}

func (m *MockRuleStore) Close() error {
	return nil
}

// MockProviderInstanceStore is an in-memory implementation of ProviderInstanceStore for tests.
type MockProviderInstanceStore struct {
	mu     sync.RWMutex
	values map[string]ProviderInstanceRecord
}

// NewMockProviderInstanceStore returns a new in-memory ProviderInstanceStore.
func NewMockProviderInstanceStore() *MockProviderInstanceStore {
	return &MockProviderInstanceStore{values: make(map[string]ProviderInstanceRecord)}
}

func (m *MockProviderInstanceStore) ListProviderInstances(ctx context.Context, provider string) ([]ProviderInstanceRecord, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	results := make([]ProviderInstanceRecord, 0, len(m.values))
	tenantID := TenantFromContext(ctx)
	for _, record := range m.values {
		if tenantID != "" && record.TenantID != tenantID {
			continue
		}
		if provider != "" && record.Provider != provider {
			continue
		}
		results = append(results, record)
	}
	return results, nil
}

func (m *MockProviderInstanceStore) GetProviderInstance(ctx context.Context, provider, key string) (*ProviderInstanceRecord, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	tenantID := TenantFromContext(ctx)
	item, ok := m.values[m.providerInstanceKey(tenantID, provider, key)]
	if !ok {
		return nil, nil
	}
	copied := item
	return &copied, nil
}

func (m *MockProviderInstanceStore) UpsertProviderInstance(ctx context.Context, record ProviderInstanceRecord) (*ProviderInstanceRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if record.TenantID == "" {
		record.TenantID = TenantFromContext(ctx)
	}
	if record.CreatedAt.IsZero() {
		record.CreatedAt = time.Now().UTC()
	}
	record.UpdatedAt = time.Now().UTC()
	key := m.providerInstanceKey(record.TenantID, record.Provider, record.Key)
	m.values[key] = record
	copied := record
	return &copied, nil
}

func (m *MockProviderInstanceStore) DeleteProviderInstance(ctx context.Context, provider, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	tenantID := TenantFromContext(ctx)
	delete(m.values, m.providerInstanceKey(tenantID, provider, key))
	return nil
}

func (m *MockProviderInstanceStore) Close() error {
	return nil
}

func (m *MockProviderInstanceStore) providerInstanceKey(tenantID, provider, key string) string {
	return tenantID + "|" + provider + "|" + key
}

// MockDriverStore is an in-memory implementation of DriverStore for tests.
type MockDriverStore struct {
	mu     sync.RWMutex
	values map[string]DriverRecord
}

// NewMockDriverStore returns a new in-memory DriverStore.
func NewMockDriverStore() *MockDriverStore {
	return &MockDriverStore{values: make(map[string]DriverRecord)}
}

func (m *MockDriverStore) ListDrivers(ctx context.Context) ([]DriverRecord, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	results := make([]DriverRecord, 0, len(m.values))
	tenantID := TenantFromContext(ctx)
	for _, record := range m.values {
		if tenantID != "" && record.TenantID != tenantID {
			continue
		}
		results = append(results, record)
	}
	return results, nil
}

func (m *MockDriverStore) GetDriver(ctx context.Context, name string) (*DriverRecord, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	tenantID := TenantFromContext(ctx)
	key := m.driverKey(tenantID, name)
	record, ok := m.values[key]
	if !ok {
		return nil, nil
	}
	copied := record
	return &copied, nil
}

func (m *MockDriverStore) UpsertDriver(ctx context.Context, record DriverRecord) (*DriverRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if record.TenantID == "" {
		record.TenantID = TenantFromContext(ctx)
	}
	if record.CreatedAt.IsZero() {
		record.CreatedAt = time.Now().UTC()
	}
	record.UpdatedAt = time.Now().UTC()
	key := m.driverKey(record.TenantID, record.Name)
	m.values[key] = record
	copied := record
	return &copied, nil
}

func (m *MockDriverStore) DeleteDriver(ctx context.Context, name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	tenantID := TenantFromContext(ctx)
	delete(m.values, m.driverKey(tenantID, name))
	return nil
}

func (m *MockDriverStore) Close() error {
	return nil
}

func (m *MockDriverStore) driverKey(tenantID, name string) string {
	return tenantID + "|" + name
}
