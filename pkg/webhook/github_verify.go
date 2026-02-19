package webhook

import (
	"crypto/hmac"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"hash"
	"strings"
)

func verifyGitHubSHA1(secret string, body []byte, signature string) bool {
	return verifyGitHubSignature(secret, body, signature)
}

func verifyGitHubSignature(secret string, body []byte, signature string) bool {
	if secret == "" || len(body) == 0 || signature == "" {
		return false
	}
	value := strings.TrimSpace(signature)
	var hashFunc func() hash.Hash
	var prefix string
	switch {
	case strings.HasPrefix(value, "sha256="):
		prefix = "sha256="
		hashFunc = sha256.New
	case strings.HasPrefix(value, "sha1="):
		prefix = "sha1="
		hashFunc = sha1.New
	default:
		return false
	}
	value = strings.TrimPrefix(value, prefix)
	mac := hmac.New(hashFunc, []byte(secret))
	_, _ = mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(value), []byte(expected))
}
