package test

import (
	"database/sql"
	"errors"
	"regexp"
	"strings"
	"testing"

	"github.com/xo/dbmeta"
)

// The line a person reads on connecting, pinned per product.
//
// Every model builds one, and usql prints it. A review compared all 26
// servers against what usql prints from the same connection. PostgreSQL
// matched on all ten releases and DuckDB matched exactly. dbmeta named the
// product where usql prints a bare number on MySQL and MariaDB, and carried
// the release year and the cumulative update on SQL Server where usql carries
// neither. docs/USQL.md records the table.
//
// Nothing pinned any of it. A model could lose its product name and print a
// bare "8.4.11", which is what usql does today, and no test would notice. This
// is that test.
//
// It checks a shape rather than a string, because the string is the server's
// and changes with every patch release.

var displayShapes = []struct {
	name    string
	dialect dbmeta.Dialect
	open    func(*testing.T) *sql.DB
	// want is the shape of the whole line.
	want *regexp.Regexp
}{
	{
		name: "postgres", dialect: dbmeta.PostgreSQL, open: open,
		want: regexp.MustCompile(`^PostgreSQL \d+(\.\d+)*( \(.*\))?$`),
	},
	{
		// One dialect, two products, and which one is named comes from the
		// suffix the server reports. That is the same detection the queries
		// gate on, so a break here is a break in D44.
		name: "mysql", dialect: dbmeta.MySQL, open: openMySQL,
		want: regexp.MustCompile(`^(MySQL|MariaDB) \d+\.\d+`),
	},
	{
		name: "sqlite3", dialect: dbmeta.SQLite3,
		open: func(t *testing.T) *sql.DB { return openSQLiteWith(t, "sqlite3") },
		want: regexp.MustCompile(`^SQLite \d+\.\d+`),
	},
	{
		// The product is SQLite whichever driver is linked. usql names the
		// driver instead and prints "SQLite3" or "ModernC SQLite" for the same
		// build. dbmeta cannot see the driver and does not guess.
		name: "sqlite3-modernc", dialect: dbmeta.SQLite3,
		open: func(t *testing.T) *sql.DB { return openSQLiteWith(t, "sqlite") },
		want: regexp.MustCompile(`^SQLite \d+\.\d+`),
	},
	{
		name: "duckdb", dialect: dbmeta.DuckDB, open: openDuckDB,
		want: regexp.MustCompile(`^DuckDB v?\d+\.\d+`),
	},
	{
		// The year comes from @@VERSION and the CU from productupdatelevel,
		// and usql reads neither. The year is required rather than optional:
		// written optional, this matched usql's shorter line too and guarded
		// nothing. "2008 R2" is why it is not simply four digits.
		//
		// The CU is not required, because productupdatelevel is NULL on a
		// release older than the one that added it. The exact "RTM-CU27" form
		// is pinned by a table case in models/sqlserver/version_test.go, which
		// needs no server and can state the value.
		name: "sqlserver", dialect: dbmeta.SQLServer, open: openSQLServer,
		want: regexp.MustCompile(
			`^Microsoft SQL Server (19|20)\d\d( R\d)? \d+(\.\d+)+, [^,]+, .*Edition`),
	},
}

// TestTheDisplayLineNamesTheProduct checks every model that can be reached.
func TestTheDisplayLineNamesTheProduct(t *testing.T) {
	var ran int
	for _, c := range displayShapes {
		t.Run(c.name, func(t *testing.T) {
			db := c.open(t)
			versions, err := c.dialect.Version(t.Context(), db)
			if err != nil {
				t.Fatalf("reading the version: %v", err)
			}
			ran++
			got := versions.String()
			if !c.want.MatchString(got) {
				t.Errorf("the display line does not match %v:\n %q", c.want, got)
			}
			// Whatever else it says, it says which build answered. A line
			// without the version is not a version line.
			if raw := versions.Main().Raw; raw == "" || !strings.Contains(got, raw) {
				t.Errorf("expected the build %q in the display line %q", raw, got)
			}
			// A bare number is what usql prints for MySQL, and it is the
			// regression this test exists to catch.
			if c.want.MatchString(versions.Main().Raw) {
				t.Errorf("the display line is the raw version and names no product: %q", got)
			}
		})
	}
	if ran == 0 {
		t.Skip("no database was reachable")
	}
}

// TestMySQLAndMariaDBAreNamedApart pins the one display that depends on which
// product answered, rather than on the dialect. See D44.
func TestMySQLAndMariaDBAreNamedApart(t *testing.T) {
	db := openMySQL(t)
	versions, err := dbmeta.MySQL.Version(t.Context(), db)
	if err != nil {
		t.Fatalf("reading the version: %v", err)
	}
	got, raw := versions.String(), versions.Main().Raw
	want := "MySQL "
	if strings.Contains(strings.ToLower(raw), "mariadb") {
		want = "MariaDB "
	}
	if !strings.HasPrefix(got, want) {
		t.Errorf("the server reported %q, so the line must begin %q, got %q", raw, want, got)
	}
}

// TestCurrentUserNamesTheConnection checks the kind D55 moved here from usql.
//
// It is the one query besides CurrentSchema that describes the connection
// rather than the database, and the one whose answer differs by who connected
// rather than by what the server holds.
func TestCurrentUserNamesTheConnection(t *testing.T) {
	var ran int
	for _, c := range displayShapes {
		t.Run(c.name, func(t *testing.T) {
			db := c.open(t)
			m, err := dbmeta.New(c.dialect, dbmeta.VersionSet{})
			if err != nil {
				t.Fatalf("building the metadata: %v", err)
			}
			if dbmeta.CurrentUser.Support(m) != dbmeta.Supported {
				// SQLite has no users and says so rather than inventing one.
				if c.dialect != dbmeta.SQLite3 {
					t.Fatalf("expected %s to answer the current user", c.dialect)
				}
				if _, _, err := dbmeta.CurrentUser.Build(m, nil); !errors.Is(err, dbmeta.ErrNotSupported) {
					t.Errorf("expected ErrNotSupported, got %v", err)
				}
				return
			}
			ran++
			user, ok, err := dbmeta.First(dbmeta.CurrentUser.All(t.Context(), m, db, nil))
			if err != nil {
				t.Fatalf("reading the current user: %v", err)
			}
			if !ok {
				t.Fatal("expected one row: a connection always has a user")
			}
			if user.Name == "" {
				t.Error("expected a user name, got an empty string")
			}
			// Absent is a fact here and empty is not. DuckDB has no session
			// user and reports NULL, and every other product reports one.
			if c.dialect == dbmeta.DuckDB {
				if user.Session.Valid {
					t.Errorf("DuckDB has no session user, got %q", user.Session.V)
				}
			} else if !user.Session.Valid || user.Session.V == "" {
				t.Errorf("expected a session user, got %#v", user.Session)
			}
			t.Logf("name=%q session=%v", user.Name, user.Session)
		})
	}
	if ran == 0 {
		t.Skip("no database was reachable")
	}
}
