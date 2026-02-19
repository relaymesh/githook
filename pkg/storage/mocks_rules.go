package storage

import (
	"context"
	"strings"
	"sync"
	"time"
)

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
	record.DriverID = strings.TrimSpace(record.DriverID)
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
	record.DriverID = strings.TrimSpace(record.DriverID)
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
