package test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"gopkg.in/inf.v0"

	"github.com/xo/dbmeta"
	"github.com/xo/dbmeta/dump"
	_ "github.com/xo/dbmeta/models/avatica"
	_ "github.com/xo/dbmeta/models/cassandra"
	_ "github.com/xo/dbmeta/models/clickhouse"
	_ "github.com/xo/dbmeta/models/cratedb"
	_ "github.com/xo/dbmeta/models/databend"
	_ "github.com/xo/dbmeta/models/exasol"
	_ "github.com/xo/dbmeta/models/firebird"
	_ "github.com/xo/dbmeta/models/hana"
	_ "github.com/xo/dbmeta/models/questdb"
	_ "github.com/xo/dbmeta/models/spanner"
	_ "github.com/xo/dbmeta/models/vertica"
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
		if stem == "firebird" {
			table = `"basic"`
		}
		var pre []string
		if stem == "postgres" {
			pre = []string{"CREATE SCHEMA dump_src"}
			table = "dump_src.basic"
		}
		if stem == "avatica" {
			table = `"basic"`
		}
		if stem == "exasol" || stem == "hana" {
			pre = []string{`CREATE SCHEMA "dump_src"`}
			table = `"dump_src"."basic"`
		}
		if stem == "cassandra" {
			pre = []string{"CREATE KEYSPACE dump_src WITH replication = {'class': 'SimpleStrategy', 'replication_factor': 1}"}
			table = "dump_src.basic"
		}
		if stem == "vertica" {
			pre = []string{"CREATE SCHEMA dump_src"}
			table = "dump_src.basic"
		}
		if stem == "clickhouse" || stem == "databend" {
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
			"firebird":   `"id" INTEGER NOT NULL PRIMARY KEY, "name" VARCHAR(20) CHARACTER SET UTF8, "amount" DECIMAL(10,2), "flag" BOOLEAN, "made" TIMESTAMP, "score" DOUBLE PRECISION, "note" BLOB SUB_TYPE TEXT CHARACTER SET UTF8`,
			"avatica":    `"id" INTEGER NOT NULL PRIMARY KEY, "name" VARCHAR(20), "amount" DECIMAL(10,2), "flag" BOOLEAN, "made" TIMESTAMP(6), "score" DOUBLE, "note" LONGVARCHAR`,
			"exasol":     `"id" DECIMAL(10,0) PRIMARY KEY, "name" VARCHAR(20) UTF8, "amount" DECIMAL(10,2), "flag" BOOLEAN, "made" TIMESTAMP(6), "score" DOUBLE PRECISION, "note" VARCHAR(2000000) UTF8`,
			"hana":       `"id" INTEGER PRIMARY KEY, "name" NVARCHAR(20), "amount" DECIMAL(10,2), "flag" BOOLEAN, "made" TIMESTAMP, "score" DOUBLE, "note" NCLOB`,
			"cassandra":  "id int PRIMARY KEY, name text, amount decimal, flag boolean, made timestamp, score double, note text",
			"databend":   "id INT NOT NULL, name VARCHAR, amount DECIMAL(10,2), flag BOOLEAN, made TIMESTAMP, score DOUBLE, note VARCHAR",
			"vertica":    "id INT NOT NULL PRIMARY KEY, name VARCHAR(80), amount NUMERIC(10,2), flag BOOLEAN, made TIMESTAMP, score FLOAT, note LONG VARCHAR",
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
	fb := crossSide{name: "firebird", dialect: dbmeta.Firebird, shared: true,
		open: func(t *testing.T, _ string) (*sql.DB, dump.Filter) {
			db := openFirebird(t)
			// The driver of Firebird commits and keeps the transaction open, and a table that a
			// transaction has used cannot be dropped, so the idle connections close first.
			drop := func() {
				db.SetMaxIdleConns(0)
				_, _ = db.ExecContext(context.Background(), `DROP TABLE "basic"`)
			}
			drop()
			t.Cleanup(drop)
			return db, dump.Filter{Tables: []string{"basic"}}
		}}
	dbend := crossSide{name: "databend", dialect: dbmeta.Databend,
		open: func(t *testing.T, role string) (*sql.DB, dump.Filter) {
			db := openDatabend(t)
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
	vert := crossSide{name: "vertica", dialect: dbmeta.Vertica,
		open: func(t *testing.T, role string) (*sql.DB, dump.Filter) {
			db := openVertica(t)
			drop := func() {
				for _, s := range []string{"DROP SCHEMA IF EXISTS dump_src CASCADE", "DROP SCHEMA IF EXISTS dump_cross CASCADE"} {
					_, _ = db.ExecContext(context.Background(), s)
				}
			}
			drop()
			t.Cleanup(drop)
			schema := map[string]string{"src": "dump_src", "dst": "dump_cross"}[role]
			return db, dump.Filter{Schemas: []string{schema}, Tables: []string{"basic"}}
		}}
	av := crossSide{name: "avatica", dialect: dbmeta.Avatica,
		open: func(t *testing.T, role string) (*sql.DB, dump.Filter) {
			db := openAvatica(t)
			drop := func() {
				for _, s := range []string{`DROP TABLE IF EXISTS "basic"`, `DROP SCHEMA IF EXISTS "dump_cross" CASCADE`} {
					_, _ = db.ExecContext(context.Background(), s)
				}
			}
			drop()
			t.Cleanup(drop)
			schema := map[string]string{"src": "PUBLIC", "dst": "dump_cross"}[role]
			return db, dump.Filter{Schemas: []string{schema}, Tables: []string{"basic"}}
		}}
	schemaSide := func(name string, dialect dbmeta.Dialect, opener func(*testing.T) *sql.DB, drop string) crossSide {
		return crossSide{name: name, dialect: dialect,
			open: func(t *testing.T, role string) (*sql.DB, dump.Filter) {
				db := opener(t)
				clean := func() {
					for _, s := range []string{drop + ` "dump_src" CASCADE`, drop + ` "dump_cross" CASCADE`} {
						_, _ = db.ExecContext(context.Background(), s)
					}
				}
				clean()
				t.Cleanup(clean)
				schema := map[string]string{"src": "dump_src", "dst": "dump_cross"}[role]
				return db, dump.Filter{Schemas: []string{schema}, Tables: []string{"basic"}}
			}}
	}
	exa := schemaSide("exasol", dbmeta.Exasol, openExasol, "DROP SCHEMA IF EXISTS")
	hdb := schemaSide("hana", dbmeta.HANA, openHANA, "DROP SCHEMA")
	cass := crossSide{name: "cassandra", dialect: dbmeta.Cassandra,
		open: func(t *testing.T, role string) (*sql.DB, dump.Filter) {
			db := openCassandra(t)
			drop := func() {
				for _, s := range []string{"DROP KEYSPACE IF EXISTS dump_src", "DROP KEYSPACE IF EXISTS dump_cross"} {
					_, _ = db.ExecContext(context.Background(), s)
				}
			}
			drop()
			t.Cleanup(drop)
			schema := map[string]string{"src": "dump_src", "dst": "dump_cross"}[role]
			return db, dump.Filter{Schemas: []string{schema}, Tables: []string{"basic"}}
		}}
	out = append(out,
		basicSide{crossSide: exa, create: basicCreate("exasol"), schema: func(role string) string {
			return map[string]string{"src": "dump_src", "dst": "dump_cross"}[role]
		}},
		basicSide{crossSide: hdb, create: basicCreate("hana"), schema: func(role string) string {
			return map[string]string{"src": "dump_src", "dst": "dump_cross"}[role]
		}},
		basicSide{crossSide: av, create: basicCreate("avatica"), schema: func(role string) string {
			return map[string]string{"src": "PUBLIC", "dst": "dump_cross"}[role]
		}},
		basicSide{crossSide: cass, create: basicCreate("cassandra"), schema: func(role string) string {
			return map[string]string{"src": "dump_src", "dst": "dump_cross"}[role]
		}},
		basicSide{crossSide: fb, create: basicCreate("firebird"), schema: func(string) string { return "" }},
		basicSide{crossSide: dbend, create: basicCreate("databend"), schema: func(role string) string {
			return map[string]string{"src": "dump_src", "dst": "dump_cross"}[role]
		}},
		basicSide{crossSide: vert, create: basicCreate("vertica"), schema: func(role string) string {
			return map[string]string{"src": "dump_src", "dst": "dump_cross"}[role]
		}},
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
	if s.dialect == dbmeta.Firebird {
		return `"basic"`
	}
	if s.quoted() && s.dialect != dbmeta.Firebird {
		return `"` + s.schema(role) + `"."basic"`
	}
	if schema := s.schema(role); schema != "" {
		return schema + ".basic"
	}
	return "basic"
}

// orderBy orders the rows of a read by the key. A table of Cassandra cannot be ordered by
// its partition key, so its rows are sorted by the test.
func orderBy(s basicSide, col string) string {
	if s.dialect == dbmeta.Cassandra {
		return ""
	}
	return " ORDER BY " + quoteAll(s, []string{col})[0]
}

// sortRows puts the rows in the order of the first column, which is an integer key.
func sortRows(s basicSide, rows [][]any) [][]any {
	if s.dialect == dbmeta.Cassandra {
		slices.SortFunc(rows, func(a, b []any) int {
			x, _ := strconv.ParseInt(fmt.Sprint(a[0]), 10, 64)
			y, _ := strconv.ParseInt(fmt.Sprint(b[0]), 10, 64)
			return int(x - y)
		})
	}
	return rows
}

// quoted says that the product makes the case of a name that is not quoted upper, so the
// test quotes every name. The script quotes every name, so the table of a copy has lower
// case names.
func (s basicSide) quoted() bool {
	switch s.dialect {
	case dbmeta.Firebird, dbmeta.Avatica, dbmeta.Exasol, dbmeta.HANA:
		return true
	}
	return false
}

// quoteAll quotes the names of columns for a product that keeps the case of a name that is
// quoted. The script quotes every name, so the table of a copy has lower case names, and
// Firebird folds the case of a name that is not quoted.
func quoteAll(s basicSide, names []string) []string {
	if !s.quoted() {
		return names
	}
	out := make([]string, len(names))
	for i, n := range names {
		out[i] = n
		if !strings.Contains(n, "(") {
			out[i] = `"` + n + `"`
		}
	}
	return out
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
func basicCells(t *testing.T, db *sql.DB, s basicSide, role string, blankIsNull bool) string {
	t.Helper()
	var lines []string
	for _, row := range sortRows(s, readCells(t, db, fresh("SELECT ")+strings.Join(quoteAll(s, sourceColumns(s, basicColumns)), ", ")+" FROM "+s.qualified(role)+orderBy(s, "id"))) {
		parts := make([]string, len(row))
		for i, v := range row {
			// The driver of Firebird reads a timestamp with no zone as a time in the local
			// zone of the machine, and the column holds the wall clock that was written, in UTC.
			if at, ok := v.(time.Time); ok && s.dialect == dbmeta.Firebird {
				v = time.Date(at.Year(), at.Month(), at.Day(), at.Hour(), at.Minute(), at.Second(), at.Nanosecond(), time.UTC)
			}
			// Exasol keeps an empty string as NULL.
			if b, ok := v.([]byte); ok && blankIsNull {
				v = string(b)
			}
			if text, ok := v.(string); ok && text == "" && blankIsNull {
				v = nil
			}
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
	sides := basicSides()
	for _, src := range sides {
		for _, dst := range sides {
			if src.name == dst.name {
				continue
			}
			for _, form := range []string{"literal", "args"} {
				t.Run(src.name+" to "+dst.name+" "+form, func(t *testing.T) {
					if (dst.dialect == dbmeta.Spanner || dst.dialect == dbmeta.Cassandra) && (src.dialect == dbmeta.QuestDB || src.dialect == dbmeta.Databend) {
						t.Skip("the target needs a key, and this product has none")
					}
					// A decimal of Cassandra has no digits, so a product that wants them takes
					// the text of the number.
					policy := dump.Policy{Default: dump.Allow{Lossy: true, Text: src.dialect == dbmeta.Cassandra}}
					srcDB, srcFilter := src.open(t, "src")
					srcFilter.Tables = []string{"basic"}
					for _, s := range src.create() {
						mustExec(t, srcDB, s)
					}
					insertValues(t, srcDB, src.dialect, src.qualified("src"), quoteAll(src, basicColumns), rowsFor(src))
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
					cells := sortRows(src, readCells(t, srcDB, fresh("SELECT ")+strings.Join(quoteAll(src, sourceColumns(src, l.Columns())), ", ")+" FROM "+srcTable+orderBy(src, l.Columns()[0])))
					conn, err := dstDB.Conn(t.Context())
					if err != nil {
						t.Fatalf("taking a connection: %v", err)
					}
					// A pool of one connection, as Exasol has, needs this one back before the read.
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
						if errors.Is(err, dump.ErrNoEquivalent) {
							t.Skipf("the product cannot bind a column of this table: %v", err)
						}
						if err != nil {
							t.Fatalf("converting the rows: %v", err)
						}
						if _, err := conn.ExecContext(t.Context(), l.InsertArgs(len(batch)), args...); err != nil {
							t.Fatalf("%s: %v", l.InsertArgs(len(batch)), err)
						}
					}
					_ = conn.Close()
					settle(t, dstDB, dst, "dst", len(basicRows))
					_ = dstFilter
					blank := src.dialect == dbmeta.Exasol || dst.dialect == dbmeta.Exasol
					want := basicCells(t, srcDB, src, "src", blank)
					got := basicCells(t, dstDB, dst, "dst", blank)
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
	if s.dialect == dbmeta.Cassandra {
		// The driver of Cassandra takes a decimal as an inf.Dec only.
		out := make([][]any, len(basicRows))
		for i, row := range basicRows {
			out[i] = slices.Clone(row)
			if text, ok := row[2].(string); ok {
				d, _ := new(inf.Dec).SetString(text)
				out[i][2] = d
			}
		}
		return out
	}
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
	// The drivers of Spanner and HANA give a decimal as a value of their own, so the test
	// reads it as text. A key of HANA can be a decimal when the table came from Exasol.
	cast := map[dbmeta.Dialect]map[string]string{
		dbmeta.Spanner: {"amount": "CAST(amount AS STRING) AS amount"},
		dbmeta.HANA:    {"amount": `CAST("amount" AS NVARCHAR(100)) AS "amount"`, "id": `CAST("id" AS BIGINT) AS "id"`},
	}[s.dialect]
	if cast == nil {
		return cols
	}
	out := slices.Clone(cols)
	for i, c := range out {
		if expr, ok := cast[strings.Trim(c, `"`)]; ok {
			out[i] = expr
		}
	}
	return out
}
