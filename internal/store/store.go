package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/DaanHessen/walker-tui/internal/util"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var ErrNoChange = errors.New("no change")

// DB wraps gorm.DB for repositories and exposes Close.
type DB struct {
	gorm *gorm.DB
	sql  *sql.DB
}

func (d *DB) Close() error { return d.sql.Close() }
func (d *DB) Gorm() *gorm.DB { return d.gorm }

// Open connects to DB per config.
func Open(ctx context.Context, cfg util.Config) (*DB, error) {
	var (
		gdb *gorm.DB
		err error
	)
	if cfg.DSN == "" {
		return nil, fmt.Errorf("missing DSN")
	}
	// Postgres-only
	gdb, err = gorm.Open(postgres.Open(cfg.DSN), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	sdb, err := gdb.DB()
	if err != nil {
		return nil, err
	}
	sdb.SetConnMaxLifetime(30 * time.Minute)
	sdb.SetMaxOpenConns(10)
	sdb.SetMaxIdleConns(5)
	if err := sdb.PingContext(ctx); err != nil {
		return nil, err
	}
	return &DB{gorm: gdb, sql: sdb}, nil
}
