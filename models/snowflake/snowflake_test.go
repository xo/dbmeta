package snowflake

import (
	"errors"
	"strings"
	"testing"

	"github.com/xo/dbmeta"
)

// TestLiteral guards the one thing that this dialect does that most do not:
// it writes every value into the statement. See D203.
func TestLiteral(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name string
		in   any
		want string
	}{
		{"plain", "AUTHOR", `'AUTHOR'`},
		{"empty", "", `''`},
		{"pattern", "AUTH%", `'AUTH%'`},
		{"quote", "o'brien", `'o''brien'`},
		{"backslash", `a\b`, `'a\\b'`},
		{"backslash then quote", `a\'b`, `'a\\''b'`},
		{"break out", `x' OR '1'='1`, `'x'' OR ''1''=''1'`},
		{"true", true, `TRUE`},
		{"false", false, `FALSE`},
		{"scope", scope(`SCHEMA "S"`), `SCHEMA "S"`},
	} {
		got, err := literal(c.in)
		if err != nil {
			t.Errorf("%s: %v", c.name, err)
			continue
		}
		if got != c.want {
			t.Errorf("%s: literal(%#v) = %s, want %s", c.name, c.in, got, c.want)
		}
	}
}

// TestLiteralRefuses checks what the function will not write.
func TestLiteralRefuses(t *testing.T) {
	t.Parallel()
	for name, v := range map[string]any{"nul": "a\x00b", "int": 7, "slice": []string{"a"}} {
		if _, err := literal(v); !errors.Is(err, dbmeta.ErrInvalidParam) {
			t.Errorf("%s: expected ErrInvalidParam, got %v", name, err)
		}
	}
}

// TestKeyScope checks the scope that the key statements take.
func TestKeyScope(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name, catalog, schema, parent, want string
	}{
		{"both exact", "", "SALES", "ORDERS", `TABLE "SALES"."ORDERS"`},
		{"all three exact", "DB", "SALES", "ORDERS", `TABLE "DB"."SALES"."ORDERS"`},
		{"exact schema and catalog", "DB", "SALES", "", `SCHEMA "DB"."SALES"`},
		{"pattern table", "DB", "SALES", "ORD%", `SCHEMA "DB"."SALES"`},
		{"table with an underscore", "DB", "SALES", "ORDER_LINES", `SCHEMA "DB"."SALES"`},
		{"exact schema alone", "", "SALES", "", `DATABASE`},
		{"pattern schema", "", "SAL%", "ORDERS", `DATABASE`},
		{"pattern schema, exact catalog", "DB", "SAL%", "ORDERS", `DATABASE "DB"`},
		{"no schema", "", "", "ORDERS", `DATABASE`},
		{"nothing", "", "", "", `DATABASE`},
		{"backslash", "", `SA\_LES`, "ORDERS", `DATABASE`},
		{"lower case is kept", "", "sales", "orders", `TABLE "sales"."orders"`},
		{"a quote is doubled", "", `A"B`, `C"D`, `TABLE "A""B"."C""D"`},
		{"a hostile name", "", `X"; DROP TABLE T; --`, "T", `TABLE "X""; DROP TABLE T; --"."T"`},
	} {
		got := keyScope(map[string]any{"catalog": c.catalog, "schema": c.schema, "parent": c.parent})
		if got != scope(c.want) {
			t.Errorf("%s: keyScope(%q, %q, %q) = %s, want %s", c.name, c.catalog, c.schema, c.parent, got, c.want)
		}
	}
}

// TestKeyStatementsAreScoped builds the two key statements and checks what
// reaches the server: every name is quoted, and a hostile one stays inside its
// quotes.
func TestKeyStatementsAreScoped(t *testing.T) {
	t.Parallel()
	m, err := dbmeta.New(dbmeta.Snowflake, dbmeta.VersionSet{})
	if err != nil {
		t.Fatalf("building the metadata: %v", err)
	}
	args := map[string]any{"schema": `X"; DROP TABLE T; --`, "parent": "O'B"}
	for name, build := range map[string]func(*dbmeta.Meta, map[string]any) (string, []any, error){
		"columns":            dbmeta.Columns.Build,
		"constraint columns": dbmeta.ConstraintColumns.Build,
	} {
		stmt, vals, err := build(m, args)
		if err != nil {
			t.Fatalf("%s: building: %v", name, err)
		}
		if len(vals) != 0 {
			t.Errorf("expected no bind values, got %v", vals)
		}
		if !strings.Contains(stmt, `IN TABLE "X""; DROP TABLE T; --"."O'B" ->>`) {
			t.Errorf("%s: expected the scope with both names quoted, got %s", name, stmt)
		}
		if !strings.Contains(stmt, `LIKE 'O''B'`) {
			t.Errorf("expected the table pattern as a literal with its quote doubled, got %s", stmt)
		}
	}
	// A caller cannot pass the derived value.
	if _, _, err := dbmeta.Columns.Build(m, map[string]any{"scope": "DATABASE"}); !errors.Is(err, dbmeta.ErrUnknownParam) {
		t.Errorf("expected ErrUnknownParam for scope, got %v", err)
	}
}
