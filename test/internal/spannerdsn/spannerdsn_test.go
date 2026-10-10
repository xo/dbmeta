package spannerdsn_test

import (
	"errors"
	"testing"

	"github.com/xo/dbmeta/test/internal/spannerdsn"
)

func TestFromURL(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		in, want string
	}{
		{"spanner:///p/i/d?credential_file=/k/key.json", "projects/p/instances/i/databases/d;credentials=/k/key.json"},
		{"spanner://127.0.0.1:9010/p/i/d", "127.0.0.1:9010/projects/p/instances/i/databases/d"},
	} {
		got, err := spannerdsn.FromURL(test.in)
		if err != nil || got != test.want {
			t.Errorf("FromURL(%q) = %q, %v, want %q", test.in, got, err, test.want)
		}
	}
	for _, in := range []string{"", "postgres://h/p/i/d", "spanner:///p/i"} {
		if _, err := spannerdsn.FromURL(in); !errors.Is(err, spannerdsn.ErrInvalid) {
			t.Errorf("FromURL(%q): expected ErrInvalid, got %v", in, err)
		}
	}
}
