package main

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
)

// samples holds the files an embedded database starts from. csvq reads a
// directory of CSV files as its tables, so an empty directory is a database
// with nothing in it. The files are the four core tables of D53, author,
// book, region and shipment, with a few rows each. See D116.
//
//go:embed sample
var samples embed.FS

// extractSamples writes the sample files of an embedded database into its
// directory. A file that is there already is kept, so a test that changed
// one keeps its change, and removing the directory gives the samples again.
func extractSamples(t target) error {
	root := path.Join("sample", t.Name)
	entries, err := fs.ReadDir(samples, root)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("reading the samples of %s: %w", t.Name, err)
	}
	if err := os.MkdirAll(t.DSN, 0o755); err != nil {
		return fmt.Errorf("making the directory for %s: %w", t.DSN, err)
	}
	for _, e := range entries {
		dst := filepath.Join(t.DSN, e.Name())
		if _, err := os.Stat(dst); err == nil {
			continue
		}
		b, err := samples.ReadFile(path.Join(root, e.Name()))
		if err != nil {
			return fmt.Errorf("reading the sample %s: %w", e.Name(), err)
		}
		if err := os.WriteFile(dst, b, 0o644); err != nil {
			return fmt.Errorf("writing the sample %s: %w", dst, err)
		}
	}
	return nil
}
