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

// Ping verifies that the backing database connection can handle a simple query.
func (s *Store) Ping(ctx context.Context) error {
	return s.db.PingContext(ctx)
}

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
            to_chirp_id text references chirps(id) on delete set null,
            ref_text text not null,
            resolved integer not null default 0,
            primary key (from_chirp_id, ref_text)
        );

        create index if not exists chirps_title_idx on chirps(title);
        create index if not exists chirps_title_nocase_idx on chirps(title collate nocase);
        create index if not exists chirp_refs_to_idx on chirp_refs(to_chirp_id);
        create index if not exists chirp_refs_missing_idx on chirp_refs(ref_text) where resolved = 0;

        create virtual table if not exists chirps_fts using fts5(id unindexed, title, text);
        insert into chirps_fts (id, title, text)
            select c.id, c.title, c.text from chirps c
            where not exists (select 1 from chirps_fts f where f.id = c.id);
    `)
	return err
}

func (s *Store) CreateChirp(ctx context.Context, title, text string, tagInput ...string) (Chirp, error) {
	title, text, tags, fields, err := prepareChirpInput(title, text, strings.Join(tagInput, ","))
	if err != nil {
		return Chirp{}, err
	}
	now := time.Now().UTC()
	c := Chirp{
		ID:        ulid.Make().String(),
		Title:     title,
		Text:      text,
		Type:      "text/markdown",
		Tags:      tags,
		Fields:    fields,
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
	if _, err := tx.ExecContext(ctx, `insert into chirps_fts (id, title, text) values (?, ?, ?)`, c.ID, c.Title, c.Text); err != nil {
		return Chirp{}, err
	}
	for _, tag := range c.Tags {
		if _, err := tx.ExecContext(ctx, `insert or ignore into chirp_tags (chirp_id, tag) values (?, ?)`, c.ID, tag); err != nil {
			return Chirp{}, err
		}
	}
	if err := replaceFields(ctx, tx, c.ID, fields); err != nil {
		return Chirp{}, err
	}
	if err := s.replaceRefs(ctx, tx, c.ID, c.Text); err != nil {
		return Chirp{}, err
	}
	if _, err := tx.ExecContext(ctx, `update chirp_refs set to_chirp_id = ?, resolved = 1 where resolved = 0 and ref_text = ? collate nocase`, c.ID, c.Title); err != nil {
		return Chirp{}, err
	}
	if err := tx.Commit(); err != nil {
		return Chirp{}, err
	}
	return c, nil
}

func (s *Store) UpdateChirp(ctx context.Context, id, title, text string, tagInput ...string) (Chirp, error) {
	title, text, tags, fields, err := prepareChirpInput(title, text, strings.Join(tagInput, ","))
	if err != nil {
		return Chirp{}, err
	}

	existing, err := s.GetChirp(ctx, id)
	if err != nil {
		return Chirp{}, err
	}

	now := time.Now().UTC()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Chirp{}, err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `update chirps set title = ?, text = ?, updated_at = ? where id = ?`, title, text, now.Format(time.RFC3339Nano), id); err != nil {
		return Chirp{}, err
	}
	if _, err := tx.ExecContext(ctx, `delete from chirps_fts where id = ?`, id); err != nil {
		return Chirp{}, err
	}
	if _, err := tx.ExecContext(ctx, `insert into chirps_fts (id, title, text) values (?, ?, ?)`, id, title, text); err != nil {
		return Chirp{}, err
	}
	if _, err := tx.ExecContext(ctx, `delete from chirp_tags where chirp_id = ?`, id); err != nil {
		return Chirp{}, err
	}
	for _, tag := range tags {
		if _, err := tx.ExecContext(ctx, `insert or ignore into chirp_tags (chirp_id, tag) values (?, ?)`, id, tag); err != nil {
			return Chirp{}, err
		}
	}
	if err := replaceFields(ctx, tx, id, fields); err != nil {
		return Chirp{}, err
	}
	if err := s.replaceRefs(ctx, tx, id, text); err != nil {
		return Chirp{}, err
	}
	if existing.Title != title {
		if _, err := tx.ExecContext(ctx, `update chirp_refs set to_chirp_id = null, resolved = 0 where to_chirp_id = ? and ref_text != ? collate nocase`, id, title); err != nil {
			return Chirp{}, err
		}
	}
	if _, err := tx.ExecContext(ctx, `update chirp_refs set to_chirp_id = ?, resolved = 1 where resolved = 0 and ref_text = ? collate nocase`, id, title); err != nil {
		return Chirp{}, err
	}
	if err := tx.Commit(); err != nil {
		return Chirp{}, err
	}
	return s.GetChirp(ctx, id)
}

func (s *Store) DeleteChirp(ctx context.Context, id string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `delete from chirps_fts where id = ?`, id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `delete from chirps where id = ?`, id); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) ListChirps(ctx context.Context, limit int) ([]Chirp, error) {
	return s.ListChirpsFiltered(ctx, FeedFilter{}, limit)
}

func (s *Store) ListChirpsFiltered(ctx context.Context, filter FeedFilter, limit int) ([]Chirp, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	var rows *sql.Rows
	var err error
	switch {
	case strings.TrimSpace(filter.Query) != "" && strings.TrimSpace(filter.Tag) != "":
		rows, err = s.db.QueryContext(ctx, `
			select c.id, c.title, c.text, c.type, c.created_at, c.updated_at
			from chirps c
			join chirps_fts f on f.id = c.id
			join chirp_tags t on t.chirp_id = c.id
			where chirps_fts match ? and t.tag = ? collate nocase
			order by bm25(chirps_fts), c.created_at desc
			limit ?`, ftsQuery(filter.Query), strings.TrimPrefix(filter.Tag, "#"), limit)
	case strings.TrimSpace(filter.Query) != "":
		rows, err = s.db.QueryContext(ctx, `
			select c.id, c.title, c.text, c.type, c.created_at, c.updated_at
			from chirps c
			join chirps_fts f on f.id = c.id
			where chirps_fts match ?
			order by bm25(chirps_fts), c.created_at desc
			limit ?`, ftsQuery(filter.Query), limit)
	case strings.TrimSpace(filter.Tag) != "":
		rows, err = s.db.QueryContext(ctx, `
			select c.id, c.title, c.text, c.type, c.created_at, c.updated_at
			from chirps c
			join chirp_tags t on t.chirp_id = c.id
			where t.tag = ? collate nocase
			order by c.created_at desc
			limit ?`, strings.TrimPrefix(filter.Tag, "#"), limit)
	case filter.Mode == "wanted":
		rows, err = s.db.QueryContext(ctx, `
			select distinct c.id, c.title, c.text, c.type, c.created_at, c.updated_at
			from chirps c
			join chirp_refs r on r.from_chirp_id = c.id
			where r.resolved = 0
			order by c.created_at desc
			limit ?`, limit)
	default:
		rows, err = s.db.QueryContext(ctx, `select id, title, text, type, created_at, updated_at from chirps order by created_at desc limit ?`, limit)
	}
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
		c.Refs, _ = s.OutgoingRefs(ctx, c.ID)
		c.Fields, _ = s.Fields(ctx, c.ID)
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
	c.Refs, _ = s.OutgoingRefs(ctx, c.ID)
	c.Backlinks, _ = s.Backlinks(ctx, c.ID)
	c.Fields, _ = s.Fields(ctx, c.ID)
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

func prepareChirpInput(title, text, tagInput string) (string, string, []string, map[string]string, error) {
	title = strings.TrimSpace(title)
	text = strings.TrimSpace(text)
	if text == "" {
		return "", "", nil, nil, errors.New("chirp text is required")
	}
	body, frontmatter := splitFrontmatter(text)
	body = strings.TrimSpace(body)
	fields := frontmatterFields(frontmatter)
	if title == "" {
		title = strings.TrimSpace(fields["title"])
	}
	if title == "" {
		title = firstLineTitle(body)
	}
	tags := extractTags(title, body, append(frontmatterTags(frontmatter), parseTagInput(tagInput)...)...)
	return title, body, tags, fields, nil
}

func parseTagInput(input string) []string {
	var tags []string
	for _, part := range strings.FieldsFunc(input, func(r rune) bool { return r == ',' || r == ' ' || r == '\t' || r == '\n' || r == '#' }) {
		if part = strings.TrimSpace(part); part != "" {
			tags = append(tags, part)
		}
	}
	return tags
}

func extractTags(title, text string, explicit ...string) []string {
	body, frontmatter := splitFrontmatter(text)
	if frontmatter != "" {
		text = body
		explicit = append(frontmatterTags(frontmatter), explicit...)
	}
	seen := map[string]bool{}
	var tags []string
	add := func(tag string) {
		tag = strings.Trim(strings.TrimPrefix(tag, "#"), " \t\n\r.,;:!?()[]{}<>")
		if tag == "" || strings.Contains(tag, "#") {
			return
		}
		for _, r := range tag {
			if !(r == '-' || r == '_' || r >= '0' && r <= '9' || r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z') {
				return
			}
		}
		if !seen[tag] {
			seen[tag] = true
			tags = append(tags, tag)
		}
	}
	for _, tag := range explicit {
		add(tag)
	}
	for _, word := range strings.Fields(title + "\n" + text) {
		word = strings.Trim(word, "\t\n\r .,;:!?()[]{}<>")
		if strings.HasPrefix(word, "#") && len(word) > 1 {
			add(word)
		}
	}
	return tags
}

func (s *Store) TagCounts(ctx context.Context, limit int) ([]TagCount, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx, `select tag, count(*) from chirp_tags group by tag order by count(*) desc, tag limit ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tags []TagCount
	for rows.Next() {
		var tag TagCount
		if err := rows.Scan(&tag.Tag, &tag.Count); err != nil {
			return nil, err
		}
		tags = append(tags, tag)
	}
	return tags, rows.Err()
}

func (s *Store) SuggestTitles(ctx context.Context, q string, limit int) ([]Chirp, error) {
	q = strings.TrimSpace(q)
	if limit <= 0 || limit > 25 {
		limit = 10
	}
	rows, err := s.db.QueryContext(ctx, `select id, title, text, type, created_at, updated_at from chirps where title like ? collate nocase order by updated_at desc limit ?`, "%"+q+"%", limit)
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
		chirps = append(chirps, c)
	}
	return chirps, rows.Err()
}

func (s *Store) SuggestTags(ctx context.Context, q string, limit int) ([]string, error) {
	q = strings.TrimSpace(strings.TrimPrefix(q, "#"))
	if limit <= 0 || limit > 25 {
		limit = 10
	}
	rows, err := s.db.QueryContext(ctx, `select tag from chirp_tags where tag like ? collate nocase group by tag order by count(*) desc, tag limit ?`, q+"%", limit)
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

func ftsQuery(q string) string {
	var terms []string
	for _, raw := range strings.Fields(q) {
		term := strings.Map(func(r rune) rune {
			switch {
			case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_', r == '-':
				return r
			default:
				return -1
			}
		}, raw)
		term = strings.Trim(term, "-")
		if term != "" {
			terms = append(terms, term+"*")
		}
	}
	if len(terms) == 0 {
		return "*"
	}
	return strings.Join(terms, " ")
}

func (s *Store) Stats(ctx context.Context) (map[string]any, error) {
	var count int
	if err := s.db.QueryRowContext(ctx, `select count(*) from chirps`).Scan(&count); err != nil {
		return nil, fmt.Errorf("count chirps: %w", err)
	}
	return map[string]any{"chirps": count}, nil
}
