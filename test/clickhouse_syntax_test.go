package test

import (
	"os"
	"testing"

	"github.com/xo/dbmeta"
)

// TestClickHouseLexicalFormsAreMeasured sends each form that a client must
// know to a ClickHouse server and holds the model to the answer. A semicolon
// inside a name between backticks or inside a comment must not end the
// statement, and the server shows which forms it reads that way.
//
// ClickHouse reads a comment from a hash only when a space or an exclamation
// mark follows it, so the model records the flag as true and a client that
// reads every hash as a comment is wider than the server. See D250.
func TestClickHouseLexicalFormsAreMeasured(t *testing.T) {
	dsn := os.Getenv("DBMETA_CLICKHOUSE")
	if dsn == "" {
		t.Skip("set DBMETA_CLICKHOUSE to run against a server")
	}
	syntax, ok := dbmeta.ClickHouse.Syntax()
	if !ok {
		t.Fatal("clickhouse has no model")
	}
	db := openAt(t, "clickhouse", dsn)
	cases := []struct {
		name   string
		query  string
		flag   bool
		accept bool
	}{
		{name: "backticks", query: "select 1 as `a;b`", flag: syntax.Backticks, accept: true},
		{name: "block comment", query: "select 1 /* a; b */", flag: syntax.BlockComments, accept: true},
		{name: "slash comment", query: "select 1 // a; b\n", flag: syntax.SlashComments, accept: true},
		{name: "hash and space", query: "select 1 # a; b\n", flag: syntax.HashComments, accept: true},
		{name: "hash and bang", query: "select 1 #! a; b\n", flag: syntax.HashComments, accept: true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var v any
			err := db.QueryRowContext(t.Context(), c.query).Scan(&v)
			if (err == nil) != c.accept {
				t.Errorf("the server accepts %q is %t (%v), and the expected value is %t", c.query, err == nil, err, c.accept)
			}
			if c.flag != c.accept {
				t.Errorf("the model says %t, and the server says %t", c.flag, c.accept)
			}
		})
	}
	// A hash with no space after it is not a comment, and the server refuses it.
	var v any
	if err := db.QueryRowContext(t.Context(), "select 1 #a\n").Scan(&v); err == nil {
		t.Error("the server accepted a hash with no space after it")
	}
}
