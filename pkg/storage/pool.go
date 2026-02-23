package storage

import (
	"time"

	"gorm.io/gorm"
)

// PoolConfig controls database connection pooling.
type PoolConfig struct {
	MaxOpenConns      int
	MaxIdleConns      int
	ConnMaxLifetimeMS int64
	ConnMaxIdleTimeMS int64
}

// ApplyPoolConfig applies connection pool limits to a GORM database handle.
func ApplyPoolConfig(db *gorm.DB, cfg PoolConfig) error {
	if db == nil {
		return nil
	}
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	if cfg.MaxOpenConns > 0 {
		sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	}
	if cfg.MaxIdleConns > 0 {
		sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	}
	if cfg.ConnMaxLifetimeMS > 0 {
		sqlDB.SetConnMaxLifetime(time.Duration(cfg.ConnMaxLifetimeMS) * time.Millisecond)
	}
	if cfg.ConnMaxIdleTimeMS > 0 {
		sqlDB.SetConnMaxIdleTime(time.Duration(cfg.ConnMaxIdleTimeMS) * time.Millisecond)
	}
	return nil
}
