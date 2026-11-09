package main

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/xo/dbmeta"
	"github.com/xo/dbmeta/container"
)

// dumpFiles holds the SQL of the dump fixture (D291). The fixture holds the
// objects that test a dump: the order of its statements, the quoting of
// names, and the variants of each kind of object. Northwind and hr stay as
// they are, because the describe commands of usql compare their output with
// them.
//
//go:embed dump/postgres.sql dump/sqlserver.sql dump/oracle.sql dump/mysql.sql dump/mysql-only.sql dump/mariadb-only.sql dump/sqlite3.sql dump/duckdb.sql
var dumpFiles embed.FS

// dumpSource names the files of a product, in the order they load. Every
// product makes the table people in each of its two schemas, with rows, and
// count reads the number of rows of the two, so that a load is checked by what
// it left.
type dumpSource struct {
	files []string
	// more names files that only one flavor of the dialect applies, by the
	// product, and they load after files.
	more map[string][]string
	// count returns the number of rows of the table people in the first
	// schema and in the second.
	count string
	// header is a statement that goes at the top of every file, such as the
	// one that chooses the schema on Oracle.
	header string
}

// dumpSources is every product that has the fixture.
var dumpSources = map[dbmeta.Dialect]dumpSource{
	dbmeta.PostgreSQL: {
		files: []string{"dump/postgres.sql"},
		count: `SELECT (SELECT COUNT(*) FROM core.people), (SELECT COUNT(*) FROM extra.people)`,
	},
	dbmeta.SQLServer: {
		files: []string{"dump/sqlserver.sql"},
		count: `SELECT (SELECT COUNT(*) FROM core.people), (SELECT COUNT(*) FROM extra.people)`,
	},
	dbmeta.Oracle: {
		files:  []string{"dump/oracle.sql"},
		count:  `SELECT (SELECT COUNT(*) FROM dump_core.people), (SELECT COUNT(*) FROM dump_extra.people) FROM dual`,
		header: "ALTER SESSION SET CURRENT_SCHEMA = dump_core;\n",
	},
	dbmeta.MySQL: {
		files: []string{"dump/mysql.sql"},
		more: map[string][]string{
			"mysql":   {"dump/mysql-only.sql"},
			"mariadb": {"dump/mariadb-only.sql"},
		},
		count: `SELECT (SELECT COUNT(*) FROM dump_core.people), (SELECT COUNT(*) FROM dump_extra.people)`,
	},
	dbmeta.SQLite3: {
		files: []string{"dump/sqlite3.sql"},
		count: `SELECT (SELECT COUNT(*) FROM people), (SELECT COUNT(*) FROM extra_people)`,
	},
	dbmeta.DuckDB: {
		files: []string{"dump/duckdb.sql"},
		count: `SELECT (SELECT COUNT(*) FROM core.people), (SELECT COUNT(*) FROM extra.people)`,
	},
}

// dumpDatabase is the name of the database that the fixture makes, where a
// server has one. MySQL and MariaDB call a schema a database, so they get the
// two databases dump_core and dump_extra, and Oracle gets the two users of the
// same names.
const dumpDatabase = "dump"

// doDump drops what an earlier run made and builds the dump fixture again from
// the embedded SQL, so that two runs leave the same rows. usql applies the
// files, as it does for northwind.
func doDump(ctx context.Context, r runner, t target) error {
	src, ok := dumpSources[t.Dialect]
	if !ok || (t.Kind != kindContainer && t.Kind != kindEmbedded) {
		return fmt.Errorf("the dump fixture is not written for %s", t.Name)
	}
	if t.Kind == kindContainer && !r.running(ctx, t.Name) {
		return fmt.Errorf("it is not running. Start it with: dbrun start %s", t.Name)
	}
	if _, err := exec.LookPath("usql"); err != nil {
		return errors.New("usql is not on the path")
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	if err := dumpPrepare(ctx, t); err != nil {
		return err
	}
	target, err := dumpURL(t)
	if err != nil {
		return err
	}
	files := slices.Concat(src.files, src.more[t.Product])
	for _, name := range files {
		b, err := dumpFiles.ReadFile(name)
		if err != nil {
			return fmt.Errorf("reading %s: %w", name, err)
		}
		if err := usqlFile(ctx, target, src.header+string(b)); err != nil {
			return fmt.Errorf("applying %s: %w", name, err)
		}
	}
	out, err := usqlOutput(ctx, target, src.count)
	if err != nil {
		return fmt.Errorf("counting the rows: %w", err)
	}
	var core, extra int
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if _, err := fmt.Sscanf(lines[len(lines)-1], "%d|%d", &core, &extra); err != nil {
		return fmt.Errorf("reading the count %q: %w", out, err)
	}
	if core == 0 || extra == 0 {
		return fmt.Errorf("the tables people have %d and %d rows, and each must have some", core, extra)
	}
	where := "the database " + dumpDatabase
	if t.Kind == kindEmbedded {
		where = "the file " + dumpFile(t)
	}
	fmt.Printf("  %-*s dump: %d files, %d and %d people, in %s\n", nameWidth(), t.Name, len(files), core, extra, where)
	return nil
}

// dumpPrepare removes what an earlier run made and makes what the files expect
// to find.
func dumpPrepare(ctx context.Context, t target) error {
	dsn := t.connectDSN()
	switch t.Dialect {
	case dbmeta.PostgreSQL:
		return runSteps(ctx, dsn, dialectDriver(t), []string{
			"SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = '" + dumpDatabase + "' AND pid <> pg_backend_pid()",
			"DROP DATABASE IF EXISTS " + dumpDatabase,
			"CREATE DATABASE " + dumpDatabase,
		})
	case dbmeta.SQLServer:
		return runSteps(ctx, dsn, dialectDriver(t), []string{
			"IF DB_ID('" + dumpDatabase + "') IS NOT NULL BEGIN ALTER DATABASE [" + dumpDatabase +
				"] SET SINGLE_USER WITH ROLLBACK IMMEDIATE; DROP DATABASE [" + dumpDatabase + "]; END",
			"CREATE DATABASE [" + dumpDatabase + "]",
		})
	case dbmeta.Oracle:
		return runSteps(ctx, dsn, dialectDriver(t), dumpOraclePrepare())
	case dbmeta.MySQL:
		return runSteps(ctx, dsn, dialectDriver(t), []string{
			// Tables of the two databases refer to each other, so a drop
			// must not check the keys.
			"SET FOREIGN_KEY_CHECKS = 0",
			"DROP DATABASE IF EXISTS dump_core",
			"DROP DATABASE IF EXISTS dump_extra",
			"SET FOREIGN_KEY_CHECKS = 1",
			"CREATE DATABASE dump_core CHARACTER SET utf8mb4",
			"CREATE DATABASE dump_extra CHARACTER SET utf8mb4",
		})
	case dbmeta.SQLite3, dbmeta.DuckDB:
		// An embedded database is a file, and the fixture owns a file of its own,
		// so that a run leaves nothing of the one before.
		for _, file := range []string{dumpFile(t)} {
			if err := os.Remove(file); err != nil && !errors.Is(err, fs.ErrNotExist) {
				return fmt.Errorf("removing %s: %w", file, err)
			}
		}
	}
	return nil
}

// dumpOraclePrepare drops and makes the two users that hold the fixture,
// because Oracle calls a schema a user.
func dumpOraclePrepare() []string {
	drop := func(name string) string {
		return fmt.Sprintf("BEGIN EXECUTE IMMEDIATE 'DROP USER %s CASCADE'; EXCEPTION WHEN OTHERS THEN IF SQLCODE != -1918 THEN RAISE; END IF; END;", name)
	}
	steps := []string{drop("dump_core"), drop("dump_extra")}
	for _, user := range []string{"dump_core", "dump_extra"} {
		steps = append(steps,
			fmt.Sprintf(`CREATE USER %s IDENTIFIED BY "%s"`, user, container.Password),
			"GRANT CREATE SESSION, CREATE TABLE, CREATE VIEW, CREATE SEQUENCE, CREATE PROCEDURE, CREATE TRIGGER, "+
				"CREATE TYPE, CREATE SYNONYM, CREATE MATERIALIZED VIEW, UNLIMITED TABLESPACE TO "+user)
	}
	return steps
}

// dumpURL is the URL that usql opens for the fixture. The first schema is the
// one it opens, and the files name the second one.
func dumpURL(t target) (string, error) {
	if t.Kind == kindEmbedded {
		return string(t.Dialect) + ":" + dumpFile(t), nil
	}
	u, err := url.Parse(t.URL)
	if err != nil {
		return "", fmt.Errorf("reading the URL of %s: %w", t.Name, err)
	}
	switch t.Dialect {
	case dbmeta.PostgreSQL:
		u.Path = "/" + dumpDatabase
	case dbmeta.MySQL:
		u.Path = "/dump_core"
	case dbmeta.SQLServer:
		q := u.Query()
		q.Set("database", dumpDatabase)
		u.RawQuery = q.Encode()
	}
	return u.String(), nil
}

// dumpFile is the file of the fixture for an embedded database, beside the file
// that dbrun keeps for the product.
func dumpFile(t target) string {
	return filepath.Join(filepath.Dir(t.DSN), dumpDatabase+filepath.Ext(t.DSN))
}
