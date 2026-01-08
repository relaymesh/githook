package worker

import (
	"context"
	"testing"
)

func TestResolveInstallationMissingID(t *testing.T) {
	evt := &Event{
		Provider: "gitlab",
		Metadata: map[string]string{},
	}
	_, err := ResolveInstallation(context.Background(), evt, &InstallationsClient{BaseURL: "http://localhost:8080"})
	if err == nil {
		t.Fatalf("expected error for missing installation_id")
	}
}
