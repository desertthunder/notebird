// package utils provides common, general purpose utility functions.
package utils

import (
	"fmt"
	"strings"
	"time"
)

func TimeAgo(t time.Time) string {
	if t.IsZero() {
		return "now"
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "now"
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return strings.ToLower(t.Format("Jan 2"))
	}
}
