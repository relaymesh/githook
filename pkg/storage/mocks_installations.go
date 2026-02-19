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
	if record.ID == "" {
		record.ID = InstallRecordID(record)
	}
	if record.CreatedAt.IsZero() {
		record.CreatedAt = time.Now().UTC()
	}
	record.UpdatedAt = time.Now().UTC()
	m.values[m.installKey(record.TenantID, record.Provider, record.AccountID, record.InstallationID, record.ProviderInstanceKey)] = record
	return nil
}

func (m *MockStore) GetInstallation(ctx context.Context, provider, accountID, installationID string) (*InstallRecord, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	tenantID := TenantFromContext(ctx)
	var (
		found  bool
		latest InstallRecord
	)
	for _, record := range m.values {
		if record.Provider != provider || record.AccountID != accountID || record.InstallationID != installationID {
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
		if record.Provider != provider {
			continue
		}
		if accountID != "" && record.AccountID != accountID {
			continue
		}
		if tenantID != "" && record.TenantID != tenantID {
			continue
		}
		results = append(results, record)
	}
	return results, nil
}

func (m *MockStore) DeleteInstallation(ctx context.Context, provider, accountID, installationID, instanceKey string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	tenantID := TenantFromContext(ctx)
	for key, record := range m.values {
		if record.Provider != provider || record.AccountID != accountID || record.InstallationID != installationID {
			continue
		}
		if instanceKey != "" && record.ProviderInstanceKey != instanceKey {
			continue
		}
		if tenantID != "" && record.TenantID != tenantID {
			continue
		}
		delete(m.values, key)
	}
	return nil
}

func (m *MockStore) Close() error {
	return nil
}

func (m *MockStore) installKey(tenantID, provider, accountID, installationID, instanceKey string) string {
	return tenantID + "|" + provider + "|" + accountID + "|" + installationID + "|" + instanceKey
}
