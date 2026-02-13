package github

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

const defaultBaseURL = "https://api.github.com"

// AppConfig contains GitHub App authentication settings.
type AppConfig struct {
	AppID          int64
	PrivateKeyPath string
	PrivateKeyPEM  string
	BaseURL        string
}

// InstallationIDFromPayload extracts the GitHub App installation ID.
func InstallationIDFromPayload(payload []byte) (int64, bool, error) {
	var raw struct {
		Installation struct {
			ID int64 `json:"id"`
		} `json:"installation"`
	}
	if err := json.Unmarshal(payload, &raw); err != nil {
		return 0, false, err
	}
	if raw.Installation.ID == 0 {
		return 0, false, nil
	}
	return raw.Installation.ID, true, nil
}

type appAuthenticator struct {
	appID    int64
	keyPath  string
	keyPEM   string
	baseURL  string
	client   *http.Client
	keyOnce  sync.Once
	key      *rsa.PrivateKey
	keyError error
}

func newAppAuthenticator(cfg AppConfig) *appAuthenticator {
	return &appAuthenticator{
		appID:   cfg.AppID,
		keyPath: cfg.PrivateKeyPath,
		keyPEM:  cfg.PrivateKeyPEM,
		baseURL: normalizeBaseURL(cfg.BaseURL),
		client:  &http.Client{Timeout: 10 * time.Second},
	}
}

// InstallationAccount contains the account identity for a GitHub App installation.
type InstallationAccount struct {
	ID   string
	Name string
	Type string
}

// FetchInstallationAccount fetches the account identity for a GitHub App installation.
func FetchInstallationAccount(ctx context.Context, cfg AppConfig, installationID int64) (InstallationAccount, error) {
	if installationID == 0 {
		return InstallationAccount{}, errors.New("installation id is required")
	}
	authenticator := newAppAuthenticator(cfg)
	jwt, err := authenticator.jwt()
	if err != nil {
		return InstallationAccount{}, err
	}
	endpoint := fmt.Sprintf("%s/app/installations/%d", authenticator.baseURL, installationID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return InstallationAccount{}, err
	}
	req.Header.Set("Authorization", "Bearer "+jwt)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := authenticator.client.Do(req)
	if err != nil {
		return InstallationAccount{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return InstallationAccount{}, fmt.Errorf("github installation lookup failed: %s", strings.TrimSpace(string(body)))
	}
	var payload struct {
		Account struct {
			ID    int64  `json:"id"`
			Login string `json:"login"`
			Type  string `json:"type"`
		} `json:"account"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return InstallationAccount{}, err
	}
	if payload.Account.ID == 0 {
		return InstallationAccount{}, errors.New("github installation account missing")
	}
	return InstallationAccount{
		ID:   fmt.Sprintf("%d", payload.Account.ID),
		Name: payload.Account.Login,
		Type: payload.Account.Type,
	}, nil
}

func (a *appAuthenticator) installationToken(ctx context.Context, installationID int64) (string, error) {
	jwt, err := a.jwt()
	if err != nil {
		return "", err
	}

	endpoint := fmt.Sprintf("%s/app/installations/%d/access_tokens", a.baseURL, installationID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+jwt)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := a.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("github token exchange failed: %s", strings.TrimSpace(string(body)))
	}

	var out struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	if out.Token == "" {
		return "", errors.New("github installation token missing from response")
	}
	return out.Token, nil
}

func (a *appAuthenticator) jwt() (string, error) {
	key, err := a.privateKey()
	if err != nil {
		return "", err
	}
	now := time.Now().UTC()
	claims := map[string]interface{}{
		"iat": now.Add(-30 * time.Second).Unix(),
		"exp": now.Add(9 * time.Minute).Unix(),
		"iss": a.appID,
	}
	header := map[string]interface{}{
		"alg": "RS256",
		"typ": "JWT",
	}
	encodedHeader, err := encodeSegment(header)
	if err != nil {
		return "", err
	}
	encodedClaims, err := encodeSegment(claims)
	if err != nil {
		return "", err
	}
	unsigned := encodedHeader + "." + encodedClaims
	hash := sha256.Sum256([]byte(unsigned))
	signature, err := rsa.SignPKCS1v15(nil, key, crypto.SHA256, hash[:])
	if err != nil {
		return "", err
	}
	encodedSig := base64.RawURLEncoding.EncodeToString(signature)
	return unsigned + "." + encodedSig, nil
}

func (a *appAuthenticator) privateKey() (*rsa.PrivateKey, error) {
	a.keyOnce.Do(func() {
		var keyBytes []byte
		if strings.TrimSpace(a.keyPEM) != "" {
			keyBytes = []byte(a.keyPEM)
		} else {
			var readErr error
			keyBytes, readErr = os.ReadFile(a.keyPath)
			if readErr != nil {
				a.keyError = readErr
				return
			}
		}
		block, _ := pem.Decode(keyBytes)
		if block == nil {
			a.keyError = errors.New("github private key PEM decode failed")
			return
		}
		if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
			a.key = key
			return
		}
		parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			a.keyError = err
			return
		}
		typed, ok := parsed.(*rsa.PrivateKey)
		if !ok {
			a.keyError = errors.New("github private key is not RSA")
			return
		}
		a.key = typed
	})
	if a.keyError != nil {
		return nil, a.keyError
	}
	if a.key == nil {
		return nil, errors.New("github private key not loaded")
	}
	return a.key, nil
}

func encodeSegment(data map[string]interface{}) (string, error) {
	raw, err := json.Marshal(data)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func normalizeBaseURL(base string) string {
	base = strings.TrimSpace(base)
	if base == "" {
		return defaultBaseURL
	}
	return strings.TrimRight(base, "/")
}
