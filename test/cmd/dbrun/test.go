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
	_ "github.com/MichaelS11/go-cql-driver"
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"
	_ "github.com/microsoft/go-mssqldb"
	_ "github.com/nakagami/firebirdsql"
	_ "github.com/prestodb/presto-go-client/v2"
	_ "github.com/sijms/go-ora/v2"
	_ "github.com/trinodb/trino-go-client/trino"

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
	dbmeta.MySQL:      "mysql",
	dbmeta.SQLServer:  "sqlserver",
	dbmeta.Oracle:     "oracle",
	dbmeta.Cassandra:  "cql",
	dbmeta.ClickHouse: "clickhouse",
	dbmeta.Trino:      "trino",
	dbmeta.Presto:     "presto",
	dbmeta.Firebird:   "firebirdsql",
}

// doVersion connects and prints what dbmeta reads, rather than what the
// product's own client prints.
//
// The difference is worth seeing. A version string a model parses one way and
// a person reads another way is how a fragment ends up gated on the wrong
// number.
func doVersion(ctx context.Context, r runner, t target) error {
	if t.Kind == kindEmbedded {
		fmt.Printf("  %-20s embedded, whatever the driver links\n", t.Name)
		return nil
	}
	if !r.running(ctx, t.Name) {
		return nil
	}
	driver, ok := drivers[t.Dialect]
	if !ok {
		return fmt.Errorf("no driver for %s", t.Dialect)
	}
	db, err := sql.Open(driver, t.DSN)
	if err != nil {
		return fmt.Errorf("opening: %w", err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	versions, err := t.Dialect.Version(ctx, db)
	if err != nil {
		return fmt.Errorf("reading the version: %w", err)
	}
	fmt.Printf("  %-20s %s\n", t.Name, versions)
	return nil
}

// doTest runs the integration tests against one server.
//
// What happens afterwards differs by kind and that is deliberate. A container
// is removed, because rebuilding it is a minute and leaving it is how a
// machine fills with servers nobody can place. A Windows machine is kept,
// because rebuilding it is an hour. Both reviews of this design wanted one
// rule with a flag, and the asymmetry is real, so the answer is to say which
// one happened rather than to pick the wrong default for one of them.
// --keep and --remove override it.
func doTest(ctx context.Context, r runner, t target, o options) error {
	if t.Kind == kindEmbedded {
		fmt.Printf("=== %s ===\n", t.Name)
		// The file is named the same way a server's DSN is, so the tests put
		// the database where dbrun says and it is still there afterwards.
		// --remove deletes it, and nothing else does: a library is kept the
		// way a machine is, so that `dbrun usql sqlite3` can open what the
		// test built.
		if err := os.MkdirAll(filepath.Dir(t.DSN), 0o755); err != nil {
			return fmt.Errorf("making the directory for %s: %w", t.DSN, err)
		}
		if err := goTest(ctx, t.Env+"="+t.DSN); err != nil {
			return err
		}
		if o.remove {
			if err := os.Remove(t.DSN); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("removing %s: %w", t.DSN, err)
			}
			fmt.Printf("  removed %s\n", t.DSN)
			return nil
		}
		fmt.Printf("  kept %s\n", t.DSN)
		return nil
	}
	fmt.Printf("=== %s ===\n", t.Name)

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
	defer func() {
		if keep {
			fmt.Printf("  kept %s\n", t.Name)
			return
		}
		// context.WithoutCancel, so that a cancelled run still cleans up
		// rather than leaving a container bound to a port.
		r.quiet(context.WithoutCancel(ctx), t.Remove...)
		fmt.Printf("  removed %s\n", t.Name)
	}()

	return goTest(ctx, t.Env+"="+t.DSN)
}

// goTest runs the integration tests, with one environment variable set when
// there is a server to point at.
func goTest(ctx context.Context, env string) error {
	cmd := exec.CommandContext(ctx, "go", "test", "-count=1", "./...")
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	cmd.Env = os.Environ()
	if env != "" {
		cmd.Env = append(cmd.Env, env)
	}
	if err := cmd.Run(); err != nil {
		return errors.New("the tests failed")
	}
	fmt.Println("  passed")
	return nil
}
