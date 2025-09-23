package store

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_migrate "github.com/golang-migrate/migrate/v4/source/file"
)

// Migrator handles DB schema migrations using golang-migrate.
type Migrator struct {
	dsn string
}

func NewMigrator(dsn string) (*Migrator, error) {
	if dsn == "" {
		return nil, fmt.Errorf("missing DSN")
	}
	return &Migrator{dsn: dsn}, nil
}

func (m *Migrator) sourceURL() (string, error) {
	// Point to local db/migrations directory
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	p := filepath.Join(wd, "db", "migrations")
	u := url.URL{Scheme: "file", Path: p}
	return u.String(), nil
}

func (m *Migrator) Up(ctx context.Context) error {
	src, err := m.sourceURL()
	if err != nil {
		return err
	}
	mig, closer, err := m.migrateInstance(src)
	if err != nil {
		return err
	}
	defer closer()
	if err := mig.Up(); err != nil {
		if err == migrate.ErrNoChange {
			return ErrNoChange
		}
		return err
	}
	return nil
}

func (m *Migrator) Down(ctx context.Context) error {
	src, err := m.sourceURL()
	if err != nil {
		return err
	}
	mig, closer, err := m.migrateInstance(src)
	if err != nil {
		return err
	}
	defer closer()
	if err := mig.Steps(-1); err != nil {
		if err == migrate.ErrNoChange {
			return ErrNoChange
		}
		return err
	}
	return nil
}

func (m *Migrator) migrateInstance(src string) (*migrate.Migrate, func(), error) {
	_ = _migrate.Asset{} // keep file driver
	// Ensure postgres driver is linked
	if _, err := postgres.WithInstance(nil, &postgres.Config{}); err != nil {
		// ignore actual nil DB error, this is only to link package
	}
	mig, err := migrate.New(src, m.dsn)
	if err != nil {
		return nil, func() {}, err
	}
	return mig, func() { mig.Close() }, nil
}
