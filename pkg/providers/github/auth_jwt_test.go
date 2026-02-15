package github

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"strings"
	"testing"
)

func TestAppAuthenticatorJWT(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	block := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	}
	pemKey := pem.EncodeToMemory(block)
	authenticator := newAppAuthenticator(AppConfig{
		AppID:         123,
		PrivateKeyPEM: string(pemKey),
	})
	token, err := authenticator.jwt()
	if err != nil {
		t.Fatalf("jwt: %v", err)
	}
	if strings.Count(token, ".") != 2 {
		t.Fatalf("unexpected jwt format: %s", token)
	}
}
