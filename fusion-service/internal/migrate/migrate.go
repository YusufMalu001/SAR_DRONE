// Package migrate applies the embedded SQL migrations (see
// internal/migrations) using golang-migrate, so schema changes ship with
// the binary instead of relying on out-of-band `psql -f` runs or the
// Postgres image's initdb.d bootstrap.
package migrate

import (
	"embed"
	"errors"
	"fmt"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

// Run applies all pending migrations in fs to databaseURL. Safe to call on
// every startup — a no-op once the schema is already current.
// Retries if the database is not ready yet during container startup.
func Run(databaseURL string, fs embed.FS) error {
	source, err := iofs.New(fs, ".")
	if err != nil {
		return fmt.Errorf("open embedded migrations: %w", err)
	}

	var m *migrate.Migrate
	var lastErr error
	for attempt := 1; attempt <= 15; attempt++ {
		m, lastErr = migrate.NewWithSourceInstance("iofs", source, databaseURL)
		if lastErr == nil {
			break
		}
		time.Sleep(1 * time.Second)
	}
	if lastErr != nil {
		return fmt.Errorf("init migrator after retries: %w", lastErr)
	}
	defer m.Close()

	for attempt := 1; attempt <= 15; attempt++ {
		lastErr = m.Up()
		if lastErr == nil || errors.Is(lastErr, migrate.ErrNoChange) {
			return nil
		}
		time.Sleep(1 * time.Second)
	}
	return fmt.Errorf("apply migrations after retries: %w", lastErr)
}
