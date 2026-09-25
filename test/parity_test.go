package test

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/xo/dbmeta"
	myfixture "github.com/xo/dbmeta/models/mysql/fixture"
	orfixture "github.com/xo/dbmeta/models/oracle/fixture"
	pgfixture "github.com/xo/dbmeta/models/postgres/fixture"
	msfixture "github.com/xo/dbmeta/models/sqlserver/fixture"
)

// The privilege parity test, which D61 requires of every dialect.
//
// Every query is asked twice of the same objects: once as the administrator
// the other tests use, and once as an ordinary principal. A query that
// answers differently is reporting a fact about the connection rather than
// about the database, and a consumer has to be told which ones do that.
//
// Oracle is why this exists. Its ALL_ views show the caller only what the
// caller may see, which was known, but nothing had measured whether the same
// thing happens elsewhere. It does: PostgreSQL filters pg_stats and
// pg_settings by role, and information_schema filters everywhere.
//
// # A principal is not one thing
//
// SQL Server has three and they are not interchangeable. A sysadmin. A server
// login mapped to a database user, which is the ordinary model. And a
// contained database user, which has a password in the database itself and no
// login at the server, and which needs CONTAINMENT set to PARTIAL.
//
// Oracle has the same three from 12c. SYSTEM is the administrator. A common
// user exists in the container database and in every pluggable database at
// once, which is what a server login is. A local user authenticates against
// one pluggable database and has nothing above it, which is what a contained
// database user is.
//
// PostgreSQL has no containment, because a role belongs to the cluster and
// not to a database. The nearest three are the superuser, the owner of the
// objects, and a role holding only grants.
//
// MySQL and MariaDB have no containment either. A user is a name and a host
// at server level and a database is only a grant scope, so there are two.
//
// SQLite and DuckDB have no user, no role and no grant, so there is nothing
// to compare and this test cannot cover them.
//
// # Why the expectation is checked in
//
// A difference is not a failure. A principal with no privilege on another
// schema has no business seeing it. The file records which queries differ so
// that a change in the set is what fails, the same way conformance.txt works.

// parityGolden is the checked in expectation.
const parityGolden = "testdata/parity.txt"

// parityHeader is written at the top of that file.
const parityHeader = `# What each query answers for a principal that is not the administrator.
#
# Written by go test -update. A line is a query that answered differently for
# the principal named in the section. An empty section means every query gave
# the administrator's answer, which is what a principal owning the objects
# ought to get.
#
# A section is <database>/<scene>/<principal>. The scene is the database the
# comparison ran in, because a contained user needs one of its own.
`

// parityPassword is what every principal is created with. SQL Server enforces
// complexity, so it has an uppercase letter, a digit and a symbol.
const parityPassword = "P4ssw0rd!x"

// parityTarget is one database, its scenes, and the principals in each.
type parityTarget struct {
	name   string
	driver string
	env    string
	// open returns the administrator connection, or skips.
	open func(*testing.T) *sql.DB
	// build runs the fixture on a connection and returns the meta.
	build func(*testing.T, *sql.DB) *dbmeta.Meta
	// schema is where the fixture built its objects.
	schema string
	scenes []parityScene
}

// parityScene is one database the comparison runs in.
//
// Most products need only the database the administrator already connects to.
// A SQL Server contained user needs one of its own, because containment
// cannot be set on master.
type parityScene struct {
	name string
	// prepare makes the database and returns the DSN to reach it. A nil
	// prepare means the administrator's own DSN.
	prepare    func(t *testing.T, admin *sql.DB, adminDSN string) string
	principals []parityPrincipal
}

// parityPrincipal is one non-administrator user.
type parityPrincipal struct {
	name string
	// make creates the principal in the scene and returns its DSN.
	make func(t *testing.T, scene *sql.DB, sceneDSN, schema string) string
}

func parityTargets() []parityTarget {
	return []parityTarget{
		{
			name: "postgres", driver: "pgx", env: "DBMETA_POSTGRES",
			open: open, build: setup, schema: pgfixture.Everything.Schema,
			scenes: []parityScene{{
				name: "same",
				principals: []parityPrincipal{
					{name: "owner", make: makePostgresOwner},
					{name: "grantee", make: makePostgresGrantee},
				},
			}},
		},
		{
			name: "mysql", driver: "mysql", env: "DBMETA_MYSQL",
			open: openMySQL, build: setupMySQL, schema: myfixture.Everything.Schema,
			scenes: []parityScene{{
				name:       "same",
				principals: []parityPrincipal{{name: "grantee", make: makeMySQLGrantee}},
			}},
		},
		{
			name: "sqlserver", driver: "sqlserver", env: "DBMETA_SQLSERVER",
			open: openSQLServer, build: setupSQLServer, schema: msfixture.Everything.Schema,
			scenes: []parityScene{
				{
					name:       "same",
					principals: []parityPrincipal{{name: "login", make: makeSQLServerLogin}},
				},
				{
					name:       "contained",
					prepare:    prepareSQLServerContained,
					principals: []parityPrincipal{{name: "contained", make: makeSQLServerContained}},
				},
			},
		},
		{
			name: "oracle", driver: "oracle", env: "DBMETA_ORACLE",
			open: openOracle, build: setupOracle, schema: orfixture.Everything.Schema,
			scenes: []parityScene{{
				// A common user cannot be made from inside a pluggable
				// database, and nothing connects to CDB$ROOT. That target is
				// recorded as missing in container/oracle.go and this is the
				// second place it would be used.
				name:       "same",
				principals: []parityPrincipal{{name: "local", make: makeOracleLocal}},
			}},
		},
	}
}

// TestPrivilegeParity asks every query as the administrator and as each
// principal, and records the queries that answer differently.
//
// Run with -update to rewrite the expectation. Read the diff: a query that
// starts differing has begun depending on who is asking.
func TestPrivilegeParity(t *testing.T) {
	want := readGoldenAt(t, parityGolden)
	var ran int
	for _, target := range parityTargets() {
		t.Run(target.name, func(t *testing.T) {
			admin := target.open(t)
			adminDSN := dsnOf(t, target.env)
			for _, scene := range target.scenes {
				t.Run(scene.name, func(t *testing.T) {
					sceneDSN := adminDSN
					sceneDB := admin
					if scene.prepare != nil {
						sceneDSN = scene.prepare(t, admin, adminDSN)
						sceneDB = openAt(t, target.driver, sceneDSN)
					}
					m := target.build(t, sceneDB)
					for _, who := range scene.principals {
						t.Run(who.name, func(t *testing.T) {
							dsn := who.make(t, sceneDB, sceneDSN, target.schema)
							db := openAt(t, target.driver, dsn)
							// The administrator is asked again after the
							// principal exists. Making one changes the
							// catalog: an owner changes who owns the schema
							// and a grantee adds a grant, and a baseline read
							// before that reports the change as a difference
							// in privilege, which it is not.
							baseline := parityRun(t, sceneDB, m, target.schema)
							got := parityReport(baseline, parityRun(t, db, m, target.schema))
							section := parityName(target.name, m) +
								"/" + scene.name + "/" + who.name
							ran++
							if *update {
								writeGoldenAt(t, parityGolden, parityHeader, section, got)
								return
							}
							expected, ok := want[section]
							if !ok {
								t.Fatalf("no expectation for %s in %s."+
									" Run go test -update and read the diff.",
									section, parityGolden)
							}
							compareReport(t, section, expected, got)
						})
					}
				})
			}
		})
	}
	if ran == 0 {
		t.Skip("no database was reachable")
	}
}

// parityName is the database the section is recorded under.
//
// It is the product rather than the dialect. MariaDB and MySQL share a
// dialect and do not share the tables these queries are refused on: mysql.proc
// was removed in MySQL 8.0 and MariaDB still has it, so one file cannot hold
// one answer for both.
func parityName(dialect string, m *dbmeta.Meta) string {
	for _, key := range m.Version().Keys() {
		if key != "" && key != dialect {
			return key
		}
	}
	return dialect
}

// openAt connects and closes at the end of the test.
func openAt(t *testing.T, driver, dsn string) *sql.DB {
	t.Helper()
	db, err := sql.Open(driver, dsn)
	if err != nil {
		t.Fatalf("opening %s: %v", driver, err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.PingContext(t.Context()); err != nil {
		t.Fatalf("connecting as the principal: %v", err)
	}
	return db
}

// parityAnswer is what one query returned.
type parityAnswer struct {
	rows int
	// body is every row as text, so a value the server blanked out is a
	// difference and not only a missing row.
	body string
	// err is what the server said, for a query the principal may not run.
	err string
}

// parityRun asks every supported query and records the answer.
func parityRun(t *testing.T, db *sql.DB, m *dbmeta.Meta, schema string) map[string]parityAnswer {
	t.Helper()
	ctx := t.Context()
	out := map[string]parityAnswer{}
	for _, q := range dbmeta.Queries() {
		if q.Support(m) != dbmeta.Supported {
			continue
		}
		args, err := parityArgs(q, m, schema)
		if err != nil {
			continue
		}
		sqlstr, vals, err := q.SQL(m, args)
		if err != nil {
			continue
		}
		out[q.Name()] = parityAsk(ctx, db, sqlstr, vals)
	}
	return out
}

// parityArgs keeps only the filter values the query declares, because passing
// one it does not declare is ErrUnknownParam.
func parityArgs(q dbmeta.AnyQuery, m *dbmeta.Meta, schema string) (map[string]any, error) {
	params, err := q.Params(m)
	if err != nil {
		return nil, err
	}
	want := dbmeta.Args{Schema: schema}.Map()
	out := map[string]any{}
	for _, p := range params {
		if v, ok := want[p.Name]; ok {
			out[p.Name] = v
		}
	}
	return out, nil
}

// parityAsk runs one statement and returns its rows as text.
//
// It reads the result generically rather than through the typed iterator,
// because the comparison is the same for every query and a scan per object
// kind would be a second copy of every model.
func parityAsk(ctx context.Context, db *sql.DB, sqlstr string, vals []any) parityAnswer {
	rows, err := db.QueryContext(ctx, sqlstr, vals...)
	if err != nil {
		return parityAnswer{err: firstLine(err.Error())}
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		return parityAnswer{err: firstLine(err.Error())}
	}
	var out []string
	for rows.Next() {
		cells := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range cells {
			ptrs[i] = &cells[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return parityAnswer{err: firstLine(err.Error())}
		}
		parts := make([]string, len(cells))
		for i, c := range cells {
			if b, ok := c.([]byte); ok {
				c = string(b)
			}
			parts[i] = fmt.Sprintf("%v", c)
		}
		out = append(out, strings.Join(parts, "\x1f"))
	}
	if err := rows.Err(); err != nil {
		return parityAnswer{err: firstLine(err.Error())}
	}
	// Sorted, because a query that orders by a name the principal cannot see
	// can return the same rows in another order.
	sort.Strings(out)
	return parityAnswer{rows: len(out), body: strings.Join(out, "\n")}
}

// firstLine keeps an error to one line and takes the host out of it.
//
// A server that prints a stack of context makes the expectation unreadable,
// and MySQL names the client in the message: "denied to user 'x'@'10.0.0.2'"
// records the address of whichever machine ran the test, which is never the
// same twice. The user is the part that matters and it stays.
func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	return clientHost.ReplaceAllString(s, "@'client'")
}

// clientHost matches the host half of a MySQL user name.
var clientHost = regexp.MustCompile(`@'[^']*'`)

// parityReport returns one line per query that answered differently.
func parityReport(admin, other map[string]parityAnswer) []string {
	names := make([]string, 0, len(admin))
	for name := range admin {
		names = append(names, name)
	}
	sort.Strings(names)
	var out []string
	for _, name := range names {
		a, p := admin[name], other[name]
		switch {
		case p.err != "" && a.err == "":
			out = append(out, fmt.Sprintf("%s refused: %s", name, p.err))
		case p.rows < a.rows:
			out = append(out, name+" fewer rows than the administrator")
		case p.rows > a.rows:
			out = append(out, name+" more rows than the administrator")
		case a.body != p.body:
			out = append(out, name+" the same rows with different values")
		}
	}
	return out
}
