package oauth

import "testing"

func TestEncodeDecodeState(t *testing.T) {
	encoded := encodeState("raw-state", "tenant-1", "instance-1")
	if encoded == "raw-state" {
		t.Fatalf("expected encoded state to differ from raw state")
	}
	decoded := decodeState(encoded)
	if decoded.State != "raw-state" {
		t.Fatalf("expected state to round-trip, got %q", decoded.State)
	}
	if decoded.TenantID != "tenant-1" || decoded.InstanceKey != "instance-1" {
		t.Fatalf("unexpected decoded payload: %+v", decoded)
	}
}

func TestEncodeStateNoTenantOrInstance(t *testing.T) {
	encoded := encodeState("raw", "", "")
	if encoded != "raw" {
		t.Fatalf("expected raw state to be returned, got %q", encoded)
	}
}

func TestDecodeStateFallback(t *testing.T) {
	decoded := decodeState("not-base64")
	if decoded.State != "not-base64" {
		t.Fatalf("expected raw state fallback, got %q", decoded.State)
	}
	if decoded.TenantID != "" || decoded.InstanceKey != "" {
		t.Fatalf("expected empty tenant/instance, got %+v", decoded)
	}
}
