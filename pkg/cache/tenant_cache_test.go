package cache

import "testing"

func TestTenantCacheSetGetDelete(t *testing.T) {
	cache := NewTenantCache[int]()
	cache.Set("tenant-a", 42)

	value, ok := cache.Get("tenant-a")
	if !ok || value != 42 {
		t.Fatalf("expected cached value 42, got %v %v", value, ok)
	}

	cache.Delete("tenant-a")
	if _, ok := cache.Get("tenant-a"); ok {
		t.Fatalf("expected value to be deleted")
	}
}

func TestTenantCacheRange(t *testing.T) {
	cache := NewTenantCache[string]()
	cache.Set("t1", "a")
	cache.Set("t2", "b")

	seen := map[string]string{}
	cache.Range(func(key, value string) {
		seen[key] = value
	})
	if len(seen) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(seen))
	}
	if seen["t1"] != "a" || seen["t2"] != "b" {
		t.Fatalf("unexpected values: %v", seen)
	}
}
