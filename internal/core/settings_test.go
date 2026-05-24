package core

import (
	"context"
	"testing"
)

func TestSettingsDefaultsAndSave(t *testing.T) {
	ctx := context.Background()
	store, err := OpenStore(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	settings, err := store.GetSettings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if settings.EditorMode != "markdown" || !settings.WordWrap || settings.EditorFontSize != 16 {
		t.Fatalf("unexpected defaults: %#v", settings)
	}

	saved, err := store.SaveSettings(ctx, Settings{EditorMode: "wysiwyg", WordWrap: false, EditorFontSize: 20})
	if err != nil {
		t.Fatal(err)
	}
	if saved.EditorMode != "wysiwyg" || saved.WordWrap || saved.EditorFontSize != 20 {
		t.Fatalf("unexpected saved settings: %#v", saved)
	}
	loaded, err := store.GetSettings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if loaded != saved {
		t.Fatalf("loaded = %#v, want %#v", loaded, saved)
	}
}
