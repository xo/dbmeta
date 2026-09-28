package main

import (
	"io/fs"
	"strings"
	"testing"

	"github.com/xo/dbmeta/container"
)

// TestEveryContainerfileHasItsImage holds that a Containerfile and the entry
// that runs it agree. dbrun builds image/<product>.Containerfile as
// localhost/dbmeta/<product>, so an entry that names another image would
// pull one nobody publishes, and a file no entry names would be built for
// nothing.
func TestEveryContainerfileHasItsImage(t *testing.T) {
	t.Parallel()
	files, err := fs.Glob(images, "image/*.Containerfile")
	if err != nil {
		t.Fatal(err)
	}
	built := map[string]bool{}
	for _, f := range files {
		built[strings.TrimSuffix(strings.TrimPrefix(f, "image/"), ".Containerfile")] = true
	}
	named := map[string]bool{}
	for _, s := range container.All() {
		want := "localhost/dbmeta/" + s.Product
		switch {
		case built[s.Product] && s.Image != want:
			t.Errorf("%s has a Containerfile and names the image %s, not %s", s.Name(), s.Image, want)
		case !built[s.Product] && strings.HasPrefix(s.Image, "localhost/dbmeta/"):
			t.Errorf("%s names the image %s, and image/%s.Containerfile is missing", s.Name(), s.Image, s.Product)
		}
		named[s.Product] = true
	}
	for p := range built {
		if !named[p] {
			t.Errorf("image/%s.Containerfile is built by no entry", p)
		}
	}
}
