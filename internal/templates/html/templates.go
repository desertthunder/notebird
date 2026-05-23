package templates

import (
	"io/fs"
	"path/filepath"
	"sort"
)

func TemplatePatterns(fsys fs.FS, root string) ([]string, error) {
	var patterns []string
	err := fs.WalkDir(fsys, root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || filepath.Ext(path) != ".html" {
			return nil
		}
		patterns = append(patterns, path)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(patterns)
	return patterns, nil
}
