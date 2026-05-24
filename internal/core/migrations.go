package core

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

//go:embed sql/*.sql
var migrationFiles embed.FS

// Migration describes one embedded database migration parsed from its filename.
type Migration struct {
	ID       string
	Date     time.Time
	Title    string
	Filename string
	SQL      string
}

var migrationFilenamePattern = regexp.MustCompile(`^(\d{4})_(\d{2})_(\d{2})_(\d{2})_(.+)\.sql$`)

// parseMigrationFilename extracts migration metadata from XXXX_yy_mm_dd_description.sql.
func parseMigrationFilename(name string) (Migration, error) {
	base := filepath.Base(name)
	matches := migrationFilenamePattern.FindStringSubmatch(base)
	if matches == nil {
		return Migration{}, fmt.Errorf("invalid migration filename %q", base)
	}
	year, _ := strconv.Atoi(matches[2])
	month, _ := strconv.Atoi(matches[3])
	day, _ := strconv.Atoi(matches[4])
	date := time.Date(2000+year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	if date.Year() != 2000+year || int(date.Month()) != month || date.Day() != day {
		return Migration{}, fmt.Errorf("invalid migration date in %q", base)
	}
	return Migration{
		ID:       matches[1],
		Date:     date,
		Title:    strings.ReplaceAll(matches[5], "_", " "),
		Filename: base,
	}, nil
}

// loadMigrations reads embedded SQL files and returns them in migration ID order.
func loadMigrations() ([]Migration, error) {
	entries, err := fs.ReadDir(migrationFiles, "sql")
	if err != nil {
		return nil, err
	}
	migrations := make([]Migration, 0, len(entries))
	seen := map[string]string{}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		path := "sql/" + entry.Name()
		migration, err := parseMigrationFilename(entry.Name())
		if err != nil {
			return nil, err
		}
		if other := seen[migration.ID]; other != "" {
			return nil, fmt.Errorf("duplicate migration id %s in %s and %s", migration.ID, other, migration.Filename)
		}
		body, err := migrationFiles.ReadFile(path)
		if err != nil {
			return nil, err
		}
		migration.SQL = string(body)
		seen[migration.ID] = migration.Filename
		migrations = append(migrations, migration)
	}
	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].ID < migrations[j].ID
	})
	return migrations, nil
}

// applyMigrations records and applies embedded migrations that have not run yet.
func (s *Store) applyMigrations(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, `
		create table if not exists schema_migrations (
			id text primary key,
			filename text not null,
			migration_date text not null,
			title text not null,
			applied_at text not null
		);
	`); err != nil {
		return err
	}

	applied, err := s.appliedMigrations(ctx)
	if err != nil {
		return err
	}
	migrations, err := loadMigrations()
	if err != nil {
		return err
	}
	for _, migration := range migrations {
		if applied[migration.ID] {
			continue
		}
		if err := s.applyMigration(ctx, migration); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) appliedMigrations(ctx context.Context) (map[string]bool, error) {
	rows, err := s.db.QueryContext(ctx, `select id from schema_migrations`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	applied := map[string]bool{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		applied[id] = true
	}
	return applied, rows.Err()
}

func (s *Store) applyMigration(ctx context.Context, migration Migration) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, migration.SQL); err != nil {
		return fmt.Errorf("apply migration %s: %w", migration.Filename, err)
	}
	if _, err := tx.ExecContext(ctx, `
		insert into schema_migrations (id, filename, migration_date, title, applied_at)
		values (?, ?, ?, ?, ?)
	`, migration.ID, migration.Filename, migration.Date.Format(time.DateOnly), migration.Title, time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
		return fmt.Errorf("record migration %s: %w", migration.Filename, err)
	}
	return tx.Commit()
}
