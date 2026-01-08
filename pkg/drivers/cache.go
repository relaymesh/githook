package drivers

import (
	"context"
	"errors"
	"log"

	"githooks/pkg/cache"
	"githooks/pkg/core"
	"githooks/pkg/storage"
)

// Cache maintains per-tenant driver configs and publishers.
type Cache struct {
	store  storage.DriverStore
	base   core.WatermillConfig
	logger *log.Logger
	config *cache.TenantCache[core.WatermillConfig]
	pub    *cache.TenantCache[core.Publisher]
}

// NewCache creates a new driver cache.
func NewCache(store storage.DriverStore, base core.WatermillConfig, logger *log.Logger) *Cache {
	if logger == nil {
		logger = log.Default()
	}
	return &Cache{
		store:  store,
		base:   base,
		logger: logger,
		config: cache.NewTenantCache[core.WatermillConfig](),
		pub:    cache.NewTenantCache[core.Publisher](),
	}
}

// Refresh reloads drivers for the tenant in the context and rebuilds the publisher.
func (c *Cache) Refresh(ctx context.Context) error {
	if c == nil || c.store == nil {
		return nil
	}
	tenantID := storage.TenantFromContext(ctx)
	records, err := c.store.ListDrivers(ctx)
	if err != nil {
		return err
	}
	if tenantID != "" {
		return c.refreshTenant(tenantID, records)
	}
	grouped := make(map[string][]storage.DriverRecord)
	for _, record := range records {
		grouped[record.TenantID] = append(grouped[record.TenantID], record)
	}
	for id, group := range grouped {
		if err := c.refreshTenant(id, group); err != nil {
			return err
		}
	}
	c.pub.Range(func(id string, existing core.Publisher) {
		if _, ok := grouped[id]; ok {
			return
		}
		if existing != nil {
			_ = existing.Close()
		}
		c.pub.Delete(id)
		c.config.Delete(id)
	})
	return nil
}

func (c *Cache) refreshTenant(tenantID string, records []storage.DriverRecord) error {
	cfg, err := ConfigFromRecords(c.base, records)
	if err != nil {
		return err
	}
	if len(cfg.Drivers) == 0 && cfg.Driver == "" {
		if existing, ok := c.pub.Get(tenantID); ok && existing != nil {
			_ = existing.Close()
		}
		c.pub.Delete(tenantID)
		c.config.Delete(tenantID)
		return nil
	}
	pub, err := core.NewPublisher(cfg)
	if err != nil {
		return err
	}
	if existing, ok := c.pub.Get(tenantID); ok && existing != nil {
		_ = existing.Close()
	}
	c.config.Set(tenantID, cfg)
	c.pub.Set(tenantID, pub)
	return nil
}

// PublisherFor returns a publisher for the tenant in the context.
func (c *Cache) PublisherFor(ctx context.Context) (core.Publisher, error) {
	if c == nil {
		return nil, errors.New("driver cache not configured")
	}
	tenantID := storage.TenantFromContext(ctx)
	if pub, ok := c.pub.Get(tenantID); ok && pub != nil {
		return pub, nil
	}
	if c.store == nil {
		return nil, errors.New("driver store not configured")
	}
	if err := c.Refresh(ctx); err != nil {
		return nil, err
	}
	pub, _ := c.pub.Get(tenantID)
	if pub == nil {
		return nil, errors.New("no publisher available")
	}
	return pub, nil
}

// Close closes all cached publishers.
func (c *Cache) Close() {
	if c == nil {
		return
	}
	c.pub.Range(func(tenantID string, pub core.Publisher) {
		if pub != nil {
			_ = pub.Close()
		}
		c.pub.Delete(tenantID)
		c.config.Delete(tenantID)
	})
}

// TenantPublisher routes publish calls to the cached publisher for each tenant.
type TenantPublisher struct {
	cache    *Cache
	fallback core.Publisher
}

// NewTenantPublisher creates a publisher that routes by tenant when possible.
func NewTenantPublisher(cache *Cache, fallback core.Publisher) core.Publisher {
	return &TenantPublisher{cache: cache, fallback: fallback}
}

func (p *TenantPublisher) Publish(ctx context.Context, topic string, event core.Event) error {
	pub, err := p.publisherFor(ctx)
	if err != nil {
		return err
	}
	return pub.Publish(ctx, topic, event)
}

func (p *TenantPublisher) PublishForDrivers(ctx context.Context, topic string, event core.Event, drivers []string) error {
	pub, err := p.publisherFor(ctx)
	if err != nil {
		return err
	}
	return pub.PublishForDrivers(ctx, topic, event, drivers)
}

func (p *TenantPublisher) Close() error {
	if p.cache != nil {
		p.cache.Close()
	}
	if p.fallback != nil {
		return p.fallback.Close()
	}
	return nil
}

func (p *TenantPublisher) publisherFor(ctx context.Context) (core.Publisher, error) {
	if p.cache == nil {
		if p.fallback == nil {
			return nil, errors.New("no publisher configured")
		}
		return p.fallback, nil
	}
	pub, err := p.cache.PublisherFor(ctx)
	if err == nil {
		return pub, nil
	}
	if p.fallback != nil && storage.TenantFromContext(ctx) == "" {
		return p.fallback, nil
	}
	return nil, err
}
