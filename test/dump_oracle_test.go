package test

import (
	"database/sql"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/xo/dbmeta"
	"github.com/xo/dbmeta/dump"
)

// TestOracleReadsTheNorthwindFixture reads the schemas NORTHWIND and HR, and
// writes the snapshot to the file that DBMETA_DUMP_OUT names, when it is set.
func TestOracleReadsTheNorthwindFixture(t *testing.T) {
	db := openOracle(t)
	m := metaFor(t, dbmeta.Oracle, db)
	snap, err := dump.Read(t.Context(), m, db, dump.Filter{Schemas: []string{"NORTHWIND", "HR"}})
	if err != nil {
		t.Fatalf("reading: %v", err)
	}
	for _, sk := range snap.Skipped {
		t.Logf("skipped %s.%s: %s: %s", sk.Schema, sk.Name, sk.Kind, sk.Reason)
	}
	if out := os.Getenv("DBMETA_DUMP_OUT"); out != "" {
		b, err := snap.JSON()
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(out, b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// renameOracleSchemas gives the schemas of a snapshot other names, in every place that
// names one, so that a copy can live in the database of the source. A name in the
// text of a view or a routine is renamed in the case that it has.
func renameOracleSchemas(t *testing.T, snap *dump.Snapshot, names map[string]string) *dump.Snapshot {
	t.Helper()
	b, err := snap.JSON()
	if err != nil {
		t.Fatal(err)
	}
	text := string(b)
	for from, to := range names {
		re := regexp.MustCompile(`(?i)\b` + from + `\b`)
		text = re.ReplaceAllStringFunc(text, func(m string) string {
			if m == strings.ToUpper(m) {
				return strings.ToUpper(to)
			}
			return strings.ToLower(to)
		})
	}
	out, err := dump.ParseSnapshot([]byte(text))
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// TestOracleCopiesTheNorthwindFixture copies NORTHWIND and HR into the schemas
// NORTHWINDC and HRC of the same database, and requires the same objects and the
// same rows.
func TestOracleCopiesTheNorthwindFixture(t *testing.T) {
	db := openOracle(t)
	m := metaFor(t, dbmeta.Oracle, db)
	var n int
	if err := db.QueryRowContext(t.Context(), "SELECT COUNT(*) FROM all_users WHERE username = 'NORTHWIND'").Scan(&n); err != nil || n == 0 {
		t.Skip("the server has no NORTHWIND schema: run dbrun fixture northwind")
	}
	drop := func() {
		for _, u := range []string{"HRC", "NORTHWINDC"} {
			_, _ = db.ExecContext(t.Context(), "BEGIN EXECUTE IMMEDIATE 'DROP USER "+u+" CASCADE'; EXCEPTION WHEN OTHERS THEN NULL; END;")
		}
	}
	drop()
	t.Cleanup(drop)
	source, err := dump.Read(t.Context(), m, db, dump.Filter{Schemas: []string{"NORTHWIND", "HR"}})
	if err != nil {
		t.Fatalf("reading: %v", err)
	}
	for _, sk := range source.Skipped {
		t.Logf("skipped %s.%s: %s: %s", sk.Schema, sk.Name, sk.Kind, sk.Reason)
	}
	renamed := renameOracleSchemas(t, source, map[string]string{"NORTHWIND": "NORTHWINDC", "HR": "HRC"})
	copyOracleWithRows(t, db, db, source, renamed)
	// The copy has the same rows.
	for i, sc := range source.Schemas {
		for j, tb := range sc.Tables {
			if tb.IsPartition() {
				continue
			}
			from, err := dump.NewLoader(source, sc.Name, tb.Name, dbmeta.Oracle, dump.Policy{})
			if err != nil {
				t.Fatal(err)
			}
			to, err := dump.NewLoader(renamed, renamed.Schemas[i].Name, renamed.Schemas[i].Tables[j].Name, dbmeta.Oracle, dump.Policy{})
			if err != nil {
				t.Fatal(err)
			}
			a := readCells(t, db, from.SelectAll())
			b := readCells(t, db, to.SelectAll())
			for _, rows := range [][][]any{a, b} {
				for _, row := range rows {
					for k, v := range row {
						if text, ok := v.(string); ok {
							for _, pair := range [][2]string{{`"HRC".`, `"HR".`}, {`"NORTHWINDC".`, `"NORTHWIND".`}} {
								text = strings.ReplaceAll(text, pair[0], pair[1])
							}
							row[k] = text
						}
					}
				}
			}
			if !equalCells(a, b) {
				t.Errorf("%s.%s has other rows in the copy: %d in the source and %d in the copy", sc.Name, tb.Name, len(a), len(b))
			}
		}
	}
	// The copy has the same objects as the source.
	second, err := dump.Read(t.Context(), m, db, dump.Filter{Schemas: []string{"NORTHWINDC", "HRC"}})
	if err != nil {
		t.Fatalf("reading the copy: %v", err)
	}
	renamed.Skipped, second.Skipped = nil, nil
	want, err := renamed.JSON()
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

// copyOracleWithRows runs the script of a renamed snapshot, and moves the rows of
// every table from the source schemas to the schemas of the copy.
func copyOracleWithRows(t *testing.T, src, dst *sql.DB, source, copied *dump.Snapshot) {
	t.Helper()
	stmts := scriptOf(t, copied, dbmeta.Oracle)
	if out := os.Getenv("DBMETA_SCRIPT_OUT"); out != "" {
		var b strings.Builder
		for _, st := range stmts {
			b.WriteString("-- " + st.Stage.String() + " " + st.Kind + " " + st.Name + " " + st.Note + "\n" + st.SQL + ";\n")
		}
		if err := os.WriteFile(out, []byte(b.String()), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	run := func(stage dump.Stage) {
		for _, st := range stmts {
			if st.Stage != stage || st.SQL == "" {
				continue
			}
			if _, err := dst.ExecContext(t.Context(), st.SQL); err != nil {
				t.Fatalf("running the %s %s: %v\n%s", st.Kind, st.Name, err, st.SQL)
			}
		}
	}
	run(dump.Pre)
	conn, err := dst.Conn(t.Context())
	if err != nil {
		t.Fatalf("taking a connection: %v", err)
	}
	defer conn.Close()
	for _, st := range stmts {
		if st.Stage == dump.Data && st.SQL != "" {
			mustExecConn(t, conn, st.SQL)
		}
	}
	for i, sc := range source.Schemas {
		for j, tb := range sc.Tables {
			if tb.IsPartition() {
				continue
			}
			from, err := dump.NewLoader(source, sc.Name, tb.Name, dbmeta.Oracle, dump.Policy{})
			if err != nil {
				t.Fatalf("preparing the loader of %s.%s: %v", sc.Name, tb.Name, err)
			}
			to, err := dump.NewLoader(copied, copied.Schemas[i].Name, copied.Schemas[i].Tables[j].Name, dbmeta.Oracle, dump.Policy{})
			if err != nil {
				t.Fatalf("preparing the loader of %s.%s: %v", copied.Schemas[i].Name, tb.Name, err)
			}
			cells := readCells(t, src, from.SelectAll())
			for _, row := range cells {
				// The text of an object value names the types of the source.
				for k, v := range row {
					if text, ok := v.(string); ok && strings.Contains(text, `"`) {
						for _, pair := range [][2]string{{`"HR".`, `"HRC".`}, {`"NORTHWIND".`, `"NORTHWINDC".`}} {
							text = strings.ReplaceAll(text, pair[0], pair[1])
						}
						row[k] = text
					}
				}
				stmt, err := to.Insert([][]any{row})
				if err != nil {
					t.Fatalf("writing the INSERT of %s.%s: %v", sc.Name, tb.Name, err)
				}
				mustExecConn(t, conn, stmt)
			}
		}
	}
	run(dump.Post)
}
