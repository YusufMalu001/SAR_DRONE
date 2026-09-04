// Package migrate applies the embedded SQL migrations (see
// internal/migrations) using golang-migrate, so schema changes ship with
// the binary instead of relying on out-of-band `psql -f` runs or the
// Postgres image's initdb.d bootstrap.
package migrate

import (
	"embed"
	"errors"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

// Run applies all pending migrations in fs to databaseURL. Safe to call on
// every startup — a no-op once the schema is already current.
func Run(databaseURL string, fs embed.FS) error {
	source, err := iofs.New(fs, ".")
	if err != nil {
		return fmt.Errorf("open embedded migrations: %w", err)
	}

	m, err := migrate.NewWithSourceInstance("iofs", source, databaseURL)
	if err != nil {
		return fmt.Errorf("init migrator: %w", err)
	}
	defer m.Close()

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("apply migrations: %w", err)
	}
	return nil
}
