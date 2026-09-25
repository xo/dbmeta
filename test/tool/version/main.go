// Command version connects to a server and prints what dbmeta makes of its
// version.
//
// run.sh calls it, so that `run.sh version` answers from the same query the
// models use rather than from whatever the product's own client prints. A
// disagreement between the two is worth seeing, and it has happened: a
// version string a model parsed one way and a person read another way is how
// a fragment ends up gated on the wrong number.
//
// It lives in the test module because it opens a connection, which means a
// driver, which the root module must never import.
package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"time"

	_ "github.com/ClickHouse/clickhouse-go/v2"
	_ "github.com/MichaelS11/go-cql-driver"
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"
	_ "github.com/microsoft/go-mssqldb"
	_ "github.com/sijms/go-ora/v2"

	"github.com/xo/dbmeta"
	_ "github.com/xo/dbmeta/all"
)

// drivers names the driver each dialect connects with. It is the one usql
// uses for that database, which D52 requires, and D59 is why Oracle differs.
var drivers = map[dbmeta.Dialect]string{
	dbmeta.PostgreSQL: "pgx",
	dbmeta.MySQL:      "mysql",
	dbmeta.SQLServer:  "sqlserver",
	dbmeta.Oracle:     "oracle",
	dbmeta.Cassandra:  "cql",
	dbmeta.ClickHouse: "clickhouse",
}

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: version <dialect> <dsn>")
		os.Exit(2)
	}
	if err := run(dbmeta.Dialect(os.Args[1]), os.Args[2]); err != nil {
		fmt.Fprintln(os.Stderr, "version:", err)
		os.Exit(1)
	}
}

func run(dialect dbmeta.Dialect, dsn string) error {
	driver, ok := drivers[dialect]
	if !ok {
		return fmt.Errorf("no driver for %s: it has no server to connect to", dialect)
	}
	db, err := sql.Open(driver, dsn)
	if err != nil {
		return fmt.Errorf("opening %s: %w", dialect, err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	versions, err := dialect.Version(ctx, db)
	if err != nil {
		return fmt.Errorf("reading the version of %s: %w", dialect, err)
	}
	fmt.Println(versions)
	return nil
}
