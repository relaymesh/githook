package server

import (
	"io"
	"log"
	"net/http"
	"testing"
)

func TestBearerToken(t *testing.T) {
	header := http.Header{}
	if _, err := bearerToken(header); err == nil || err.Error() != "missing authorization header" {
		t.Fatalf("expected missing authorization header error, got %v", err)
	}

	header.Set("Authorization", "Basic abc")
	if _, err := bearerToken(header); err == nil || err.Error() != "invalid authorization header" {
		t.Fatalf("expected invalid authorization header error, got %v", err)
	}

	header.Set("Authorization", "Bearer   ")
	if _, err := bearerToken(header); err == nil || err.Error() != "invalid authorization header" {
		t.Fatalf("expected invalid authorization header error, got %v", err)
	}

	header.Set("Authorization", "Bearer token-value")
	token, err := bearerToken(header)
	if err != nil {
		t.Fatalf("expected token, got error %v", err)
	}
	if token != "token-value" {
		t.Fatalf("expected token-value, got %q", token)
	}
}

func TestLogAuthError(t *testing.T) {
	logAuthError(nil, nil)
	logAuthError(log.New(io.Discard, "", 0), nil)
	logAuthError(log.New(io.Discard, "", 0), errTestSentinel{})
}

type errTestSentinel struct{}

func (errTestSentinel) Error() string { return "sentinel" }
