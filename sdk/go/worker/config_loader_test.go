package worker

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadTopicsFromConfigEmitList(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.yaml")
	content := `
rules:
  - when: action == "opened"
    emit: [pr.opened, audit.pr.opened]
  - when: action == "closed"
    emit: pr.merged
  - when: action == "closed"
    emit: [pr.merged, audit.pr.merged]
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	topics, err := LoadTopicsFromConfig(path)
	if err != nil {
		t.Fatalf("load topics: %v", err)
	}

	want := map[string]bool{
		"pr.opened":       false,
		"audit.pr.opened": false,
		"pr.merged":       false,
		"audit.pr.merged": false,
	}

	if len(topics) != len(want) {
		t.Fatalf("expected %d topics, got %d: %v", len(want), len(topics), topics)
	}
	for _, topic := range topics {
		if _, ok := want[topic]; !ok {
			t.Fatalf("unexpected topic %q", topic)
		}
		want[topic] = true
	}
	for topic, seen := range want {
		if !seen {
			t.Fatalf("missing topic %q", topic)
		}
	}
}
