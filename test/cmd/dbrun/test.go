package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	_ "github.com/ClickHouse/clickhouse-go/v2"
	_ "github.com/SAP/go-hdb/driver"
	_ "github.com/beltran/gohive/v2"
	_ "github.com/exasol/exasol-driver-go"
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"
	_ "github.com/microsoft/go-mssqldb"
	_ "github.com/nakagami/firebirdsql"
	_ "github.com/prestodb/presto-go-client/v2"
	_ "github.com/sijms/go-ora/v2"
	_ "github.com/trinodb/trino-go-client/trino"
	_ "github.com/vertica/vertica-sql-go"
	_ "github.com/xo/cql"
	_ "github.com/xo/dbimp/couchbase"

	"github.com/xo/dbmeta"
	_ "github.com/xo/dbmeta/all"
)

// drivers names the driver each dialect connects with. It is the one usql
// uses for that database, which D52 requires, and D59 is why Oracle differs.
//
// SQLite and DuckDB are not here. They are a library rather than a server, so
// there is no connection for dbrun to make: their tests open their own file.
var drivers = map[dbmeta.Dialect]string{
	dbmeta.PostgreSQL: "pgx",
	// cockroachdb:// opens pgx, as dburl v0.35.0 says.
	dbmeta.CockroachDB: "pgx",
	dbmeta.MySQL:       "mysql",
	dbmeta.SQLServer:   "sqlserver",
	dbmeta.Oracle:      "oracle",
	dbmeta.Cassandra:   "cql",
	dbmeta.ClickHouse:  "clickhouse",
	dbmeta.Trino:       "trino",
	dbmeta.Presto:      "presto",
	dbmeta.Firebird:    "firebirdsql",
	dbmeta.HANA:        "hdb",
	dbmeta.Hive:        "hive",
	dbmeta.Exasol:      "exasol",
	dbmeta.Vertica:     "vertica",
	dbmeta.Couchbase:   "couchbase",
}

// doVersion connects and prints what dbmeta reads, rather than what the
// product's own client prints.
//
// The difference is worth seeing. A version string a model parses one way and
// a person reads another way is how a fragment ends up gated on the wrong
// number.
func doVersion(ctx context.Context, r runner, t target) error {
	if t.Kind == kindEmbedded {
		fmt.Printf("  %-*s embedded, whatever the driver links\n", nameWidth(), t.Name)
		return nil
	}
	if t.Kind != kindHosted && !r.running(ctx, t.Name) {
		return nil
	}
	versions, err := readVersion(ctx, t)
	if err != nil {
		return err
	}
	fmt.Printf("  %-*s %s\n", nameWidth(), t.Name, versions)
	return nil
}

// readVersion connects to a running server and reads its version the way
// dbmeta does.
func readVersion(ctx context.Context, t target) (dbmeta.VersionSet, error) {
	driver, ok := drivers[t.Dialect]
	if !ok {
		return dbmeta.VersionSet{}, fmt.Errorf("no driver for %s", t.Dialect)
	}
	db, err := sql.Open(driver, t.connectDSN())
	if err != nil {
		return dbmeta.VersionSet{}, fmt.Errorf("opening: %w", err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	versions, err := t.Dialect.Version(ctx, db)
	if err != nil {
		return dbmeta.VersionSet{}, fmt.Errorf("reading the version: %w", err)
	}
	return versions, nil
}

// doTest runs the integration tests against one server.
//
// What happens afterwards differs by kind and that is deliberate. A container
// is removed, because rebuilding it is a minute and leaving it is how a
// machine fills with servers nobody can place. A virtual machine is kept,
// because rebuilding it is an hour for Windows and a fresh import for an
// appliance. Both reviews of this design wanted one
// rule with a flag, and the asymmetry is real, so the answer is to say which
// one happened rather than to pick the wrong default for one of them.
// --keep and --remove override it.
func doTest(ctx context.Context, r runner, t target, o options) error {
	if t.Kind == kindHosted {
		// Nothing to start or remove. The tests read the connection string
		// from their environment, where no process list shows it.
		fmt.Printf("=== %s ===\n", t.Name)
		return goTest(ctx, t.env()...)
	}
	if t.Kind == kindEmbedded {
		fmt.Printf("=== %s ===\n", t.Name)
		// The file is named the same way a server's DSN is, so the tests put
		// the database where dbrun says and it is still there afterwards.
		// --remove deletes it, and nothing else does: a library is kept the
		// way a machine is, so that `dbrun usql sqlite3` can open what the
		// test built.
		dir := filepath.Dir(t.DSN)
		if t.Directory {
			// csvq reads a directory of files, which has to exist.
			dir = t.DSN
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("making the directory for %s: %w", t.DSN, err)
		}
		if err := extractSamples(t); err != nil {
			return err
		}
		if err := goTest(ctx, t.Env+"="+t.DSN); err != nil {
			return err
		}
		if o.remove {
			// chai and csvq are a directory, which goes with what it holds.
			remove := os.Remove
			if t.Directory {
				remove = os.RemoveAll
			}
			if err := remove(t.DSN); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("removing %s: %w", t.DSN, err)
			}
			fmt.Printf("  removed %s\n", t.DSN)
			return nil
		}
		fmt.Printf("  kept %s\n", t.DSN)
		return nil
	}
	fmt.Printf("=== %s ===\n", t.Name)

	// A server that was already up when the test began is somebody's, and
	// the test only borrows it, unless it is the caller's own. A server with
	// no owner was removed here once, when it predated owners and another
	// session had started it. See D98.
	wasUp := r.running(ctx, t.Name)
	if err := doStart(ctx, r, t, o); err != nil {
		return err
	}

	keep := t.Kind == kindMachine
	switch {
	case o.keep:
		keep = true
	case o.remove:
		keep = false
	}
	// A test never removes a server it may not touch, which is one another
	// session started and this test only shared, nor one that was up before
	// it began and is not the caller's. See D98.
	owner := r.owner(ctx, t.Name)
	shared := !mayTouch(owner, currentOwner(), o.force) ||
		(wasUp && owner != currentOwner() && !o.force)
	defer func() {
		if shared {
			fmt.Printf("  kept %s, which belongs to %s\n", t.Name, r.who(ctx, t.Name))
			return
		}
		if keep {
			fmt.Printf("  kept %s\n", t.Name)
			return
		}
		// context.WithoutCancel, so that a cancelled run still cleans up
		// rather than leaving a container bound to a port.
		r.quiet(context.WithoutCancel(ctx), t.Remove...)
		fmt.Printf("  removed %s\n", t.Name)
	}()

	return goTest(ctx, t.env()...)
}

// goTest runs the integration tests, with the environment variables of the
// server set when there is one to point at. A server that answers two
// dialects sets two (D114).
//
// DBMETA_TEST_BINARY names a test binary built earlier by `go test -c`, and
// running that rather than `go test` is what makes the CI matrix affordable.
// Compiling the tests takes about ninety seconds, because the test module
// links every driver and two of them are cgo, and a matrix job spends only a
// second or two actually testing. Measured on one nightly job: 103 seconds in
// the step, 95 of them compiling and 1.4 running. The workflow builds the
// binary once and every job runs it. See D82.
//
// A person runs `go test`, which is what they want: it recompiles what they
// just changed. Nothing is set for them and nothing changes.
func goTest(ctx context.Context, env ...string) error {
	argv := []string{"go", "test", "-count=1", "./..."}
	if bin := os.Getenv("DBMETA_TEST_BINARY"); bin != "" {
		// An absolute path, because the tests run with the test module as the
		// working directory and a relative one would be read against it.
		abs, err := filepath.Abs(bin)
		if err != nil {
			return fmt.Errorf("resolving DBMETA_TEST_BINARY %s: %w", bin, err)
		}
		if _, err := os.Stat(abs); err != nil {
			return fmt.Errorf("reading DBMETA_TEST_BINARY %s: %w", abs, err)
		}
		argv = []string{abs, "-test.count=1"}
	}
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	cmd.Env = append(os.Environ(), env...)
	if err := cmd.Run(); err != nil {
		return errors.New("the tests failed")
	}
	fmt.Println("  passed")
	return nil
}
