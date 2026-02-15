package github

import "testing"

func TestEnterpriseUploadURL(t *testing.T) {
	if got := enterpriseUploadURL("https://github.example.com/api/v3"); got != "https://github.example.com/api/uploads" {
		t.Fatalf("unexpected upload url: %q", got)
	}
	if got := enterpriseUploadURL("https://github.example.com/api"); got != "https://github.example.com/api/uploads" {
		t.Fatalf("unexpected upload url: %q", got)
	}
	if got := enterpriseUploadURL("https://github.example.com"); got != "https://github.example.com" {
		t.Fatalf("unexpected upload url: %q", got)
	}
}
