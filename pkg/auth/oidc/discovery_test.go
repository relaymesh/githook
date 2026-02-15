package oidc

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"sync/atomic"
	"testing"
)

func TestDiscoverCachesResponses(t *testing.T) {
	discoveryCacheMu.Lock()
	discoveryCache = make(map[string]cacheEntry)
	discoveryCacheMu.Unlock()

	var hits int32
	payload := Discovery{
		AuthorizationEndpoint: "https://issuer/auth",
		TokenEndpoint:         "https://issuer/token",
		JWKSURI:               "https://issuer/jwks",
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	issuer := "https://issuer.example.com"
	originalTransport := http.DefaultTransport
	http.DefaultTransport = roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		atomic.AddInt32(&hits, 1)
		if req.URL.String() != issuer+"/.well-known/openid-configuration" {
			t.Fatalf("unexpected url: %s", req.URL.String())
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     http.StatusText(http.StatusOK),
			Header:     make(http.Header),
			Body:       io.NopCloser(bytes.NewReader(raw)),
		}, nil
	})
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	first, err := Discover(context.Background(), issuer)
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	second, err := Discover(context.Background(), issuer)
	if err != nil {
		t.Fatalf("discover second: %v", err)
	}
	if first != second {
		t.Fatalf("expected cached discovery, got %+v and %+v", first, second)
	}
	if atomic.LoadInt32(&hits) != 1 {
		t.Fatalf("expected cached response, hits=%d", hits)
	}
}

func TestDiscoverRequiresIssuer(t *testing.T) {
	if _, err := Discover(context.Background(), " "); err == nil {
		t.Fatalf("expected issuer required error")
	}
}
