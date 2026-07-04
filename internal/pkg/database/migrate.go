package database

import (
	"database/sql"
	"embed"
	"fmt"

	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

func runMigrations(db *sql.DB) error {
	goose.SetBaseFS(migrationFS)
	if err := goose.SetDialect("sqlite3"); err != nil {
		return fmt.Errorf("failed to set migration dialect: %w", err)
	}
	if err := goose.Up(db, "migrations"); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}
	return nil
}
