package eventlogs

import (
	"context"
	"errors"

	"githook/pkg/storage"
)

// ListEventLogs returns event logs matching the filter.
func (s *Store) ListEventLogs(ctx context.Context, filter storage.EventLogFilter) ([]storage.EventLogRecord, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("store is not initialized")
	}
	query := applyFilter(s.tableDB().WithContext(ctx), filter, ctx).
		Order("created_at desc")
	if filter.Limit > 0 {
		query = query.Limit(filter.Limit)
	}
	if filter.Offset > 0 {
		query = query.Offset(filter.Offset)
	}
	var data []row
	if err := query.Find(&data).Error; err != nil {
		return nil, err
	}
	out := make([]storage.EventLogRecord, 0, len(data))
	for _, item := range data {
		out = append(out, fromRow(item))
	}
	return out, nil
}
