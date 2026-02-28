package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestApplyMiddlewares(t *testing.T) {
	if got := applyMiddlewares(nil, nil); got != nil {
		t.Fatalf("expected nil handler for nil input")
	}

	order := make([]string, 0, 4)
	base := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		order = append(order, "handler")
		w.WriteHeader(http.StatusNoContent)
	})

	m1 := Middleware(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			order = append(order, "m1-before")
			next.ServeHTTP(w, r)
			order = append(order, "m1-after")
		})
	})
	m2 := Middleware(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			order = append(order, "m2-before")
			next.ServeHTTP(w, r)
			order = append(order, "m2-after")
		})
	})

	wrapped := applyMiddlewares(base, []Middleware{m1, nil, m2})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	res := httptest.NewRecorder()
	wrapped.ServeHTTP(res, req)

	if res.Code != http.StatusNoContent {
		t.Fatalf("unexpected status: %d", res.Code)
	}
	got := order
	want := []string{"m1-before", "m2-before", "handler", "m2-after", "m1-after"}
	if len(got) != len(want) {
		t.Fatalf("unexpected middleware order: %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("unexpected middleware order: %v", got)
		}
	}
}
