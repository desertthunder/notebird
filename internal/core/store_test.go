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

	frontmatterChirp, err := store.CreateChirp(ctx, "With Fields", "---\ntags: [fieldtag]\nkind: note\n---\nBody")
	if err != nil {
		t.Fatal(err)
	}
	frontmatterFields, err := store.Fields(ctx, frontmatterChirp.ID)
	if err != nil {
		t.Fatal(err)
	}
	if frontmatterFields["tags"] != "fieldtag" || frontmatterFields["kind"] != "note" {
		t.Fatalf("expected frontmatter fields stored, got %#v", frontmatterFields)
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

func TestFrontmatterTitleUsedAsDefaultTitle(t *testing.T) {
	ctx := context.Background()
	store, err := OpenStore(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	c, err := store.CreateChirp(ctx, "", "---\ntitle: Art\ntags: [front, matter]\n---\nBody #inline", "manual #typed")
	if err != nil {
		t.Fatal(err)
	}
	if c.Title != "Art" {
		t.Fatalf("expected frontmatter title Art, got %q", c.Title)
	}
	if c.Text != "Body #inline" {
		t.Fatalf("expected frontmatter pruned from text, got %q", c.Text)
	}
	stored, err := store.GetChirp(ctx, c.ID)
	if err != nil {
		t.Fatal(err)
	}
	wantTags := map[string]bool{"front": true, "matter": true, "inline": true, "manual": true, "typed": true}
	if len(stored.Tags) != len(wantTags) {
		t.Fatalf("expected tags %#v, got %#v", wantTags, stored.Tags)
	}
	for _, tag := range stored.Tags {
		if !wantTags[tag] {
			t.Fatalf("unexpected tag %q in %#v", tag, stored.Tags)
		}
	}
}

func TestFrontmatterTagsAndHashExtraction(t *testing.T) {
	text := "---\ntags: [front, matter]\nkind: note\n---\n# Title\n```\n#### ascii fence\n```\nBody #real-tag ####"
	_, fm := splitFrontmatter(text)
	fields := frontmatterFields(fm)
	if fields["tags"] != "front,matter" || fields["kind"] != "note" {
		t.Fatalf("unexpected frontmatter fields: %#v", fields)
	}
	tags := extractTags("", text)
	want := map[string]bool{"front": true, "matter": true, "real-tag": true}
	if len(tags) != len(want) {
		t.Fatalf("expected %d tags, got %#v", len(want), tags)
	}
	for _, tag := range tags {
		if !want[tag] {
			t.Fatalf("unexpected tag %q in %#v", tag, tags)
		}
	}
}

func TestStoreSearchAndNavigationQueries(t *testing.T) {
	ctx := context.Background()
	store, err := OpenStore(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	if _, err := store.CreateChirp(ctx, "Go Note", "Working with sqlite and search #go"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateChirp(ctx, "Garden", "Tomatoes and soil #garden [[Missing Plant]]"); err != nil {
		t.Fatal(err)
	}

	results, err := store.ListChirpsFiltered(ctx, FeedFilter{Query: "sqlite"}, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Title != "Go Note" {
		t.Fatalf("unexpected search results: %#v", results)
	}

	results, err = store.ListChirpsFiltered(ctx, FeedFilter{Tag: "garden"}, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Title != "Garden" {
		t.Fatalf("unexpected tag results: %#v", results)
	}

	results, err = store.ListChirpsFiltered(ctx, FeedFilter{Mode: "wanted"}, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Title != "Garden" {
		t.Fatalf("unexpected wanted results: %#v", results)
	}

	tags, err := store.TagCounts(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(tags) != 2 {
		t.Fatalf("expected two tag counts, got %#v", tags)
	}

	wanted, err := store.SuggestWantedRefs(ctx, "Plant", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(wanted) != 1 || wanted[0].RefText != "Missing Plant" {
		t.Fatalf("unexpected wanted suggestions: %#v", wanted)
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
