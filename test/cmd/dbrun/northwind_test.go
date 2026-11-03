package main

import (
	"strings"
	"testing"

	"github.com/xo/dbmeta"
)

// TestSplitStatementsKeepsWhatIsQuoted holds the splitter to the forms that
// the files of the fixture use.
func TestSplitStatementsKeepsWhatIsQuoted(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name          string
		sql           string
		blocks        bool
		dollar        bool
		want          []string
		wantStatement int
	}{
		{"two statements", "select 1; select 2;", false, false, []string{"select 1", "select 2"}, 0},
		{"a quote", "select 'a;b'; select 2", false, false, []string{"select 'a;b'", "select 2"}, 0},
		{"a doubled quote", "select 'it''s; fine'", false, false, []string{"select 'it''s; fine'"}, 0},
		{"a comment", "-- a; b\nselect 1; /* c; d */ select 2", false, false, []string{"-- a; b\nselect 1", "/* c; d */ select 2"}, 0},
		{"only a comment", "-- nothing\n;", false, false, nil, 0},
		{"dollar quotes", "create function f() returns int as $$ select 1; select 2; $$; select 3", false, true,
			[]string{"create function f() returns int as $$ select 1; select 2; $$", "select 3"}, 0},
		{"a tagged dollar quote", "do $body$ begin null; end $body$; select 1", false, true,
			[]string{"do $body$ begin null; end $body$", "select 1"}, 0},
		{"a bracket name", "select [a;b] from t; select 2", true, false, []string{"select [a;b] from t", "select 2"}, 0},
		{"a block", "create procedure p as begin select 1; select 2; end; select 3", true, false,
			[]string{"create procedure p as begin select 1; select 2; end", "select 3"}, 0},
		{"a case inside a block", "create function f() returns int as begin return (case when 1 = 1 then 1 else 2 end); end; select 3", true, false,
			[]string{"create function f() returns int as begin return (case when 1 = 1 then 1 else 2 end); end", "select 3"}, 0},
		{"a transaction is not a block", "begin transaction; select 1; commit; select 2", true, false,
			[]string{"begin transaction", "select 1", "commit", "select 2"}, 0},
		{"a set joins the next statement", "SET identity_insert [T] on\nINSERT INTO [T] VALUES (1);", true, false,
			[]string{"SET identity_insert [T] on\nINSERT INTO [T] VALUES (1)"}, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got := splitStatements(c.sql, c.blocks, c.dollar)
			if strings.Join(got, "|") != strings.Join(c.want, "|") {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}

// TestTheNorthwindFilesSplitIntoWholeStatements splits each embedded file and
// checks the pieces: none is empty, and none ends in a semicolon.
func TestTheNorthwindFilesSplitIntoWholeStatements(t *testing.T) {
	t.Parallel()
	for d, src := range northwindSources {
		for _, name := range src.files {
			b, err := northwindFiles.ReadFile(name)
			if err != nil {
				t.Fatal(err)
			}
			got := splitStatements(string(b), src.blocks, d == dbmeta.PostgreSQL)
			if len(got) < 20 {
				t.Errorf("%s: %d statements, and the expected count is above 20", name, len(got))
			}
			for _, s := range got {
				if s == "" || strings.HasSuffix(s, ";") {
					t.Errorf("%s: a statement is empty or ends in a semicolon: %q", name, firstLines(s, 3))
				}
			}
		}
	}
	if got := len(northwindSources); got != 2 {
		t.Errorf("the fixture is for %d products, and the expected count is 2", got)
	}
}

// TestIdentityInsertIsTurnedOffBeforeAnotherTable holds that SQL Server sees
// one table at a time with identity insert on.
func TestIdentityInsertIsTurnedOffBeforeAnotherTable(t *testing.T) {
	t.Parallel()
	var it identityTracker
	if got := it.steps("SET identity_insert [A] on\nINSERT INTO [A] VALUES (1)"); len(got) != 1 {
		t.Errorf("the first table: %q", got)
	}
	if got := it.steps("SET identity_insert [a] on\nINSERT INTO [a] VALUES (2)"); len(got) != 1 {
		t.Errorf("the same table in another case: %q", got)
	}
	got := it.steps("SET identity_insert [B] on\nINSERT INTO [B] VALUES (1)")
	if len(got) != 2 || got[0] != "SET IDENTITY_INSERT [A] OFF" {
		t.Errorf("another table: %q", got)
	}
	if got := it.steps("select 1"); len(got) != 1 {
		t.Errorf("a plain statement: %q", got)
	}
}
