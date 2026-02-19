package eventlogs

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"githook/pkg/storage"
)

// CreateEventLogs inserts event log records.
func (s *Store) CreateEventLogs(ctx context.Context, records []storage.EventLogRecord) error {
	if s == nil || s.db == nil {
		return errors.New("store is not initialized")
	}
	if len(records) == 0 {
		return nil
	}
	now := time.Now().UTC()
	rows := make([]row, 0, len(records))
	for _, record := range records {
		if record.TenantID == "" {
			record.TenantID = storage.TenantFromContext(ctx)
		}
		if record.ID == "" {
			record.ID = uuid.NewString()
		}
		if record.Status == "" {
			record.Status = "queued"
		}
		if record.CreatedAt.IsZero() {
			record.CreatedAt = now
		}
		if record.UpdatedAt.IsZero() {
			record.UpdatedAt = record.CreatedAt
		}
		if record.LatencyMS < 0 {
			record.LatencyMS = 0
		}
		data, err := toRow(record)
		if err != nil {
			return err
		}
		rows = append(rows, data)
	}
	return s.tableDB().WithContext(ctx).Create(&rows).Error
}

// UpdateEventLogStatus updates the status and error message of an event log.
func (s *Store) UpdateEventLogStatus(ctx context.Context, id, status, errorMessage string) error {
	if s == nil || s.db == nil {
		return errors.New("store is not initialized")
	}
	id = strings.TrimSpace(id)
	status = strings.TrimSpace(status)
	if id == "" {
		return errors.New("id is required")
	}
	if status == "" {
		return errors.New("status is required")
	}
	tenantID := storage.TenantFromContext(ctx)
	query := s.tableDB().WithContext(ctx).Where("id = ?", id)
	if tenantID != "" {
		query = query.Where("tenant_id = ?", tenantID)
	}
	updates := map[string]interface{}{
		"status":        status,
		"error_message": strings.TrimSpace(errorMessage),
		"updated_at":    time.Now().UTC(),
	}
	if status == "success" || status == "failed" {
		var existing row
		if err := query.Select("created_at").First(&existing).Error; err == nil && !existing.CreatedAt.IsZero() {
			updates["latency_ms"] = time.Since(existing.CreatedAt).Milliseconds()
		}
	}
	return query.Updates(updates).Error
}
