package core

import (
	"strings"
	"testing"
)

func TestPostgresPayloadMigrationQuery(t *testing.T) {
	query := postgresPayloadMigrationQuery(`"events"`)
	if query == "" {
		t.Fatalf("expected migration query")
	}
	if want := `table_name = 'events'`; !strings.Contains(query, want) {
		t.Fatalf("expected table name in query, got %q", query)
	}
}

func TestPostgresPayloadMigrationQueryEmpty(t *testing.T) {
	if query := postgresPayloadMigrationQuery(" "); query != "" {
		t.Fatalf("expected empty query, got %q", query)
	}
}

func TestEscapePostgresLiteral(t *testing.T) {
	if got := escapePostgresLiteral("a'b"); got != "a''b" {
		t.Fatalf("expected escaped literal, got %q", got)
	}
}

func TestSchemaInitializingQueries(t *testing.T) {
	schema := postgresBinarySchema{}
	queries := schema.SchemaInitializingQueries("events")
	if len(queries) == 0 {
		t.Fatalf("expected schema queries")
	}
	if !strings.Contains(queries[0], "CREATE TABLE IF NOT EXISTS") {
		t.Fatalf("expected create table query, got %q", queries[0])
	}
}
