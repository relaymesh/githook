package github

import "testing"

func TestInstallationIDFromPayload(t *testing.T) {
	id, ok, err := InstallationIDFromPayload([]byte(`{"installation":{"id":99}}`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !ok || id != 99 {
		t.Fatalf("expected installation id 99")
	}
}

func TestInstallationIDFromPayloadMissing(t *testing.T) {
	id, ok, err := InstallationIDFromPayload([]byte(`{"installation":{}}`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if ok || id != 0 {
		t.Fatalf("expected no installation id")
	}
}

func TestInstallationIDFromPayloadInvalid(t *testing.T) {
	_, _, err := InstallationIDFromPayload([]byte(`{`))
	if err == nil {
		t.Fatalf("expected error for invalid JSON")
	}
}
