package core

import (
	"context"
	"strconv"
	"time"
)

var defaultSettings = Settings{
	EditorMode:     "markdown",
	WordWrap:       true,
	EditorFontSize: 16,
}

// GetSettings returns persisted UI preferences overlaid on application defaults.
func (s *Store) GetSettings(ctx context.Context) (Settings, error) {
	settings := defaultSettings
	rows, err := s.db.QueryContext(ctx, `select key, value from app_settings`)
	if err != nil {
		return Settings{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return Settings{}, err
		}
		switch key {
		case "editor_mode":
			if value == "markdown" || value == "wysiwyg" {
				settings.EditorMode = value
			}
		case "word_wrap":
			settings.WordWrap = value == "true"
		case "editor_font_size":
			if n, err := strconv.Atoi(value); err == nil {
				settings.EditorFontSize = clampEditorFontSize(n)
			}
		}
	}
	return settings, rows.Err()
}

// SaveSettings persists UI preferences used by the composer.
func (s *Store) SaveSettings(ctx context.Context, settings Settings) (Settings, error) {
	settings = normalizeSettings(settings)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Settings{}, err
	}
	defer tx.Rollback()
	updatedAt := time.Now().UTC().Format(time.RFC3339Nano)
	values := map[string]string{
		"editor_mode":      settings.EditorMode,
		"word_wrap":        strconv.FormatBool(settings.WordWrap),
		"editor_font_size": strconv.Itoa(settings.EditorFontSize),
	}
	for key, value := range values {
		if _, err := tx.ExecContext(ctx, `insert into app_settings (key, value, updated_at) values (?, ?, ?) on conflict(key) do update set value = excluded.value, updated_at = excluded.updated_at`, key, value, updatedAt); err != nil {
			return Settings{}, err
		}
	}
	return settings, tx.Commit()
}

func normalizeSettings(settings Settings) Settings {
	if settings.EditorMode != "wysiwyg" {
		settings.EditorMode = "markdown"
	}
	settings.EditorFontSize = clampEditorFontSize(settings.EditorFontSize)
	return settings
}

func clampEditorFontSize(size int) int {
	if size < 11 {
		return 11
	}
	if size > 24 {
		return 24
	}
	return size
}
