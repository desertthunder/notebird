package templates

import "embed"

// Assets contains vendored scripts, fonts, CSS, and templates.
//
//go:embed static/** templates/**
var Assets embed.FS
