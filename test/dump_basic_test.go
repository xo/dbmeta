package test

import (
	"context"
	"database/sql"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/xo/dbmeta"
	"github.com/xo/dbmeta/dump"
	_ "github.com/xo/dbmeta/models/clickhouse"
	_ "github.com/xo/dbmeta/models/cratedb"
	_ "github.com/xo/dbmeta/models/questdb"
	_ "github.com/xo/dbmeta/models/spanner"
)

// basicRows are the rows of the table of the basic dump test. The boolean is never NULL,
// because QuestDB has no NULL for one.
var basicRows = [][]any{
	{int64(1), "ann", "12.50", true, time.Date(2024, 2, 29, 13, 14, 15, 0, time.UTC), 1.5, "hello"},
	{int64(2), "it's a \\ test é€", "-0.25", false, time.Date(1999, 12, 31, 23, 59, 59, 0, time.UTC), -2.25, nil},
	{int64(3), "", nil, true, nil, nil, "Unicode: ありがとう"},
}

var (
	basicColumns = []string{"id", "name", "amount", "flag", "made", "score", "note"}
	basicKinds   = []dump.Kind{dump.Integer, dump.String, dump.Decimal, dump.Boolean, dump.Timestamp, dump.Float, dump.String}
)

// queryCount makes the text of each query different. QuestDB keeps a plan for the text of
// a query, and refuses it when the table of that name was made again with other types.
var queryCount atomic.Int64

// fresh begins a query with a comment that no other query has.
func fresh(query string) string {
	return "/* " + strconv.FormatInt(queryCount.Add(1), 10) + " */ " + query
}

// basicSide is a product in the basic dump test: the table that it makes, the schema
// that the table is in, and how it opens.
type basicSide struct {
	crossSide

	create func() []string
	schema func(role string) string
}

// basicCreate is the DDL of the table in the types of each product. Every product has these:
// an integer, text, a decimal, a boolean, a timestamp, a float and a NULL.
func basicCreate(stem string) func() []string {
	return func() []string {
		table := "basic"
		var pre []string
		if stem == "postgres" {
			pre = []string{"CREATE SCHEMA dump_src"}
			table = "dump_src.basic"
		}
		if stem == "clickhouse" {
			pre = []string{"CREATE DATABASE dump_src"}
			table = "dump_src.basic"
		}
		def := map[string]string{
			"postgres":   "id integer PRIMARY KEY, name varchar(20), amount numeric(10,2), flag boolean, made timestamp, score double precision, note text",
			"mysql":      "id int PRIMARY KEY, name varchar(20), amount decimal(10,2), flag tinyint(1), made datetime, score double, note text",
			"sqlite3":    "id integer PRIMARY KEY, name varchar(20), amount decimal(10,2), flag boolean, made datetime, score real, note text",
			"duckdb":     "id integer PRIMARY KEY, name varchar, amount decimal(10,2), flag boolean, made timestamp, score double, note varchar",
			"cratedb":    "id integer PRIMARY KEY, name varchar(20), amount numeric(10,2), flag boolean, made timestamp without time zone, score double precision, note text",
			"clickhouse": "id Int32, name Nullable(String), amount Nullable(Decimal(10, 2)), flag Bool, made Nullable(DateTime64(6)), score Nullable(Float64), note Nullable(String)",
			"spanner":    "id INT64 NOT NULL, name STRING(20), amount NUMERIC, flag BOOL, made TIMESTAMP, score FLOAT64, note STRING(MAX)",
			"questdb":    "id int, name varchar, amount double, flag boolean, made timestamp, score double, note varchar",
		}[stem]
		tail := ""
		switch stem {
		case "clickhouse":
			tail = " ENGINE = MergeTree ORDER BY id"
		case "spanner":
			tail = " PRIMARY KEY (id)"
		}
		return append(pre, "CREATE TABLE "+table+" ("+def+")"+tail)
	}
}

func basicSides() []basicSide {
	byName := map[string]crossSide{}
	for _, s := range crossSides() {
		byName[s.name] = s
	}
	var out []basicSide
	for _, name := range []string{"postgres", "mysql", "duckdb"} {
		s, ok := byName[name]
		if !ok {
			continue
		}
		out = append(out, basicSide{crossSide: s, create: basicCreate(name), schema: func(role string) string {
			switch name {
			case "postgres":
				return map[string]string{"src": "dump_src", "dst": "dump_cross"}[role]
			case "duckdb":
				return map[string]string{"src": "main", "dst": "dump_cross"}[role]
			}
			return ""
		}})
	}
	crate := crossSide{name: "cratedb", dialect: dbmeta.CrateDB, shared: true,
		open: func(t *testing.T, role string) (*sql.DB, dump.Filter) {
			db := openCrateDB(t)
			drop := func() {
				for _, s := range []string{"DROP TABLE IF EXISTS doc.basic", "DROP TABLE IF EXISTS dump_cross.basic"} {
					_, _ = db.ExecContext(context.Background(), s)
				}
			}
			drop()
			t.Cleanup(drop)
			schema := map[string]string{"src": "doc", "dst": "dump_cross"}[role]
			return db, dump.Filter{Schemas: []string{schema}, Tables: []string{"basic"}}
		}}
	quest := crossSide{name: "questdb", dialect: dbmeta.QuestDB, shared: true,
		open: func(t *testing.T, _ string) (*sql.DB, dump.Filter) {
			db := openQuestDB(t)
			drop := func() { _, _ = db.ExecContext(context.Background(), "DROP TABLE IF EXISTS basic") }
			drop()
			t.Cleanup(drop)
			return db, dump.Filter{Schemas: []string{"public"}, Tables: []string{"basic"}}
		}}
	ch := crossSide{name: "clickhouse", dialect: dbmeta.ClickHouse,
		open: func(t *testing.T, role string) (*sql.DB, dump.Filter) {
			db := openClickHouse(t)
			drop := func() {
				for _, s := range []string{"DROP DATABASE IF EXISTS dump_src", "DROP DATABASE IF EXISTS dump_cross"} {
					_, _ = db.ExecContext(context.Background(), s)
				}
			}
			drop()
			t.Cleanup(drop)
			schema := map[string]string{"src": "dump_src", "dst": "dump_cross"}[role]
			return db, dump.Filter{Schemas: []string{schema}, Tables: []string{"basic"}}
		}}
	sp := crossSide{name: "spanner", dialect: dbmeta.Spanner,
		open: func(t *testing.T, role string) (*sql.DB, dump.Filter) {
			db := openSpanner(t)
			drop := func() {
				for _, s := range []string{"DROP TABLE IF EXISTS basic", "DROP TABLE IF EXISTS dump_cross.basic", "DROP SCHEMA IF EXISTS dump_cross"} {
					_, _ = db.ExecContext(context.Background(), s)
				}
			}
			drop()
			t.Cleanup(drop)
			schema := map[string]string{"src": "", "dst": "dump_cross"}[role]
			return db, dump.Filter{Schemas: []string{schema}, Tables: []string{"basic"}}
		}}
	out = append(out,
		basicSide{crossSide: sp, create: basicCreate("spanner"), schema: func(role string) string {
			return map[string]string{"src": "", "dst": "dump_cross"}[role]
		}},
		basicSide{crossSide: ch, create: basicCreate("clickhouse"), schema: func(role string) string {
			return map[string]string{"src": "dump_src", "dst": "dump_cross"}[role]
		}},
		basicSide{crossSide: crate, create: basicCreate("cratedb"), schema: func(role string) string {
			return map[string]string{"src": "doc", "dst": "dump_cross"}[role]
		}},
		basicSide{crossSide: quest, create: basicCreate("questdb"), schema: func(string) string { return "" }},
	)
	return out
}

// qualified names the table of a side for a role.
func (s basicSide) qualified(role string) string {
	if schema := s.schema(role); schema != "" {
		return schema + ".basic"
	}
	return "basic"
}

// settle waits until the rows that were written can be read. CrateDB shows a row after a
// refresh, and QuestDB applies the rows of a table in the background.
func settle(t *testing.T, db *sql.DB, s basicSide, role string, want int) {
	t.Helper()
	switch s.dialect {
	case dbmeta.CrateDB:
		mustExec(t, db, "REFRESH TABLE "+s.qualified(role))
	case dbmeta.QuestDB:
		deadline := time.Now().Add(30 * time.Second)
		for {
			var n int
			if err := db.QueryRowContext(t.Context(), "SELECT count(*) FROM "+s.qualified(role)).Scan(&n); err == nil && n == want {
				return
			}
			if time.Now().After(deadline) {
				t.Fatalf("%s never showed %d rows", s.name, want)
			}
			time.Sleep(100 * time.Millisecond)
		}
	}
}

// basicCells writes the rows of the table as one string.
func basicCells(t *testing.T, db *sql.DB, s basicSide, role string) string {
	t.Helper()
	var lines []string
	for _, row := range readCells(t, db, fresh("SELECT ")+strings.Join(sourceColumns(s, basicColumns), ", ")+" FROM "+s.qualified(role)+" ORDER BY id") {
		parts := make([]string, len(row))
		for i, v := range row {
			parts[i] = canonical(t, v, basicKinds[i])
		}
		lines = append(lines, strings.Join(parts, " | "))
	}
	return strings.Join(lines, "\n")
}

// TestABasicDumpMovesATable makes one table with rows in every product that has no
// need of more than columns and rows, writes the script and the rows for every other
// product, in the two forms, and requires the same rows. It is the dump that every
// product can take: the schema of a table and its data.
func TestABasicDumpMovesATable(t *testing.T) {
	policy := dump.Policy{Default: dump.Allow{Lossy: true}}
	sides := basicSides()
	for _, src := range sides {
		for _, dst := range sides {
			if src.name == dst.name {
				continue
			}
			for _, form := range []string{"literal", "args"} {
				t.Run(src.name+" to "+dst.name+" "+form, func(t *testing.T) {
					if dst.dialect == dbmeta.Spanner && src.dialect == dbmeta.QuestDB {
						t.Skip("Spanner holds one row of a table with no key, and QuestDB has no key")
					}
					srcDB, srcFilter := src.open(t, "src")
					srcFilter.Tables = []string{"basic"}
					for _, s := range src.create() {
						mustExec(t, srcDB, s)
					}
					insertValues(t, srcDB, src.dialect, src.qualified("src"), basicColumns, rowsFor(src))
					settle(t, srcDB, src, "src", len(basicRows))
					snap := snapshotOf(t, src.crossSide, srcDB, srcFilter)

					dstDB, dstFilter := dst.open(t, "dst")
					var stmts []dump.Statement
					for st, err := range dump.Script(snap, dst.dialect, policy) {
						if err != nil {
							t.Fatalf("writing the script: %v", err)
						}
						stmts = append(stmts, st)
					}
					for _, st := range stmts {
						if st.Stage == dump.Pre && st.SQL != "" {
							mustExec(t, dstDB, st.SQL)
						}
					}
					srcTable := src.qualified("src")
					l, err := dump.NewLoader(snap, "dump_cross", "basic", dst.dialect, policy)
					if err != nil {
						t.Fatalf("preparing the loader: %v", err)
					}
					cells := readCells(t, srcDB, fresh("SELECT ")+strings.Join(sourceColumns(src, l.Columns()), ", ")+" FROM "+srcTable+" ORDER BY "+l.Columns()[0])
					conn, err := dstDB.Conn(t.Context())
					if err != nil {
						t.Fatalf("taking a connection: %v", err)
					}
					defer conn.Close()
					for start := 0; start < len(cells); start += l.BatchSize() {
						batch := cells[start:min(start+l.BatchSize(), len(cells))]
						if form == "literal" {
							stmt, err := l.Insert(batch)
							if err != nil {
								t.Fatalf("writing the INSERT: %v", err)
							}
							if _, err := conn.ExecContext(t.Context(), stmt); err != nil {
								t.Fatalf("%s: %v", stmt, err)
							}
							continue
						}
						args, err := l.Args(batch)
						if err != nil {
							t.Fatalf("converting the rows: %v", err)
						}
						if _, err := conn.ExecContext(t.Context(), l.InsertArgs(len(batch)), args...); err != nil {
							t.Fatalf("%s: %v", l.InsertArgs(len(batch)), err)
						}
					}
					settle(t, dstDB, dst, "dst", len(basicRows))
					_ = dstFilter
					want := basicCells(t, srcDB, src, "src")
					got := basicCells(t, dstDB, dst, "dst")
					if want != got {
						t.Errorf("the rows differ.\nsource:\n%s\ncopy:\n%s", want, got)
					}
				})
			}
		}
	}
}

// rowsFor returns the rows for a product. QuestDB sends the type of a DECIMAL as unknown, so
// the table of the test has a double there, and the amounts are numbers.
func rowsFor(s basicSide) [][]any {
	if s.dialect != dbmeta.QuestDB {
		return basicRows
	}
	out := make([][]any, len(basicRows))
	for i, row := range basicRows {
		out[i] = slices.Clone(row)
		if text, ok := row[2].(string); ok {
			f, _ := strconv.ParseFloat(text, 64)
			out[i][2] = f
		}
	}
	return out
}

// sourceColumns names the columns of a read. A NUMERIC of Spanner comes as text, as the
// style of the product reads it.
func sourceColumns(s basicSide, cols []string) []string {
	if s.dialect != dbmeta.Spanner {
		return cols
	}
	out := slices.Clone(cols)
	for i, c := range out {
		if c == "amount" {
			out[i] = "CAST(amount AS STRING) AS amount"
		}
	}
	return out
}
