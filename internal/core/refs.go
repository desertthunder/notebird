package core

import (
	"html"
	"net/url"
	"regexp"
	"strings"
)

var wikiLinkPattern = regexp.MustCompile(`\[\[([^\]]+)\]\]`)

type WikiLink struct {
	Raw    string
	Label  string
	Target string
}

func ParseWikiLinks(text string) []WikiLink {
	matches := wikiLinkPattern.FindAllStringSubmatch(text, -1)
	links := make([]WikiLink, 0, len(matches))
	seen := map[string]bool{}
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		inner := strings.TrimSpace(match[1])
		if inner == "" {
			continue
		}
		label, target := splitWikiLink(inner)
		key := target + "\x00" + label
		if seen[key] {
			continue
		}
		seen[key] = true
		links = append(links, WikiLink{Raw: match[0], Label: label, Target: target})
	}
	return links
}

func splitWikiLink(inner string) (label, target string) {
	parts := strings.SplitN(inner, "|", 2)
	if len(parts) == 2 {
		label = strings.TrimSpace(parts[0])
		target = strings.TrimSpace(parts[1])
		if label == "" {
			label = target
		}
		return label, target
	}
	inner = strings.TrimSpace(inner)
	return inner, inner
}

func RenderWikiLinks(text string, refs []ChirpRef) string {
	byTarget := map[string]ChirpRef{}
	for _, ref := range refs {
		byTarget[ref.RefText] = ref
	}
	return wikiLinkPattern.ReplaceAllStringFunc(text, func(raw string) string {
		inner := strings.TrimSuffix(strings.TrimPrefix(raw, "[["), "]]")
		label, target := splitWikiLink(inner)
		ref, ok := byTarget[target]
		if ok && ref.Resolved && ref.ToID != "" {
			return `<a class="wiki-link" href="/chirps/` + html.EscapeString(ref.ToID) + `" hx-get="/chirps/` + html.EscapeString(ref.ToID) + `" hx-target="#detail" hx-swap="outerHTML">` + html.EscapeString(label) + `</a>`
		}
		return `<a class="wiki-link wiki-link--missing" href="/?wanted=` + url.QueryEscape(target) + `" title="Missing Chirp">` + html.EscapeString(label) + `</a>`
	})
}
