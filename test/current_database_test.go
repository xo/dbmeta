package test

import (
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/xo/dbmeta"
)

// TestCurrentDatabaseIsMeasured reads the current database from a connection,
// and then after a statement that changes it (D265).
func TestCurrentDatabaseIsMeasured(t *testing.T) {
	for _, tt := range []struct {
		dialect dbmeta.Dialect
		env     string
		dsn     string
		driver  string
		// want is the name at the start, and use is the statement that changes
		// it, with the name after it. A name that is empty is absent. dbrun gives
		// MySQL a URL with no database. Oracle answers the pluggable database that the service names, and 11g its own name, XE.
		want    string
		use     string
		wantUse string
	}{
		{dbmeta.PostgreSQL, "DBMETA_POSTGRES", "", "pgx", "postgres", "", ""},
		{dbmeta.SQLServer, "DBMETA_SQLSERVER", "", "sqlserver", "master", "USE tempdb", "tempdb"},
		{dbmeta.MySQL, "DBMETA_MYSQL", "", "mysql", "", "USE information_schema", "information_schema"},
		{dbmeta.Oracle, "DBMETA_ORACLE", "", "oracle", "service", "", ""},
		{dbmeta.SQLite3, "", ":memory:", "sqlite3", "main", "", ""},
		{dbmeta.SQLite3, "", ":memory:", "sqlite", "main", "", ""},
		{dbmeta.DuckDB, "", "", "duckdb", "memory", "", ""},
	} {
		t.Run(string(tt.dialect)+"/"+tt.driver, func(t *testing.T) {
			dsn := tt.dsn
			if tt.env != "" {
				if dsn = os.Getenv(tt.env); dsn == "" {
					t.Skipf("set %s to run against a server", tt.env)
				}
			}
			ctx := t.Context()
			db := openAt(t, tt.driver, dsn)
			conn, err := db.Conn(ctx)
			if err != nil {
				t.Fatalf("connecting: %v", err)
			}
			defer conn.Close()
			m, err := dbmeta.Open(ctx, tt.dialect, conn)
			if err != nil {
				t.Fatalf("opening the meta: %v", err)
			}
			read := func(want string) {
				t.Helper()
				v, ok, err := dbmeta.First(dbmeta.CurrentDatabase.All(ctx, m, conn, nil))
				if err != nil {
					t.Fatalf("reading the current database: %v", err)
				}
				if !ok || v.Name.V != want || v.Name.Valid != (want != "") {
					t.Errorf("got %q valid %t and row %t, want %q", v.Name.V, v.Name.Valid, ok, want)
				}
			}
			want := tt.want
			if want == "service" {
				// Oracle names the pluggable database of the service in the URL,
				// which is XEPDB1, FREEPDB1 or the like, and the database XE
				// on 11g, which has no pluggable database.
				u, err := url.Parse(dsn)
				if err != nil {
					t.Fatalf("reading the URL: %v", err)
				}
				want = strings.ToUpper(strings.TrimPrefix(u.Path, "/"))
			}
			read(want)
			if tt.use != "" {
				if _, err := conn.ExecContext(ctx, tt.use); err != nil {
					t.Fatalf("%s: %v", tt.use, err)
				}
				read(tt.wantUse)
			}
		})
	}
}
