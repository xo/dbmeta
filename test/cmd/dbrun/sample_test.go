package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestCsvqStartsFromTheSamples checks that the csvq directory gets the four
// core tables, and that a file already there is kept. See D116.
func TestCsvqStartsFromTheSamples(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "csvq")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	kept := filepath.Join(dir, "book.csv")
	if err := os.WriteFile(kept, []byte("changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := extractSamples(target{Name: "csvq", DSN: dir}); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"author.csv", "book.csv", "region.csv", "shipment.csv"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("%s is missing: %v", name, err)
		}
	}
	if b, _ := os.ReadFile(kept); string(b) != "changed\n" {
		t.Errorf("book.csv was overwritten: %q", b)
	}
	if err := extractSamples(target{Name: "chai", DSN: filepath.Join(t.TempDir(), "chai")}); err != nil {
		t.Errorf("a database with no samples: %v", err)
	}
}
