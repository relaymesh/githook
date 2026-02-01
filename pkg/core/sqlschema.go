package core

import (
	"fmt"
	"strings"

	wmsql "github.com/ThreeDotsLabs/watermill-sql/pkg/sql"
)

type postgresBinarySchema struct {
	wmsql.DefaultPostgreSQLSchema
}

func (s postgresBinarySchema) SchemaInitializingQueries(topic string) []string {
	table := s.MessagesTable(topic)
	createMessagesTable := strings.Join([]string{
		`CREATE TABLE IF NOT EXISTS ` + table + ` (`,
		`"offset" SERIAL,`,
		`"uuid" VARCHAR(36) NOT NULL,`,
		`"created_at" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,`,
		`"payload" BYTEA DEFAULT NULL,`,
		`"metadata" JSON DEFAULT NULL`,
		`);`,
	}, "\n")

	migratePayload := postgresPayloadMigrationQuery(table)
	if migratePayload == "" {
		return []string{createMessagesTable}
	}
	return []string{createMessagesTable, migratePayload}
}

func postgresPayloadMigrationQuery(table string) string {
	tableName := strings.ReplaceAll(strings.TrimSpace(table), `"`, "")
	if tableName == "" {
		return ""
	}
	return fmt.Sprintf(`
DO $$
BEGIN
  IF EXISTS (
    SELECT 1
    FROM information_schema.columns
    WHERE table_schema = current_schema()
      AND table_name = '%s'
      AND column_name = 'payload'
      AND data_type <> 'bytea'
  ) THEN
    EXECUTE 'ALTER TABLE %s ALTER COLUMN "payload" TYPE BYTEA USING convert_to("payload"::text, ''UTF8'')';
  END IF;
END $$;`, escapePostgresLiteral(tableName), table)
}

func escapePostgresLiteral(value string) string {
	return strings.ReplaceAll(value, `'`, `''`)
}
