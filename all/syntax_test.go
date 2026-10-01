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

// TestPattern checks how a psql pattern splits and folds: at the first dot
// outside double quotes, with the text outside them folded as the product
// folds a name, and * and ? made LIKE wildcards. See D169.
func TestPattern(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		dialect      dbmeta.Dialect
		in           string
		schema, name string
	}{
		{dbmeta.PostgreSQL, "", "", ""},
		{dbmeta.PostgreSQL, "film*", "", "film%"},
		{dbmeta.PostgreSQL, "Public.Film?", "public", "film_"},
		{dbmeta.PostgreSQL, `"Public"."Film*"`, "Public", "Film*"},
		{dbmeta.PostgreSQL, `"a.b".c`, "a.b", "c"},
		{dbmeta.PostgreSQL, `x."a""b"`, "x", `a"b`},
		{dbmeta.PostgreSQL, "a.b.c", "a", "b.c"},
		{dbmeta.PostgreSQL, `my"Table"x`, "", "myTablex"},
		{dbmeta.Oracle, "hr.emp*", "HR", "EMP%"},
		{dbmeta.Oracle, `hr."emp"`, "HR", "emp"},
		{dbmeta.MySQL, "Shop.Film*", "Shop", "Film%"},
		{dbmeta.Dialect("no such dialect"), "A.b*", "A", "b%"},
	} {
		schema, name := c.dialect.Pattern(c.in)
		if schema != c.schema || name != c.name {
			t.Errorf("%s: Pattern(%q) = %q, %q, want %q, %q", c.dialect, c.in, schema, name, c.schema, c.name)
		}
	}
}
