// Package db opens a GORM PostgreSQL connection with sane connection-pool
// defaults. Using the zero-value *gorm.Config leaves the database/sql pool
// unbounded, which can exhaust PostgreSQL's max_connections under load or
// during rolling pod restarts in Kubernetes.
package db

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// Defaults chosen for a single service replica talking to a shared Postgres.
// Multiply by replica count to estimate peak connections.
const (
	DefaultMaxOpenConns    = 25
	DefaultMaxIdleConns    = 5
	DefaultConnMaxLifetime = 5 * time.Minute
)

// Config holds pool tuning knobs. Zero values fall back to the defaults above.
type Config struct {
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	GormConfig      *gorm.Config
}

// Open opens a GORM PostgreSQL connection using the given DSN and applies the
// pool configuration. Pool values not set on cfg are read from the
// DB_MAX_OPEN_CONNS, DB_MAX_IDLE_CONNS, DB_CONN_MAX_LIFETIME env vars, then
// fall back to the package defaults.
func Open(dsn string, cfg Config) (*gorm.DB, error) {
	gormCfg := cfg.GormConfig
	if gormCfg == nil {
		gormCfg = &gorm.Config{}
	}
	db, err := gorm.Open(postgres.Open(dsn), gormCfg)
	if err != nil {
		return nil, fmt.Errorf("open gorm: %w", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("unwrap *sql.DB: %w", err)
	}

	sqlDB.SetMaxOpenConns(resolveInt(cfg.MaxOpenConns, "DB_MAX_OPEN_CONNS", DefaultMaxOpenConns))
	sqlDB.SetMaxIdleConns(resolveInt(cfg.MaxIdleConns, "DB_MAX_IDLE_CONNS", DefaultMaxIdleConns))
	sqlDB.SetConnMaxLifetime(resolveDuration(cfg.ConnMaxLifetime, "DB_CONN_MAX_LIFETIME", DefaultConnMaxLifetime))

	return db, nil
}

func resolveInt(explicit int, envKey string, fallback int) int {
	if explicit > 0 {
		return explicit
	}
	if v := os.Getenv(envKey); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return fallback
}

func resolveDuration(explicit time.Duration, envKey string, fallback time.Duration) time.Duration {
	if explicit > 0 {
		return explicit
	}
	if v := os.Getenv(envKey); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			return d
		}
	}
	return fallback
}
