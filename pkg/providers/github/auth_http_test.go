package github

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func testRSAPrivateKeyPEM(t *testing.T) string {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	block := &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}
	return string(pem.EncodeToMemory(block))
}

func TestFetchInstallationAccountAndTokenValidation(t *testing.T) {
	if _, err := FetchInstallationAccount(context.Background(), AppConfig{}, 0); err == nil {
		t.Fatalf("expected installation id required error")
	}
	if _, err := FetchInstallationToken(context.Background(), AppConfig{}, 0); err == nil {
		t.Fatalf("expected installation id required error")
	}
}

func TestFetchInstallationAccountSuccessAndErrors(t *testing.T) {
	privateKey := testRSAPrivateKeyPEM(t)

	t.Run("success", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				t.Fatalf("unexpected method: %s", r.Method)
			}
			if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
				t.Fatalf("missing bearer auth")
			}
			_, _ = w.Write([]byte(`{"account":{"id":101,"login":"org","type":"Organization"}}`))
		}))
		defer srv.Close()

		got, err := FetchInstallationAccount(context.Background(), AppConfig{AppID: 1, PrivateKeyPEM: privateKey, BaseURL: srv.URL}, 10)
		if err != nil {
			t.Fatalf("fetch account: %v", err)
		}
		if got.ID != "101" || got.Name != "org" || got.Type != "Organization" {
			t.Fatalf("unexpected account: %+v", got)
		}
	})

	t.Run("http error body", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "denied", http.StatusForbidden)
		}))
		defer srv.Close()

		_, err := FetchInstallationAccount(context.Background(), AppConfig{AppID: 1, PrivateKeyPEM: privateKey, BaseURL: srv.URL}, 10)
		if err == nil || !strings.Contains(err.Error(), "denied") {
			t.Fatalf("expected denied error, got: %v", err)
		}
	})

	t.Run("missing account id", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"account":{"id":0}}`))
		}))
		defer srv.Close()

		_, err := FetchInstallationAccount(context.Background(), AppConfig{AppID: 1, PrivateKeyPEM: privateKey, BaseURL: srv.URL}, 10)
		if err == nil || !strings.Contains(err.Error(), "missing") {
			t.Fatalf("expected missing account error, got: %v", err)
		}
	})
}

func TestFetchInstallationTokenSuccessAndErrors(t *testing.T) {
	privateKey := testRSAPrivateKeyPEM(t)

	t.Run("success with expiry", func(t *testing.T) {
		expires := time.Now().UTC().Add(10 * time.Minute).Format(time.RFC3339)
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Fatalf("unexpected method: %s", r.Method)
			}
			_, _ = w.Write([]byte(`{"token":"tok-1","expires_at":"` + expires + `"}`))
		}))
		defer srv.Close()

		got, err := FetchInstallationToken(context.Background(), AppConfig{AppID: 1, PrivateKeyPEM: privateKey, BaseURL: srv.URL}, 10)
		if err != nil {
			t.Fatalf("fetch token: %v", err)
		}
		if got.Token != "tok-1" || got.ExpiresAt == nil {
			t.Fatalf("unexpected token response: %+v", got)
		}
	})

	t.Run("missing token", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"token":""}`))
		}))
		defer srv.Close()

		_, err := FetchInstallationToken(context.Background(), AppConfig{AppID: 1, PrivateKeyPEM: privateKey, BaseURL: srv.URL}, 10)
		if err == nil || !strings.Contains(err.Error(), "token missing") {
			t.Fatalf("expected token missing error, got: %v", err)
		}
	})

	t.Run("exchange http error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "exchange failed", http.StatusBadGateway)
		}))
		defer srv.Close()

		_, err := FetchInstallationToken(context.Background(), AppConfig{AppID: 1, PrivateKeyPEM: privateKey, BaseURL: srv.URL}, 10)
		if err == nil || !strings.Contains(err.Error(), "exchange failed") {
			t.Fatalf("expected exchange error, got: %v", err)
		}
	})

	t.Run("invalid expires format tolerated", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"token":"tok-2","expires_at":"not-a-time"}`))
		}))
		defer srv.Close()

		got, err := FetchInstallationToken(context.Background(), AppConfig{AppID: 1, PrivateKeyPEM: privateKey, BaseURL: srv.URL}, 10)
		if err != nil {
			t.Fatalf("fetch token with invalid expiry: %v", err)
		}
		if got.Token != "tok-2" || got.ExpiresAt != nil {
			t.Fatalf("expected nil expiry, got: %+v", got)
		}
	})
}

func TestAppAuthenticatorPrivateKeyFailures(t *testing.T) {
	a := newAppAuthenticator(AppConfig{AppID: 1, PrivateKeyPath: "/definitely/missing.pem"})
	if _, err := a.privateKey(); err == nil {
		t.Fatalf("expected missing key file error")
	}

	b := newAppAuthenticator(AppConfig{AppID: 1, PrivateKeyPEM: "not-pem"})
	if _, err := b.privateKey(); err == nil {
		t.Fatalf("expected pem decode error")
	}
}
