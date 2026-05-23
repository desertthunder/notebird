package core

import (
	"strings"
	"testing"
)

func TestParseWikiLinks(t *testing.T) {
	links := ParseWikiLinks("See [[Alpha]] and [[Readable label|Beta Note]] and [[Alpha]].")
	if len(links) != 2 {
		t.Fatalf("expected 2 unique links, got %d", len(links))
	}
	if links[0].Label != "Alpha" || links[0].Target != "Alpha" {
		t.Fatalf("unexpected first link: %#v", links[0])
	}
	if links[1].Label != "Readable label" || links[1].Target != "Beta Note" {
		t.Fatalf("unexpected second link: %#v", links[1])
	}
}

func TestRenderWikiLinks(t *testing.T) {
	html := RenderWikiLinks("See [[Alpha]] and [[Missing]].", []ChirpRef{{RefText: "Alpha", ToID: "01ABC", Resolved: true}})
	if want := `hx-get="/chirps/01ABC"`; !contains(html, want) {
		t.Fatalf("expected resolved htmx link %q in %s", want, html)
	}
	if want := `wiki-link--missing`; !contains(html, want) {
		t.Fatalf("expected missing link class %q in %s", want, html)
	}
}

func contains(s, substr string) bool { return strings.Contains(s, substr) }
