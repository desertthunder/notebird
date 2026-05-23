package core

import (
	"context"
	"testing"
)

func TestStoreFieldsAndRefs(t *testing.T) {
	ctx := context.Background()
	store, err := OpenStore(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	a, err := store.CreateChirp(ctx, "Alpha", "Link to [[Beta]] and [[Missing Note]].")
	if err != nil {
		t.Fatal(err)
	}

	refs, err := store.OutgoingRefs(ctx, a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) != 2 {
		t.Fatalf("expected 2 refs, got %d", len(refs))
	}
	if refs[0].Resolved || refs[1].Resolved {
		t.Fatalf("expected refs to start unresolved: %#v", refs)
	}

	b, err := store.CreateChirp(ctx, "Beta", "I am beta.")
	if err != nil {
		t.Fatal(err)
	}

	refs, err = store.OutgoingRefs(ctx, a.ID)
	if err != nil {
		t.Fatal(err)
	}
	var foundBeta bool
	for _, ref := range refs {
		if ref.RefText == "Beta" {
			foundBeta = true
			if !ref.Resolved || ref.ToID != b.ID {
				t.Fatalf("expected Beta ref resolved to %s, got %#v", b.ID, ref)
			}
		}
	}
	if !foundBeta {
		t.Fatalf("expected Beta ref in %#v", refs)
	}

	backlinks, err := store.Backlinks(ctx, b.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(backlinks) != 1 || backlinks[0].FromID != a.ID || backlinks[0].FromTitle != "Alpha" {
		t.Fatalf("unexpected backlinks: %#v", backlinks)
	}

	if err := store.SetField(ctx, a.ID, "mood", "curious"); err != nil {
		t.Fatal(err)
	}
	fields, err := store.Fields(ctx, a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if fields["mood"] != "curious" {
		t.Fatalf("expected mood field, got %#v", fields)
	}

	if err := store.DeleteField(ctx, a.ID, "mood"); err != nil {
		t.Fatal(err)
	}
	fields, err = store.Fields(ctx, a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := fields["mood"]; ok {
		t.Fatalf("expected mood deleted, got %#v", fields)
	}
}

func TestStoreUpdateDeleteAndSuggestions(t *testing.T) {
	ctx := context.Background()
	store, err := OpenStore(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	c, err := store.CreateChirp(ctx, "Alpha", "Initial #one")
	if err != nil {
		t.Fatal(err)
	}
	updated, err := store.UpdateChirp(ctx, c.ID, "Beta", "Updated #two [[Alpha]]")
	if err != nil {
		t.Fatal(err)
	}
	if updated.Title != "Beta" || updated.Text != "Updated #two [[Alpha]]" {
		t.Fatalf("unexpected updated chirp: %#v", updated)
	}

	titles, err := store.SuggestTitles(ctx, "Bet", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(titles) != 1 || titles[0].ID != c.ID {
		t.Fatalf("unexpected title suggestions: %#v", titles)
	}

	tags, err := store.SuggestTags(ctx, "tw", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(tags) != 1 || tags[0] != "two" {
		t.Fatalf("unexpected tag suggestions: %#v", tags)
	}

	if err := store.DeleteChirp(ctx, c.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetChirp(ctx, c.ID); err == nil {
		t.Fatal("expected deleted chirp to be missing")
	}
}
