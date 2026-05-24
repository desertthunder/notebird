package core

import "time"

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

type ChirpRef struct {
	FromID     string
	ToID       string
	RefText    string
	Resolved   bool
	FromTitle  string
	ToTitle    string
	CreatedFor string
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

func shortID(s string) string {
	if len(s) <= 8 {
		return s
	}
	return s[:8]
}
