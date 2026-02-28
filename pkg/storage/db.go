package storage

import (
	"errors"
	"fmt"
	"strings"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func NormalizeSQLDriver(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	switch value {
	case "postgres", "postgresql", "pgx":
		return "postgres"
	case "mysql":
		return "mysql"
	case "sqlite", "sqlite3":
		return "sqlite"
	default:
		return ""
	}
}

func ResolveSQLDriver(driver, dialect string) (string, error) {
	resolved := NormalizeSQLDriver(driver)
	if resolved == "" {
		resolved = NormalizeSQLDriver(dialect)
	}
	if resolved == "" {
		return "", errors.New("unsupported storage driver")
	}
	return resolved, nil
}

func OpenGorm(driver, dsn string) (*gorm.DB, error) {
	switch driver {
	case "postgres":
		return gorm.Open(postgres.Open(dsn), &gorm.Config{})
	case "mysql":
		return gorm.Open(mysql.Open(dsn), &gorm.Config{})
	case "sqlite":
		return gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	default:
		return nil, fmt.Errorf("unsupported storage driver: %s", driver)
	}
}
