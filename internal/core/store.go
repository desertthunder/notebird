package core

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"
	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

func OpenStore(ctx context.Context, dataDir string) (*Store, error) {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", filepath.Join(dataDir, "notebird.db"))
	if err != nil {
		return nil, err
	}
	s := &Store{db: db}
	if err := s.migrate(ctx); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) migrate(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `
        pragma journal_mode = wal;
        pragma foreign_keys = on;

        create table if not exists chirps (
            id text primary key,
            title text not null,
            text text not null,
            type text not null default 'text/markdown',
            created_at text not null,
            updated_at text not null
        );

        create table if not exists chirp_tags (
            chirp_id text not null references chirps(id) on delete cascade,
            tag text not null,
            primary key (chirp_id, tag)
        );

        create table if not exists chirp_fields (
            chirp_id text not null references chirps(id) on delete cascade,
            key text not null,
            value text not null,
            primary key (chirp_id, key)
        );

        create table if not exists chirp_refs (
            from_chirp_id text not null references chirps(id) on delete cascade,
            to_chirp_id text,
            ref_text text not null,
            resolved integer not null default 0,
            primary key (from_chirp_id, ref_text)
        );
    `)
	return err
}

func (s *Store) CreateChirp(ctx context.Context, title, text string) (Chirp, error) {
	title = strings.TrimSpace(title)
	text = strings.TrimSpace(text)
	if text == "" {
		return Chirp{}, errors.New("chirp text is required")
	}
	if title == "" {
		title = firstLineTitle(text)
	}

	now := time.Now().UTC()
	c := Chirp{
		ID:        ulid.Make().String(),
		Title:     title,
		Text:      text,
		Type:      "text/markdown",
		Tags:      extractTags(text),
		CreatedAt: now,
		UpdatedAt: now,
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Chirp{}, err
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx, `insert into chirps (id, title, text, type, created_at, updated_at) values (?, ?, ?, ?, ?, ?)`,
		c.ID, c.Title, c.Text, c.Type, c.CreatedAt.Format(time.RFC3339Nano), c.UpdatedAt.Format(time.RFC3339Nano))
	if err != nil {
		return Chirp{}, err
	}
	for _, tag := range c.Tags {
		if _, err := tx.ExecContext(ctx, `insert or ignore into chirp_tags (chirp_id, tag) values (?, ?)`, c.ID, tag); err != nil {
			return Chirp{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return Chirp{}, err
	}
	return c, nil
}

func (s *Store) ListChirps(ctx context.Context, limit int) ([]Chirp, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx, `select id, title, text, type, created_at, updated_at from chirps order by created_at desc limit ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var chirps []Chirp
	for rows.Next() {
		c, err := scanChirp(rows)
		if err != nil {
			return nil, err
		}
		c.Tags, _ = s.tags(ctx, c.ID)
		chirps = append(chirps, c)
	}
	return chirps, rows.Err()
}

func (s *Store) GetChirp(ctx context.Context, id string) (Chirp, error) {
	row := s.db.QueryRowContext(ctx, `select id, title, text, type, created_at, updated_at from chirps where id = ?`, id)
	c, err := scanChirp(row)
	if err != nil {
		return Chirp{}, err
	}
	c.Tags, _ = s.tags(ctx, c.ID)
	return c, nil
}

func (s *Store) tags(ctx context.Context, id string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `select tag from chirp_tags where chirp_id = ? order by tag`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tags []string
	for rows.Next() {
		var tag string
		if err := rows.Scan(&tag); err != nil {
			return nil, err
		}
		tags = append(tags, tag)
	}
	return tags, rows.Err()
}

type chirpScanner interface {
	Scan(dest ...any) error
}

func scanChirp(row chirpScanner) (Chirp, error) {
	var c Chirp
	var created, updated string
	if err := row.Scan(&c.ID, &c.Title, &c.Text, &c.Type, &created, &updated); err != nil {
		return Chirp{}, err
	}
	c.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
	c.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updated)
	return c, nil
}

func firstLineTitle(text string) string {
	line := strings.TrimSpace(strings.Split(text, "\n")[0])
	line = strings.TrimPrefix(line, "#")
	line = strings.TrimSpace(line)
	if line == "" {
		return "Untitled Chirp"
	}
	if len([]rune(line)) > 72 {
		return string([]rune(line)[:72]) + "…"
	}
	return line
}

func extractTags(text string) []string {
	seen := map[string]bool{}
	var tags []string
	for _, word := range strings.Fields(text) {
		word = strings.Trim(word, "\t\n\r .,;:!?()[]{}<>")
		if strings.HasPrefix(word, "#") && len(word) > 1 {
			tag := strings.TrimPrefix(word, "#")
			if !seen[tag] {
				seen[tag] = true
				tags = append(tags, tag)
			}
		}
	}
	return tags
}

func (s *Store) Stats(ctx context.Context) (map[string]any, error) {
	var count int
	if err := s.db.QueryRowContext(ctx, `select count(*) from chirps`).Scan(&count); err != nil {
		return nil, fmt.Errorf("count chirps: %w", err)
	}
	return map[string]any{"chirps": count}, nil
}
