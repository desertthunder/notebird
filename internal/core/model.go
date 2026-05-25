package core

import (
	"sort"
	"time"
)

var fieldSummaryKeys = []string{"kind", "status", "project", "source", "rating", "due"}
var reservedDisplayFields = map[string]bool{"title": true, "tags": true}

type Chirp struct {
	ID          string
	Title       string
	Text        string
	Type        string
	Tags        []string
	Refs        []ChirpRef
	Backlinks   []ChirpRef
	Attachments []Attachment
	Fields      map[string]string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	HTML        string
}

type Attachment struct {
	Hash        string
	Filename    string
	ContentType string
	Size        int64
	CreatedAt   time.Time
	URL         string
	IsImage     bool
}

type Settings struct {
	EditorMode     string `json:"editor_mode"`
	WordWrap       bool   `json:"word_wrap"`
	EditorFontSize int    `json:"editor_font_size"`
}

type ChirpRef struct {
	FromID     string
	ToID       string
	RefText    string
	Resolved   bool
	FromTitle  string
	ToTitle    string
	CreatedFor string
}

type Field struct {
	Key   string
	Value string
}

type TagCount struct {
	Tag   string
	Count int
}

type FeedFilter struct {
	Query string
	Tag   string
	Mode  string
}

type ChirpForm struct {
	Chirp            Chirp
	Action           string
	Method           string
	SubmitLabel      string
	Heading          string
	Draft            bool
	ShowCancel       bool
	TitlePlaceholder string
	TextPlaceholder  string
	TagValue         string
	FieldValue       string
	DraftID          string
	Attachments      []Attachment
	Settings         Settings
}

type Config struct {
	DataDir string `json:"data_dir"`
	Host    string `json:"host"`
	Port    int    `json:"port"`
}

func NewConfig(h string, p int) Config {
	return Config{
		Host: h,
		Port: p,
	}
}

// FieldSummary returns fields shown as title-adjacent badges.
func FieldSummary(fields map[string]string) []Field {
	if len(fields) == 0 {
		return nil
	}
	out := make([]Field, 0, len(fieldSummaryKeys))
	for _, key := range fieldSummaryKeys {
		if value := fields[key]; value != "" {
			out = append(out, Field{Key: key, Value: value})
		}
	}
	return out
}

// FieldRows returns displayable fields in key order.
func FieldRows(fields map[string]string) []Field {
	if len(fields) == 0 {
		return nil
	}
	keys := make([]string, 0, len(fields))
	for key, value := range fields {
		if key == "" || value == "" || reservedDisplayFields[key] {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]Field, 0, len(keys))
	for _, key := range keys {
		out = append(out, Field{Key: key, Value: fields[key]})
	}
	return out
}

func shortID(s string) string {
	if len(s) <= 8 {
		return s
	}
	return s[:8]
}
