package storage

import (
	"context"
	"sync"
	"time"
)

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
	if record.ID == "" {
		record.ID = NamespaceRecordID(record)
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
		if filter.InstallationID != "" && record.InstallationID != filter.InstallationID {
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

func (m *MockNamespaceStore) DeleteNamespace(ctx context.Context, provider, repoID, instanceKey string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	tenantID := TenantFromContext(ctx)
	for key, record := range m.values {
		if record.Provider != provider || record.RepoID != repoID {
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

func (m *MockNamespaceStore) Close() error {
	return nil
}

func (m *MockNamespaceStore) namespaceKey(tenantID, provider, instanceKey, repoID string) string {
	return tenantID + "|" + provider + "|" + instanceKey + "|" + repoID
}
