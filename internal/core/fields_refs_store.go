package core

import (
	"context"
	"database/sql"
)

type txer interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func (s *Store) Fields(ctx context.Context, chirpID string) (map[string]string, error) {
	rows, err := s.db.QueryContext(ctx, `select key, value from chirp_fields where chirp_id = ? order by key`, chirpID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	fields := map[string]string{}
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return nil, err
		}
		fields[key] = value
	}
	return fields, rows.Err()
}

func (s *Store) SetField(ctx context.Context, chirpID, key, value string) error {
	_, err := s.db.ExecContext(ctx, `
		insert into chirp_fields (chirp_id, key, value) values (?, ?, ?)
		on conflict (chirp_id, key) do update set value = excluded.value
	`, chirpID, key, value)
	return err
}

func (s *Store) DeleteField(ctx context.Context, chirpID, key string) error {
	_, err := s.db.ExecContext(ctx, `delete from chirp_fields where chirp_id = ? and key = ?`, chirpID, key)
	return err
}

func (s *Store) ReplaceFields(ctx context.Context, chirpID string, fields map[string]string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `delete from chirp_fields where chirp_id = ?`, chirpID); err != nil {
		return err
	}
	for key, value := range fields {
		if _, err := tx.ExecContext(ctx, `insert into chirp_fields (chirp_id, key, value) values (?, ?, ?)`, chirpID, key, value); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) replaceRefs(ctx context.Context, tx txer, chirpID, text string) error {
	if _, err := tx.ExecContext(ctx, `delete from chirp_refs where from_chirp_id = ?`, chirpID); err != nil {
		return err
	}
	for _, link := range ParseWikiLinks(text) {
		toID, resolved, err := s.resolveTitle(ctx, tx, link.Target)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `insert or replace into chirp_refs (from_chirp_id, to_chirp_id, ref_text, resolved) values (?, ?, ?, ?)`, chirpID, nullableString(toID), link.Target, boolInt(resolved)); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) resolveTitle(ctx context.Context, q txer, title string) (string, bool, error) {
	var id string
	err := q.QueryRowContext(ctx, `select id from chirps where title = ? collate nocase order by updated_at desc limit 1`, title).Scan(&id)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return id, true, nil
}

func (s *Store) OutgoingRefs(ctx context.Context, chirpID string) ([]ChirpRef, error) {
	rows, err := s.db.QueryContext(ctx, `
		select r.from_chirp_id, coalesce(r.to_chirp_id, ''), r.ref_text, r.resolved, coalesce(t.title, '')
		from chirp_refs r
		left join chirps t on t.id = r.to_chirp_id
		where r.from_chirp_id = ?
		order by r.ref_text
	`, chirpID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var refs []ChirpRef
	for rows.Next() {
		var ref ChirpRef
		var resolved int
		if err := rows.Scan(&ref.FromID, &ref.ToID, &ref.RefText, &resolved, &ref.ToTitle); err != nil {
			return nil, err
		}
		ref.Resolved = resolved == 1
		refs = append(refs, ref)
	}
	return refs, rows.Err()
}

func (s *Store) Backlinks(ctx context.Context, chirpID string) ([]ChirpRef, error) {
	rows, err := s.db.QueryContext(ctx, `
		select r.from_chirp_id, coalesce(r.to_chirp_id, ''), r.ref_text, r.resolved, f.title
		from chirp_refs r
		join chirps f on f.id = r.from_chirp_id
		where r.to_chirp_id = ?
		order by f.updated_at desc
	`, chirpID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var refs []ChirpRef
	for rows.Next() {
		var ref ChirpRef
		var resolved int
		if err := rows.Scan(&ref.FromID, &ref.ToID, &ref.RefText, &resolved, &ref.FromTitle); err != nil {
			return nil, err
		}
		ref.Resolved = resolved == 1
		refs = append(refs, ref)
	}
	return refs, rows.Err()
}

func (s *Store) WantedRefs(ctx context.Context) ([]ChirpRef, error) {
	rows, err := s.db.QueryContext(ctx, `
		select r.from_chirp_id, '', r.ref_text, r.resolved, f.title
		from chirp_refs r
		join chirps f on f.id = r.from_chirp_id
		where r.resolved = 0
		order by r.ref_text, f.updated_at desc
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var refs []ChirpRef
	for rows.Next() {
		var ref ChirpRef
		var resolved int
		if err := rows.Scan(&ref.FromID, &ref.ToID, &ref.RefText, &resolved, &ref.FromTitle); err != nil {
			return nil, err
		}
		ref.Resolved = resolved == 1
		refs = append(refs, ref)
	}
	return refs, rows.Err()
}

func nullableString(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}
