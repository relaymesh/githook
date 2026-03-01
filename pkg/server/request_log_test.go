package server

import (
	"bytes"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRequestLogMiddleware(t *testing.T) {
	var buf bytes.Buffer
	logger := log.New(&buf, "", 0)
	mw := requestLogMiddleware(logger)
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Test", "ok")
		_, _ = w.Write([]byte("hello"))
	}))
	req := httptest.NewRequest(http.MethodPost, "/cloud.v1.DriversService/UpsertDriver", strings.NewReader("{}"))
	req.RemoteAddr = "127.0.0.1:9999"
	req.Header.Set("User-Agent", "test-agent")
	req.Header.Set("X-Tenant-ID", "tenant-a")
	req.Header.Set("X-Request-Id", "req-1")
	res := httptest.NewRecorder()
	h.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.Code)
	}
	line := buf.String()
	checks := []string{
		"method=POST",
		"path=/cloud.v1.DriversService/UpsertDriver",
		"status=200",
		"tenant=tenant-a",
		"request_id=req-1",
		"remote_ip=127.0.0.1",
		`ua="test-agent"`,
	}
	for _, want := range checks {
		if !strings.Contains(line, want) {
			t.Fatalf("expected log to contain %q, got %q", want, line)
		}
	}
}
