package all_test

import (
	"testing"

	"github.com/xo/dbmeta"
)

// TestFoldIdentifier checks the rule each fold follows, and that a quoted
// name keeps its case. Each product's fold is measured against a real server
// by scanEveryQuery in the test module (D143).
func TestFoldIdentifier(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		dialect  dbmeta.Dialect
		in, want string
	}{
		{dbmeta.PostgreSQL, "Book", "book"},
		{dbmeta.PostgreSQL, `"Book"`, "Book"},
		{dbmeta.PostgreSQL, `"a""b"`, `a"b`},
		{dbmeta.Oracle, "book", "BOOK"},
		{dbmeta.Oracle, `"book"`, "book"},
		{dbmeta.MySQL, "Book", "Book"},
		{dbmeta.MySQL, "`Book`", "Book"},
		// SQL Server quotes with brackets and double quotes, and not with
		// backticks, so a backtick is part of the name.
		{dbmeta.SQLServer, "`Book`", "`Book`"},
		{dbmeta.Dialect("no such dialect"), "Book", "Book"},
	} {
		if got := c.dialect.FoldIdentifier(c.in); got != c.want {
			t.Errorf("%s: FoldIdentifier(%q) = %q, want %q", c.dialect, c.in, got, c.want)
		}
	}
}
