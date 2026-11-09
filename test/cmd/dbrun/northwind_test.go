package main

import (
	"slices"
	"strings"
	"testing"
)

// TestEveryNorthwindFileIsEmbedded holds that each file that a product names is
// in the embedded files and is not empty.
func TestEveryNorthwindFileIsEmbedded(t *testing.T) {
	t.Parallel()
	for d, src := range northwindSources {
		for _, name := range northwindAll(src) {
			b, err := northwindFiles.ReadFile(name)
			if err != nil {
				t.Errorf("%s: reading %s: %v", d, name, err)
				continue
			}
			if len(b) == 0 {
				t.Errorf("%s: %s is empty", d, name)
			}
		}
	}
}

// TestNorthwindFilesNeedOnlyUsql holds that no file ends a statement with a
// line that holds only a slash. usql reads a file as psql does, and ends a
// statement at a semicolon.
func TestNorthwindFilesNeedOnlyUsql(t *testing.T) {
	t.Parallel()
	for d, src := range northwindSources {
		for _, name := range northwindAll(src) {
			b, err := northwindFiles.ReadFile(name)
			if err != nil {
				t.Fatalf("%s: reading %s: %v", d, name, err)
			}
			for i, l := range strings.Split(string(b), "\n") {
				if strings.TrimSpace(l) == "/" {
					t.Errorf("%s: %s line %d is a slash, and usql ends a statement at a semicolon", d, name, i+1)
					break
				}
			}
		}
	}
}

// TestNorthwindProductsCount holds the products that have the fixture.
func TestNorthwindProductsCount(t *testing.T) {
	t.Parallel()
	if got := len(northwindSources); got != 8 {
		t.Errorf("the fixture is written for %d products, and the test expects 8", got)
	}
}

// northwindAll is every file of a source, whichever product applies it.
func northwindAll(src northwindSource) []string {
	names := slices.Clone(src.files)
	for _, more := range src.more {
		names = append(names, more...)
	}
	return names
}
