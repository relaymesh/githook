package storage

import (
	"context"
	"strings"

	"gorm.io/gorm"
)

func ResolveTenant(ctx context.Context, explicit string) string {
	explicit = strings.TrimSpace(explicit)
	if explicit != "" {
		return explicit
	}
	return TenantFromContext(ctx)
}

func TenantScope(ctx context.Context, explicitTenant, column string) func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		tenantID := ResolveTenant(ctx, explicitTenant)
		if tenantID == "" {
			return db
		}
		col := strings.TrimSpace(column)
		if col == "" {
			col = "tenant_id"
		}
		return db.Where(col+" = ?", tenantID)
	}
}
