package github

import (
	"encoding/base64"
	"encoding/json"
	"testing"
)

func TestInstallationIDFromPayloadHelpers(t *testing.T) {
	id, ok, err := InstallationIDFromPayload([]byte(`{"installation":{"id":42}}`))
	if err != nil || !ok || id != 42 {
		t.Fatalf("unexpected installation id: %v %v %v", id, ok, err)
	}
	id, ok, err = InstallationIDFromPayload([]byte(`{"installation":{}}`))
	if err != nil || ok || id != 0 {
		t.Fatalf("expected missing installation id")
	}
}

func TestEncodeSegment(t *testing.T) {
	encoded, err := encodeSegment(map[string]interface{}{"foo": "bar"})
	if err != nil {
		t.Fatalf("encode segment: %v", err)
	}
	raw, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatalf("decode segment: %v", err)
	}
	var payload map[string]string
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if payload["foo"] != "bar" {
		t.Fatalf("unexpected payload: %v", payload)
	}
}

func TestNormalizeBaseURL(t *testing.T) {
	if got := normalizeBaseURL(""); got != defaultBaseURL {
		t.Fatalf("expected default base url, got %q", got)
	}
	if got := normalizeBaseURL("https://github.example.com/"); got != "https://github.example.com" {
		t.Fatalf("unexpected base url: %q", got)
	}
}
