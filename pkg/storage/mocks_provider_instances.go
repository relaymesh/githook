package storage

import (
	"context"
	"sync"
	"time"
)

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
	if record.ID == "" {
		record.ID = ProviderInstanceRecordID(record)
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
