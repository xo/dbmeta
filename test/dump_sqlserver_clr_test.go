package test

import (
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/xo/dbmeta"
	"github.com/xo/dbmeta/dump"
)

// sqlServerCLR is a schema with the types that the common language runtime
// supplies, a column that the server numbers, and a system versioned table with
// history that an update made.
var sqlServerCLR = []string{
	"CREATE SCHEMA clr_rt",
	`CREATE TABLE clr_rt.place (
		id int IDENTITY(1, 1) PRIMARY KEY,
		name nvarchar(50) NOT NULL,
		spot geography NULL,
		area geometry NULL,
		node hierarchyid NULL,
		version rowversion
	)`,
	`INSERT INTO clr_rt.place (name, spot, area, node) VALUES
		(N'one', geography::Point(47.65, -122.34, 4326), geometry::STGeomFromText('POLYGON((0 0, 4 0, 4 4, 0 4, 0 0))', 0), hierarchyid::Parse('/1/2/')),
		(N'two', NULL, geometry::Point(1, 2, 0), hierarchyid::GetRoot())`,
	`CREATE TABLE clr_rt.price (
		id int NOT NULL PRIMARY KEY,
		amount money NOT NULL,
		valid_from datetime2 GENERATED ALWAYS AS ROW START NOT NULL,
		valid_to datetime2 GENERATED ALWAYS AS ROW END NOT NULL,
		PERIOD FOR SYSTEM_TIME (valid_from, valid_to)
	) WITH (SYSTEM_VERSIONING = ON (HISTORY_TABLE = clr_rt.price_history))`,
	"INSERT INTO clr_rt.price (id, amount) VALUES (1, 10), (2, 20), (3, 30)",
	"UPDATE clr_rt.price SET amount = amount * 2 WHERE id <= 2",
	"DELETE FROM clr_rt.price WHERE id = 3",
}

// sqlServerCLRChecks are the queries that must answer the same on both sides.
var sqlServerCLRChecks = []string{
	"SELECT id, name, spot.STAsText(), spot.STSrid, area.STAsText(), node.ToString() FROM clr_rt.place ORDER BY id",
	"SELECT id, amount, valid_from, valid_to FROM clr_rt.price ORDER BY id",
	"SELECT id, amount, valid_from, valid_to FROM clr_rt.price_history ORDER BY id, valid_from",
}

// TestSQLServerCopiesSpatialValuesAndHistory copies the schema and the rows, in both
// forms that a client writes them, and compares what a reader sees.
func TestSQLServerCopiesSpatialValuesAndHistory(t *testing.T) {
	db := openSQLServerNamed(t, "dbmeta_dump_clr")
	m := metaFor(t, dbmeta.SQLServer, db)
	var major int
	if err := db.QueryRowContext(t.Context(), "SELECT CAST(SERVERPROPERTY('ProductMajorVersion') AS int)").Scan(&major); err != nil {
		t.Fatalf("reading the version: %v", err)
	}
	if major < 13 {
		t.Skip("system versioned tables came in SQL Server 2016")
	}
	for _, s := range sqlServerCLR {
		mustExec(t, db, s)
	}
	filter := dump.Filter{Schemas: []string{"clr_rt"}}
	snap, err := dump.Read(t.Context(), m, db, filter)
	if err != nil {
		t.Fatalf("reading: %v", err)
	}
	for _, sk := range snap.Skipped {
		t.Errorf("skipped %s.%s: %s: %s", sk.Schema, sk.Name, sk.Kind, sk.Reason)
	}
	var price *dump.Table
	for i := range snap.Schemas[0].Tables {
		if tb := &snap.Schemas[0].Tables[i]; tb.Name == "price" {
			price = tb
		}
	}
	if price == nil || price.Versioning == nil {
		t.Fatalf("the table price has no versioning: %+v", price)
	}
	want := dump.Versioning{Schema: "clr_rt", Table: "price_history", Start: "valid_from", End: "valid_to"}
	if *price.Versioning != want {
		t.Errorf("versioning is %+v, want %+v", *price.Versioning, want)
	}
	for _, form := range []string{"literal", "args"} {
		t.Run(form, func(t *testing.T) {
			copyDB := openSQLServerNamed(t, "dbmeta_dump_clr_"+form)
			copyWithRows(t, db, copyDB, snap, form)
			for _, q := range sqlServerCLRChecks {
				if a, b := readCells(t, db, q), readCells(t, copyDB, q); !equalCells(a, b) {
					t.Errorf("%s\nsource: %v\ncopy:   %v", q, a, b)
				}
			}
			var typ int
			var history string
			if err := copyDB.QueryRowContext(t.Context(), "SELECT t.temporal_type, h.name FROM sys.tables t JOIN sys.tables h ON h.object_id = t.history_table_id WHERE t.name = 'price'").Scan(&typ, &history); err != nil {
				t.Fatalf("reading the versioning of the copy: %v", err)
			}
			if typ != 2 || history != "price_history" {
				t.Errorf("the copy has temporal type %d and history table %q", typ, history)
			}
		})
	}
}

// copyWithRows runs a script on the target, with the rows of every table that is
// not a partition loaded between the stages, as a client does.
func copyWithRows(t *testing.T, src, dst *sql.DB, snap *dump.Snapshot, form string) {
	t.Helper()
	stmts := scriptOf(t, snap, dbmeta.SQLServer)
	runStage := func(stage dump.Stage) {
		var run []dump.Statement
		for _, st := range stmts {
			if st.Stage == stage {
				run = append(run, st)
			}
		}
		for _, st := range run {
			if st.SQL == "" {
				continue
			}
			if _, err := dst.ExecContext(t.Context(), st.SQL); err != nil {
				t.Fatalf("running the %s %s: %v\n%s", st.Kind, st.Name, err, st.SQL)
			}
		}
	}
	runStage(dump.Pre)
	// The settings of the Data stage hold for the connection that runs them, and
	// ALTER TABLE does not, so one connection takes the settings and the rows.
	conn, err := dst.Conn(t.Context())
	if err != nil {
		t.Fatalf("taking a connection: %v", err)
	}
	defer conn.Close()
	for _, st := range stmts {
		if st.Stage == dump.Data && st.SQL != "" {
			if _, err := conn.ExecContext(t.Context(), st.SQL); err != nil {
				t.Fatalf("%s: %v", st.SQL, err)
			}
		}
	}
	for _, sc := range snap.Schemas {
		for _, tb := range sc.Tables {
			if tb.IsPartition() {
				continue
			}
			l, err := dump.NewLoader(snap, sc.Name, tb.Name, dbmeta.SQLServer, dump.Policy{})
			if err != nil {
				t.Fatalf("preparing the loader of %s.%s: %v", sc.Name, tb.Name, err)
			}
			cells := readCells(t, src, l.SelectAll())
			if p := l.Prologue(); p != "" {
				mustExecConn(t, conn, p)
			}
			for start := 0; start < len(cells); start += l.BatchSize() {
				batch := cells[start:min(start+l.BatchSize(), len(cells))]
				if form == "literal" {
					stmt, err := l.Insert(batch)
					if err != nil {
						t.Fatalf("writing the INSERT of %s.%s: %v", sc.Name, tb.Name, err)
					}
					mustExecConn(t, conn, stmt)
					continue
				}
				args, err := l.Args(batch)
				if err != nil {
					t.Fatalf("converting the rows of %s.%s: %v", sc.Name, tb.Name, err)
				}
				if _, err := conn.ExecContext(t.Context(), l.InsertArgs(len(batch)), args...); err != nil {
					t.Fatalf("%s: %v", l.InsertArgs(len(batch)), err)
				}
			}
			if e := l.Epilogue(); e != "" {
				mustExecConn(t, conn, e)
			}
		}
	}
	runStage(dump.Post)
}

// mustExecConn runs a statement on one connection.
func mustExecConn(t *testing.T, conn *sql.Conn, stmt string) {
	t.Helper()
	if _, err := conn.ExecContext(t.Context(), stmt); err != nil {
		t.Fatalf("%s: %v", stmt, err)
	}
}

// equalCells compares two results by the text of each cell.
func equalCells(a, b [][]any) bool {
	text := func(rows [][]any) string {
		var out strings.Builder
		for _, row := range rows {
			for _, v := range row {
				out.WriteString(fmt.Sprint(v) + "|")
			}
			out.WriteString("\n")
		}
		return out.String()
	}
	return text(a) == text(b)
}

// TestSQLServerCopiesTheNorthwindFixture copies the whole northwind database, when
// the server has it, and requires the same rows in every table. It reads the
// database and writes only to a database of its own.
func TestSQLServerCopiesTheNorthwindFixture(t *testing.T) {
	admin := openSQLServer(t)
	var n int
	if err := admin.QueryRowContext(t.Context(), "SELECT COUNT(*) FROM sys.databases WHERE name = 'northwind'").Scan(&n); err != nil || n == 0 {
		t.Skip("the server has no northwind database: run dbrun fixture northwind")
	}
	src := openSQLServerAt(t, "northwind")
	m := metaFor(t, dbmeta.SQLServer, src)
	snap, err := dump.Read(t.Context(), m, src, dump.Filter{})
	if err != nil {
		t.Fatalf("reading: %v", err)
	}
	for _, sk := range snap.Skipped {
		t.Logf("skipped %s.%s: %s: %s", sk.Schema, sk.Name, sk.Kind, sk.Reason)
	}
	copyDB := openSQLServerNamed(t, "dbmeta_dump_northwind")
	copyWithRows(t, src, copyDB, snap, "literal")
	for _, sc := range snap.Schemas {
		for _, tb := range sc.Tables {
			l, err := dump.NewLoader(snap, sc.Name, tb.Name, dbmeta.SQLServer, dump.Policy{})
			if err != nil {
				t.Fatalf("preparing the loader of %s.%s: %v", sc.Name, tb.Name, err)
			}
			if a, b := readCells(t, src, l.SelectAll()), readCells(t, copyDB, l.SelectAll()); !equalCells(a, b) {
				t.Errorf("%s.%s has other rows in the copy:\nsource: %v\ncopy:   %v", sc.Name, tb.Name, a, b)
			}
		}
	}
	// The copy has the same objects as the source.
	second, err := dump.Read(t.Context(), metaFor(t, dbmeta.SQLServer, copyDB), copyDB, dump.Filter{})
	if err != nil {
		t.Fatalf("reading the copy: %v", err)
	}
	normalizeViews(snap)
	normalizeViews(second)
	want, err := snap.JSON()
	if err != nil {
		t.Fatal(err)
	}
	got, err := second.JSON()
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Errorf("the copy differs from the source:\n%s", lineDiff(string(want), string(got)))
	}
}

// viewHead is the statement up to the query of a view.
var viewHead = regexp.MustCompile(`(?is)^\s*create\s+view\s+.+?\bas\b\s*`)

// normalizeViews reduces the definition of each view, each routine and each trigger to its query with one space
// between words. The script writes the name of the view with its schema and
// brackets and takes off the line breaks of the head and the closing semicolon, so
// the text of the head and the white space differ and the query does not.
func normalizeViews(s *dump.Snapshot) {
	// The copy is not partitioned, which the source snapshot says.
	s.Skipped = nil
	for i := range s.Schemas {
		for j := range s.Schemas[i].Views {
			v := &s.Schemas[i].Views[j]
			def := viewHead.ReplaceAllString(v.Definition, "")
			def = strings.TrimRight(strings.Join(strings.Fields(def), " "), "; ")
			v.Definition = def
		}
		// The server names the keys and the indexes of a table type, so the names
		// of the copy are other ones.
		for j := range s.Schemas[i].TableTypes {
			tt := &s.Schemas[i].TableTypes[j]
			for k := range tt.Constraints {
				tt.Constraints[k].Name = "generated"
			}
			for k := range tt.Indexes {
				tt.Indexes[k].Name = "generated"
			}
		}
		for j := range s.Schemas[i].Tables {
			for k := range s.Schemas[i].Tables[j].Triggers {
				tg := &s.Schemas[i].Tables[j].Triggers[k]
				tg.Definition = strings.TrimRight(strings.TrimSpace(tg.Definition), "; \n\t")
			}
		}
		// The script takes the closing semicolon off a routine.
		for j := range s.Schemas[i].Routines {
			r := &s.Schemas[i].Routines[j]
			r.Definition = strings.TrimRight(strings.TrimSpace(r.Definition), "; \n\t")
		}
	}
}

// openSQLServerAt opens a database of the server that already exists.
func openSQLServerAt(t *testing.T, name string) *sql.DB {
	t.Helper()
	u, err := url.Parse(os.Getenv("DBMETA_SQLSERVER"))
	if err != nil {
		t.Fatalf("reading DBMETA_SQLSERVER: %v", err)
	}
	q := u.Query()
	q.Set("database", name)
	u.RawQuery = q.Encode()
	db, err := sql.Open("sqlserver", u.String())
	if err != nil {
		t.Fatalf("opening %s: %v", name, err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}
