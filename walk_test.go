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

func TestLikeFold(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		pattern, s string
		want       bool
	}{
		{"", "Anything", true},
		{"BOOK", "book", true},
		{"bo%", "BOOK", true},
		{"b_OK", "Book", true},
		{`DB\_x`, "db_X", true},
		{`DB\_x`, "dbaX", false},
		{"book", "books", false},
	} {
		if got := dbmeta.LikeFold(c.pattern, c.s); got != c.want {
			t.Errorf("LikeFold(%q, %q) = %v, want %v", c.pattern, c.s, got, c.want)
		}
	}
}

func TestListHas(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		list, word string
		want       bool
	}{
		{"", "table", true},
		{"table", "table", true},
		{"table,view", "view", true},
		{"table,view", "views", false},
		{"table,view", "tab", false},
		{"table, view", "view", false},
	} {
		if got := dbmeta.ListHas(c.list, c.word); got != c.want {
			t.Errorf("ListHas(%q, %q) = %v, want %v", c.list, c.word, got, c.want)
		}
	}
}
