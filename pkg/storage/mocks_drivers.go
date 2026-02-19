package storage

import (
	"context"
	"strings"
	"sync"
	"time"
)

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
	tenantID := TenantFromContext(ctx)
	results := make([]DriverRecord, 0, len(m.values))
	for _, record := range m.values {
		if record.TenantID == tenantID {
			results = append(results, record)
		}
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

func (m *MockDriverStore) GetDriverByID(ctx context.Context, id string) (*DriverRecord, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	tenantID := TenantFromContext(ctx)
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, nil
	}
	for _, record := range m.values {
		if record.TenantID != tenantID {
			continue
		}
		if record.ID == id {
			copied := record
			return &copied, nil
		}
	}
	return nil, nil
}

func (m *MockDriverStore) UpsertDriver(ctx context.Context, record DriverRecord) (*DriverRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	tenantID := TenantFromContext(ctx)
	record.TenantID = tenantID
	if record.ID == "" {
		if tenantID != "" {
			record.ID = tenantID + ":" + record.Name
		} else {
			record.ID = record.Name
		}
	}
	if record.CreatedAt.IsZero() {
		record.CreatedAt = time.Now().UTC()
	}
	record.UpdatedAt = time.Now().UTC()
	for key, stored := range m.values {
		if stored.TenantID == tenantID && stored.Name != record.Name {
			delete(m.values, key)
		}
	}
	key := m.driverKey(tenantID, record.Name)
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
