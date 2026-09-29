package dbmeta_test

import (
	"testing"

	"github.com/xo/dbmeta"
)

func TestLike(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		pattern, s string
		want       bool
	}{
		{"", "anything", true},
		{"book", "book", true},
		{"book", "books", false},
		{"bo%", "book", true},
		{"%ok", "book", true},
		{"b_ok", "book", true},
		{"b_ok", "bok", false},
		{"%", "", true},
		{"a%b%c", "axxbyyc", true},
		{"a%b%c", "axxbyy", false},
		{`db\_x`, "db_x", true},
		{`db\_x`, "dbax", false},
	} {
		if got := dbmeta.Like(c.pattern, c.s); got != c.want {
			t.Errorf("Like(%q, %q) = %v, want %v", c.pattern, c.s, got, c.want)
		}
	}
}
