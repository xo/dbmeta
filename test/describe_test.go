package test

import (
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/xo/dbmeta"
)

// TestSQLServerDescribesAResultWithoutRunningIt runs the statement that
// DescribeResult returns, and checks the names and types it gives and that
// the described query did not run. See D273.
func TestSQLServerDescribesAResultWithoutRunningIt(t *testing.T) {
	db := openSQLServer(t)
	// The WAITFOR holds the batch for twenty seconds if it runs.
	query := `WAITFOR DELAY '00:00:20'; SELECT 1 AS a, CAST('it''s' AS varchar(10)) AS b, N'x' AS [c d]`
	d, err := dbmeta.SQLServer.DescribeResult(query)
	if err != nil {
		t.Fatalf("describing: %v", err)
	}
	start := time.Now()
	rows, err := db.QueryContext(t.Context(), d.Stmt)
	if err != nil {
		t.Fatalf("running %s: %v", d.Stmt, err)
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		t.Fatalf("reading the columns: %v", err)
	}
	index := map[string]int{}
	for i, c := range cols {
		index[c] = i
	}
	for _, name := range []string{d.Name, d.Type} {
		if _, ok := index[name]; !ok {
			t.Fatalf("the rows have no column %q, they have %v", name, cols)
		}
	}
	type result struct {
		name sql.NullString
		typ  string
	}
	var got []result
	for rows.Next() {
		vals := make([]any, len(cols))
		for i := range vals {
			vals[i] = new(any)
		}
		if err := rows.Scan(vals...); err != nil {
			t.Fatalf("scanning: %v", err)
		}
		var r result
		if v := *(vals[index[d.Name]].(*any)); v != nil {
			r.name = sql.NullString{String: v.(string), Valid: true}
		}
		r.typ, _ = (*(vals[index[d.Type]].(*any))).(string)
		got = append(got, r)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("reading: %v", err)
	}
	if took := time.Since(start); took > 10*time.Second {
		t.Errorf("describing took %s, so the query ran", took)
	}
	want := []result{
		{sql.NullString{String: "a", Valid: true}, "int"},
		{sql.NullString{String: "b", Valid: true}, "varchar(10)"},
		{sql.NullString{String: "c d", Valid: true}, "nvarchar(1)"},
	}
	if len(got) != len(want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("column %d is %+v, want %+v", i+1, got[i], want[i])
		}
	}
}

// TestTheTypesOfAResultAreNamed runs the statement that TypeNames returns on
// every product of the PostgreSQL family that answers, with the numbers that
// pgx reports for integer, varchar(10) and numeric(10,6).
func TestTheTypesOfAResultAreNamed(t *testing.T) {
	eachPostgres(t, func(t *testing.T, db *sql.DB, m *dbmeta.Meta) {
		stmt, err := m.Dialect().TypeNames([]dbmeta.TypeRef{{OID: 23, Modifier: -1}, {OID: 1043, Modifier: 14}, {OID: 1700, Modifier: 655370}})
		if err != nil {
			t.Fatalf("naming: %v", err)
		}
		rows, err := db.QueryContext(t.Context(), stmt)
		if err != nil {
			t.Fatalf("running %s: %v", stmt, err)
		}
		defer rows.Close()
		var got []string
		for rows.Next() {
			var ord int
			var name string
			if err := rows.Scan(&ord, &name); err != nil {
				t.Fatalf("scanning: %v", err)
			}
			if ord != len(got)+1 {
				t.Errorf("row %d has the place %d", len(got)+1, ord)
			}
			got = append(got, name)
		}
		if err := rows.Err(); err != nil {
			t.Fatalf("reading: %v", err)
		}
		want := []string{"integer", "character varying(10)", "numeric(10,6)"}
		if len(got) != len(want) {
			t.Fatalf("got %v, want %v", got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("type %d is %q, want %q", i+1, got[i], want[i])
			}
		}
	})
}

// TestDescribingIsRefusedWhereTheProductCannot checks the error for a product
// that has neither statement, and for an empty request.
func TestDescribingIsRefusedWhereTheProductCannot(t *testing.T) {
	t.Parallel()
	for _, d := range []dbmeta.Dialect{dbmeta.MySQL, dbmeta.Oracle, dbmeta.CrateDB} {
		if _, err := d.DescribeResult("select 1"); !errors.Is(err, dbmeta.ErrNotSupported) {
			t.Errorf("%s DescribeResult gave %v, want ErrNotSupported", d, err)
		}
	}
	for _, d := range []dbmeta.Dialect{dbmeta.MySQL, dbmeta.SQLServer, dbmeta.CrateDB} {
		if _, err := d.TypeNames([]dbmeta.TypeRef{{OID: 23, Modifier: -1}}); !errors.Is(err, dbmeta.ErrNotSupported) {
			t.Errorf("%s TypeNames gave %v, want ErrNotSupported", d, err)
		}
	}
	if _, err := dbmeta.SQLServer.DescribeResult(""); !errors.Is(err, dbmeta.ErrMissingParam) {
		t.Errorf("an empty query gave %v, want ErrMissingParam", err)
	}
	if _, err := dbmeta.PostgreSQL.TypeNames(nil); !errors.Is(err, dbmeta.ErrMissingParam) {
		t.Errorf("no types gave %v, want ErrMissingParam", err)
	}
}
