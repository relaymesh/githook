package core

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/lib/pq"
)

// riverQueuePublisher is a publisher that sends events to a RiverQueue job queue.
type riverQueuePublisher struct {
	db  *sql.DB
	cfg RiverQueueConfig
}

// newRiverQueuePublisher creates a new RiverQueue publisher.
func newRiverQueuePublisher(cfg RiverQueueConfig) (*riverQueuePublisher, error) {
	driver := cfg.Driver
	if driver == "" {
		driver = "postgres"
	}
	if cfg.DSN == "" {
		return nil, fmt.Errorf("riverqueue dsn is required")
	}
	db, err := sql.Open(driver, cfg.DSN)
	if err != nil {
		return nil, err
	}
	return &riverQueuePublisher{db: db, cfg: cfg}, nil
}

// Publish inserts a new job into the RiverQueue jobs table.
func (p *riverQueuePublisher) Publish(ctx context.Context, topic string, event Event) error {
	argsPayload := event.RawPayload
	if len(argsPayload) == 0 {
		encoded, err := json.Marshal(event)
		if err != nil {
			return err
		}
		argsPayload = encoded
	}

	metadata := map[string]interface{}{
		"provider": event.Provider,
		"name":     event.Name,
		"topic":    topic,
	}
	if event.LogID != "" {
		metadata["log_id"] = event.LogID
	}
	if event.RequestID != "" {
		metadata["request_id"] = event.RequestID
	}
	if event.StateID != "" {
		metadata["state_id"] = event.StateID
	}
	if event.InstallationID != "" {
		metadata["installation_id"] = event.InstallationID
	}
	metadataPayload, err := json.Marshal(metadata)
	if err != nil {
		return err
	}

	table := p.cfg.Table
	if table == "" {
		table = "river_job"
	}
	table = strings.TrimSpace(table)
	if table == "" {
		table = "river_job"
	}

	query := fmt.Sprintf(
		`INSERT INTO %s (args, kind, max_attempts, metadata, priority, queue, scheduled_at, tags)
VALUES ($1, $2, $3, $4, $5, $6, now(), $7)`,
		table,
	)

	_, err = p.db.ExecContext(
		ctx,
		query,
		string(argsPayload),
		p.cfg.Kind,
		p.cfg.MaxAttempts,
		string(metadataPayload),
		p.cfg.Priority,
		p.cfg.Queue,
		pq.Array(p.cfg.Tags),
	)
	return err
}

// Close closes the underlying database connection.
func (p *riverQueuePublisher) Close() error {
	if p.db == nil {
		return nil
	}
	return p.db.Close()
}

// PublishForDrivers is a convenience method that calls Publish.
func (p *riverQueuePublisher) PublishForDrivers(ctx context.Context, topic string, event Event, drivers []string) error {
	return p.Publish(ctx, topic, event)
}
