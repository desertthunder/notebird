package core

import (
	"testing"
	"time"
)

func TestParseMigrationFilename(t *testing.T) {
	migration, err := parseMigrationFilename("0001_26_05_24_initial_schema.sql")
	if err != nil {
		t.Fatal(err)
	}
	if migration.ID != "0001" {
		t.Fatalf("ID = %q, want %q", migration.ID, "0001")
	}
	if got, want := migration.Date.Format(time.DateOnly), "2026-05-24"; got != want {
		t.Fatalf("Date = %q, want %q", got, want)
	}
	if migration.Title != "initial schema" {
		t.Fatalf("Title = %q, want %q", migration.Title, "initial schema")
	}
	if migration.Filename != "0001_26_05_24_initial_schema.sql" {
		t.Fatalf("Filename = %q", migration.Filename)
	}
}

func TestParseMigrationFilenameRejectsInvalidName(t *testing.T) {
	if _, err := parseMigrationFilename("initial_schema.sql"); err == nil {
		t.Fatal("expected invalid filename error")
	}
}

func TestLoadMigrations(t *testing.T) {
	migrations, err := loadMigrations()
	if err != nil {
		t.Fatal(err)
	}
	if len(migrations) == 0 {
		t.Fatal("expected at least one embedded migration")
	}
	if migrations[0].SQL == "" {
		t.Fatal("expected migration SQL to be loaded")
	}
}
