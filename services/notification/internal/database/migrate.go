package database

import (
	"errors"
	"fmt"
	"io/fs"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

// Migrate applies all pending migrations from the given filesystem.
func Migrate(databaseURL string, migrations fs.FS) error {
	source, err := iofs.New(migrations, ".")
	if err != nil {
		return fmt.Errorf("open migrations: %w", err)
	}

	m, err := migrate.NewWithSourceInstance("iofs", source, migrateURL(databaseURL))
	if err != nil {
		return fmt.Errorf("create migrator: %w", err)
	}
	defer m.Close()

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("apply migrations: %w", err)
	}
	return nil
}

// migrateURL switches the scheme so golang-migrate picks its pgx v5 driver.
func migrateURL(databaseURL string) string {
	for _, scheme := range []string{"postgresql://", "postgres://"} {
		if strings.HasPrefix(databaseURL, scheme) {
			return "pgx5://" + strings.TrimPrefix(databaseURL, scheme)
		}
	}
	return databaseURL
}
