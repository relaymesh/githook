package drivers

import (
	"context"
	"testing"

	"github.com/relaymesh/githook/pkg/core"
	"github.com/relaymesh/githook/pkg/storage"
)

type stubPublisher struct {
	published bool
	closed    bool
}

func (s *stubPublisher) Publish(ctx context.Context, topic string, event core.Event) error {
	s.published = true
	return nil
}

func (s *stubPublisher) PublishForDrivers(ctx context.Context, topic string, event core.Event, drivers []string) error {
	s.published = true
	return nil
}

func (s *stubPublisher) Close() error {
	s.closed = true
	return nil
}

func TestCacheKeyFromContext(t *testing.T) {
	ctx := context.Background()
	if key := cacheKeyFromContext(ctx); key != globalDriverKey {
		t.Fatalf("expected global key")
	}
	ctx = storage.WithTenant(ctx, "tenant-a")
	if key := cacheKeyFromContext(ctx); key != "tenant-a" {
		t.Fatalf("expected tenant key")
	}
}

func TestCachePublisherForMissingStore(t *testing.T) {
	cache := NewCache(nil, core.RelaybusConfig{}, nil)
	if _, err := cache.PublisherFor(context.Background()); err == nil {
		t.Fatalf("expected error for missing store")
	}
}

func TestTenantPublisherFallback(t *testing.T) {
	fallback := &stubPublisher{}
	tenantPub := NewTenantPublisher(NewCache(nil, core.RelaybusConfig{}, nil), fallback)
	if err := tenantPub.Publish(context.Background(), "topic", core.Event{}); err != nil {
		t.Fatalf("publish: %v", err)
	}
	if !fallback.published {
		t.Fatalf("expected fallback publish")
	}
}

func TestCacheClose(t *testing.T) {
	cache := NewCache(nil, core.RelaybusConfig{}, nil)
	cache.pub.Set(globalDriverKey, &stubPublisher{})
	cache.Close()
	pub, ok := cache.pub.Get(globalDriverKey)
	if ok && pub != nil {
		t.Fatalf("expected publisher removed")
	}
}

func TestTenantPublisherNoFallback(t *testing.T) {
	tenantPub := NewTenantPublisher(NewCache(nil, core.RelaybusConfig{}, nil), nil)
	if err := tenantPub.Publish(context.Background(), "topic", core.Event{}); err == nil {
		t.Fatalf("expected error")
	}
	if err := tenantPub.PublishForDrivers(context.Background(), "topic", core.Event{}, nil); err == nil {
		t.Fatalf("expected error")
	}
	if err := tenantPub.Close(); err != nil {
		t.Fatalf("unexpected close error")
	}
}
