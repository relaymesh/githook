package drivers

import "testing"

func TestDynamicPublisherCacheBasics(t *testing.T) {
	t.Run("new cache initialized", func(t *testing.T) {
		cache := NewDynamicPublisherCache()
		if cache == nil {
			t.Fatalf("expected cache instance")
		}
		if len(cache.pubs) != 0 {
			t.Fatalf("expected empty cache")
		}
	})

	t.Run("publisher requires driver name", func(t *testing.T) {
		cache := NewDynamicPublisherCache()
		if _, err := cache.Publisher("", "{}"); err == nil {
			t.Fatalf("expected driver name error")
		}
	})

	t.Run("publisher rejects unsupported driver", func(t *testing.T) {
		cache := NewDynamicPublisherCache()
		if _, err := cache.Publisher("unknown", "{}"); err == nil {
			t.Fatalf("expected unsupported driver error")
		}
	})
}

func TestDynamicPublisherCacheCloseAndCacheKey(t *testing.T) {
	if err := (*DynamicPublisherCache)(nil).Close(); err != nil {
		t.Fatalf("nil cache close should not error: %v", err)
	}

	cache := NewDynamicPublisherCache()
	if err := cache.Close(); err != nil {
		t.Fatalf("empty cache close should not error: %v", err)
	}

	k1 := cacheKey("AMQP", " {\"url\":\"a\"} ")
	k2 := cacheKey("amqp", "{\"url\":\"a\"}")
	if k1 != k2 {
		t.Fatalf("expected normalized cache keys to match")
	}

	k3 := cacheKey("amqp", "{\"url\":\"b\"}")
	if k1 == k3 {
		t.Fatalf("expected distinct cache keys for different config")
	}
}

func TestDynamicPublisherCachePublisherFromCacheAndCloseError(t *testing.T) {
	cache := NewDynamicPublisherCache()
	key := cacheKey("amqp", "{}")
	p := &stubPublisher{err: assertErr("close failed")}
	cache.pubs[key] = p

	pub, err := cache.Publisher("amqp", "{}")
	if err != nil {
		t.Fatalf("publisher from cache: %v", err)
	}
	if pub != p {
		t.Fatalf("expected cached publisher")
	}

	if err := cache.Close(); err == nil {
		t.Fatalf("expected close error from cached publisher")
	}
	if len(cache.pubs) != 0 {
		t.Fatalf("expected cache cleared after close")
	}
}

type assertErr string

func (e assertErr) Error() string {
	return string(e)
}
