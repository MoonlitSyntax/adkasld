package ingest

import (
	"io/fs"
	"path/filepath"
	"strings"
)

type SourceFile struct {
	Path string
}

func DiscoverSource(root string) ([]SourceFile, error) {
	var out []SourceFile

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if strings.HasSuffix(strings.ToLower(d.Name()), ".md") || strings.HasSuffix(strings.ToLower(d.Name()), ".markdown") {
			out = append(out, SourceFile{Path: path})
		}
		return nil
	})
	return out, err
}
