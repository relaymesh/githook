package worker

import "testing"

func TestRiverQueueMetadata(t *testing.T) {
	if _, _, _, _, err := riverQueueMetadata(nil); err == nil {
		t.Fatalf("expected missing metadata error")
	}
	if _, _, _, _, err := riverQueueMetadata([]byte(`invalid`)); err == nil {
		t.Fatalf("expected decode error")
	}
	if _, _, _, _, err := riverQueueMetadata([]byte(`{"provider":"github"}`)); err == nil {
		t.Fatalf("expected missing topic error")
	}
	provider, name, topic, logID, err := riverQueueMetadata([]byte(`{"provider":"github","event":"push","topic":"t","log_id":"l"}`))
	if err != nil {
		t.Fatalf("expected metadata, got %v", err)
	}
	if provider != "github" || name != "push" || topic != "t" || logID != "l" {
		t.Fatalf("unexpected metadata values")
	}
}

func TestRiverQueueSchemaFromTable(t *testing.T) {
	if schema, err := riverQueueSchemaFromTable(""); err != nil || schema != "" {
		t.Fatalf("expected empty schema")
	}
	if schema, err := riverQueueSchemaFromTable("river_job"); err != nil || schema != "" {
		t.Fatalf("expected default table schema")
	}
	if schema, err := riverQueueSchemaFromTable("public.river_job"); err != nil || schema != "public" {
		t.Fatalf("expected schema name, got %q", schema)
	}
	if _, err := riverQueueSchemaFromTable("bad.table"); err == nil {
		t.Fatalf("expected unsupported table error")
	}
}
