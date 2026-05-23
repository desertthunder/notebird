package core

import "time"

type Chirp struct {
	ID        string
	Title     string
	Text      string
	Type      string
	Tags      []string
	Refs      []ChirpRef
	Backlinks []ChirpRef
	Fields    map[string]string
	CreatedAt time.Time
	UpdatedAt time.Time
	HTML      string
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
