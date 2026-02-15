package cache

import "sync"

// TenantCache stores values keyed by tenant ID.
type TenantCache[T any] struct {
	mu    sync.RWMutex
	items map[string]T
}

// NewTenantCache initializes a tenant cache.
func NewTenantCache[T any]() *TenantCache[T] {
	return &TenantCache[T]{items: make(map[string]T)}
}

// Get returns a cached value for the tenant.
func (c *TenantCache[T]) Get(tenantID string) (T, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	val, ok := c.items[tenantID]
	return val, ok
}

// Set stores a value for the tenant.
func (c *TenantCache[T]) Set(tenantID string, value T) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items[tenantID] = value
}

// Delete removes a tenant entry.
func (c *TenantCache[T]) Delete(tenantID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.items, tenantID)
}

// Range iterates over entries.
func (c *TenantCache[T]) Range(fn func(string, T)) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	for key, val := range c.items {
		fn(key, val)
	}
}

func (c *TenantCache[T]) Keys() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	keys := make([]string, 0, len(c.items))
	for key := range c.items {
		keys = append(keys, key)
	}
	return keys
}
