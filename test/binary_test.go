package test

import (
	"context"
	"encoding/hex"
	"os"
	"testing"

	"github.com/xo/dbmeta"
)

// binaryColumn is a column of a probe table. binary says that the
// model finds it binary, and a column that is not binary gets no format.
type binaryColumn struct {
	name   string
	binary bool
}

// binaryProbe is a table with one row, and the columns of it that the test
// reads in this order.
type binaryProbe struct {
	setup   []string
	query   string
	cleanup []string
	// fallback holds a statement to run when the setup statement that is its
	// key fails, because ScyllaDB refuses what Cassandra takes.
	fallback map[string]string
	cols     []binaryColumn
}

// mysqlBinaryProbe is the probe of every product that speaks the protocol of
// MySQL, because the client decides the form, and the mysql client is the one
// they ship (D258).
var mysqlBinaryProbe = binaryProbe{
	setup: []string{
		"CREATE TEMPORARY TABLE dbm_bin (b binary(8), vb varbinary(8), tb tinyblob, bl blob, mb mediumblob, lb longblob, c char(3), vc varchar(8), t text, i int)",
		"INSERT INTO dbm_bin (b, c, vc, t, i) VALUES (x'151c2f00ffab', 'abc', 'abc', 'abc', 1)",
		"UPDATE dbm_bin SET vb = b, tb = b, bl = b, mb = b, lb = b",
	},
	query: "SELECT b, vb, tb, bl, mb, lb, c, vc, t, i FROM dbm_bin",
	cols: []binaryColumn{{"binary", true}, {"varbinary", true}, {"tinyblob", true}, {"blob", true}, {"mediumblob", true},
		{"longblob", true}, {"char", false}, {"varchar", false}, {"text", false}, {"int", false}},
}

// memsqlBinaryProbe is the MySQL probe for SingleStore, whose URL names no
// database, so a table needs one chosen first.
var memsqlBinaryProbe = func() binaryProbe {
	p := mysqlBinaryProbe
	p.setup = append([]string{"CREATE DATABASE IF NOT EXISTS dbm_bin", "USE dbm_bin"}, p.setup...)
	p.cleanup = []string{"DROP DATABASE IF EXISTS dbm_bin"}
	return p
}()

var binaryProbes = map[dbmeta.Dialect]binaryProbe{
	dbmeta.PostgreSQL: {
		setup: []string{
			"CREATE TEMPORARY TABLE dbm_bin (b bytea, t text, u uuid, i integer, bits bit(8))",
			"INSERT INTO dbm_bin VALUES (decode('151c2f00ffab', 'hex'), 'abc', '123e4567-e89b-12d3-a456-426614174000', 1, B'10101010')",
		},
		query: "SELECT b, t, u, i, bits FROM dbm_bin",
		cols:  []binaryColumn{{"bytea", true}, {"text", false}, {"uuid", false}, {"integer", false}, {"bit", false}},
	},
	dbmeta.CockroachDB: {
		setup: []string{
			"CREATE TABLE dbm_bin (b bytea, t text, u uuid, i integer)",
			"INSERT INTO dbm_bin VALUES (decode('151c2f00ffab', 'hex'), 'abc', gen_random_uuid(), 1)",
		},
		query:   "SELECT b, t, u, i FROM dbm_bin",
		cleanup: []string{"DROP TABLE IF EXISTS dbm_bin"},
		cols:    []binaryColumn{{"bytea", true}, {"text", false}, {"uuid", false}, {"integer", false}},
	},
	dbmeta.MySQL: {
		setup: []string{
			"CREATE TEMPORARY TABLE dbm_bin (b binary(8), vb varbinary(8), tb tinyblob, bl blob, mb mediumblob, lb longblob, c char(3), vc varchar(8), t text, j json, i int)",
			"INSERT INTO dbm_bin (b, c, vc, t, j, i) VALUES (x'151c2f00ffab', 'abc', 'abc', 'abc', '{}', 1)",
			"UPDATE dbm_bin SET vb = b, tb = b, bl = b, mb = b, lb = b",
		},
		query: "SELECT b, vb, tb, bl, mb, lb, c, vc, t, j, i FROM dbm_bin",
		cols: []binaryColumn{{"binary", true}, {"varbinary", true}, {"tinyblob", true}, {"blob", true}, {"mediumblob", true},
			{"longblob", true}, {"char", false}, {"varchar", false}, {"text", false}, {"json", false}, {"int", false}},
	},
	dbmeta.SQLServer: {
		setup: []string{
			"CREATE TABLE #dbm_bin (b binary(8), vb varbinary(8), vm varbinary(max), im image, rv rowversion, c char(3), vc varchar(8), nv nvarchar(8), u uniqueidentifier, i int)",
			"INSERT INTO #dbm_bin (b, vb, vm, im, c, vc, nv, u, i) VALUES (0x151C2F00FFAB, 0x151C2F00FFAB, 0x151C2F00FFAB, 0x151C2F00FFAB, 'abc', 'abc', N'abc', NEWID(), 1)",
		},
		query: "SELECT b, vb, vm, im, rv, c, vc, nv, u, i FROM #dbm_bin",
		cols: []binaryColumn{{"binary", true}, {"varbinary", true}, {"varbinary max", true}, {"image", true}, {"rowversion", true},
			{"char", false}, {"varchar", false}, {"nvarchar", false}, {"uniqueidentifier", false}, {"int", false}},
	},
	dbmeta.TiDB:   mysqlBinaryProbe,
	dbmeta.MemSQL: memsqlBinaryProbe,
	dbmeta.Vitess: mysqlBinaryProbe,
	dbmeta.Trino: {
		query: "SELECT from_hex('151C2F00FFAB') AS b, 'abc' AS t, 1 AS i, CAST('abc' AS varchar) AS v",
		cols:  []binaryColumn{{"varbinary", true}, {"varchar", false}, {"integer", false}, {"varchar cast", false}},
	},
	dbmeta.Presto: {
		query: "SELECT from_hex('151C2F00FFAB') AS b, 'abc' AS t, 1 AS i, CAST('abc' AS varchar) AS v",
		cols:  []binaryColumn{{"varbinary", true}, {"varchar", false}, {"integer", false}, {"varchar cast", false}},
	},
	dbmeta.HANA: {
		query: "SELECT HEXTOBIN('151C2F00FFAB') AS vb, CAST(HEXTOBIN('151C2F00FFAB') AS VARBINARY(8)) AS v8, " +
			"TO_BLOB(HEXTOBIN('151C2F00FFAB')) AS bl, 'abc' AS t, 1 AS i FROM DUMMY",
		cols: []binaryColumn{{"hextobin", true}, {"varbinary", true}, {"blob", true}, {"varchar", false}, {"integer", false}},
	},
	dbmeta.Vertica: {
		setup: []string{
			"CREATE LOCAL TEMPORARY TABLE dbm_bin (b binary(8), vb varbinary(8), lv long varbinary(100), t varchar(8), i int) ON COMMIT PRESERVE ROWS",
			"INSERT INTO dbm_bin (b, t, i) VALUES (HEX_TO_BINARY('151C2F00FFAB'), 'abc', 1)",
			"UPDATE dbm_bin SET vb = b, lv = 'abc'::long varbinary",
		},
		query: "SELECT b, vb, lv, t, i FROM dbm_bin",
		cols:  []binaryColumn{{"binary", true}, {"varbinary", true}, {"long varbinary", true}, {"varchar", false}, {"int", false}},
	},
	dbmeta.Cassandra: {
		setup: []string{
			"CREATE KEYSPACE IF NOT EXISTS dbm_bin WITH replication = {'class': 'SimpleStrategy', 'replication_factor': 1}",
			"CREATE TABLE dbm_bin.t (id int PRIMARY KEY, b blob, t text, u uuid, i int)",
			"INSERT INTO dbm_bin.t (id, b, t, u, i) VALUES (1, 0x151c2f00ffab, 'abc', 123e4567-e89b-12d3-a456-426614174000, 1)",
		},
		query:   "SELECT b, t, u, i FROM dbm_bin.t",
		cleanup: []string{"DROP KEYSPACE IF EXISTS dbm_bin"},
		fallback: map[string]string{
			"CREATE KEYSPACE IF NOT EXISTS dbm_bin WITH replication = {'class': 'SimpleStrategy', 'replication_factor': 1}": "CREATE KEYSPACE IF NOT EXISTS dbm_bin WITH replication = {'class': 'NetworkTopologyStrategy', 'replication_factor': 1} AND tablets = {'enabled': false}",
		},
		cols: []binaryColumn{{"blob", true}, {"text", false}, {"uuid", false}, {"int", false}},
	},
	dbmeta.Oracle: {
		setup: []string{
			"CREATE TABLE dbm_bin (r raw(8), l long raw, b blob, c clob, v varchar2(8), n number, staff_id raw(16), h raw(16))",
			"INSERT INTO dbm_bin VALUES (hextoraw('151C2F00FFAB'), hextoraw('151C2F00FFAB'), to_blob(hextoraw('151C2F00FFAB')), 'abc', 'abc', 1, hextoraw('5F6CE3362B560FBDE0630501A8C0AF0E'), hextoraw('5F6CE3362B560FBDE0630501A8C0AF0E'))",
		},
		query:   "SELECT r, l, b, c, v, n, staff_id, h FROM dbm_bin",
		cleanup: []string{"DROP TABLE dbm_bin PURGE"},
		cols:    []binaryColumn{{"raw", true}, {"long raw", true}, {"blob", true}, {"clob", false}, {"varchar2", false}, {"number", false}, {"raw16", true}, {"raw16 without id in its name", true}},
	},
}

// TestFormatBinaryIsMeasured makes a table with one row that holds each binary
// type of the product, and a few that are not binary, reads it through each
// driver that dburl names, and holds the model to the one form that every dialect
// writes, which is psql's form for bytea (D258).
func TestFormatBinaryIsMeasured(t *testing.T) {
	for _, tg := range alignTargets {
		probe, ok := binaryProbes[tg.dialect]
		if !ok {
			continue
		}
		for _, driver := range tg.drivers {
			t.Run(string(tg.dialect)+"/"+driver, func(t *testing.T) {
				dsn, ok := tg.dsns[driver]
				if !ok {
					if dsn = os.Getenv(tg.env); dsn == "" {
						t.Skipf("set %s to run against a server", tg.env)
					}
				}
				measureBinary(t, tg, probe, driver, dsn)
			})
		}
	}
}

func measureBinary(t *testing.T, tg alignTarget, probe binaryProbe, driver, dsn string) {
	t.Helper()
	ctx := context.WithoutCancel(t.Context())
	db := openAt(t, driver, dsn)
	conn, err := db.Conn(ctx)
	if err != nil {
		t.Fatalf("connecting: %v", err)
	}
	defer conn.Close()
	for _, q := range tg.before {
		if _, err := conn.ExecContext(ctx, q); err != nil {
			t.Fatalf("preparing the session: %v", err)
		}
	}
	for _, q := range tg.after {
		defer conn.ExecContext(ctx, q) //nolint:errcheck // a cleanup at the end is best effort
	}
	for _, q := range probe.cleanup {
		conn.ExecContext(ctx, q)       //nolint:errcheck // what an earlier run left is not a fault
		defer conn.ExecContext(ctx, q) //nolint:errcheck // a cleanup at the end is best effort
	}
	for _, q := range probe.setup {
		_, err := conn.ExecContext(ctx, q)
		if err != nil && probe.fallback[q] != "" {
			_, err = conn.ExecContext(ctx, probe.fallback[q])
		}
		if err != nil {
			t.Fatalf("making the probe table: %v\n%s", err, q)
		}
	}
	rows, err := conn.QueryContext(ctx, probe.query)
	if err != nil {
		t.Fatalf("reading the probe table: %v", err)
	}
	defer rows.Close()
	types, err := rows.ColumnTypes()
	if err != nil {
		t.Fatalf("reading the column types: %v", err)
	}
	if !rows.Next() {
		t.Fatalf("the probe table has no row: %v", rows.Err())
	}
	values := make([]any, len(types))
	targets := make([]any, len(types))
	for i := range values {
		targets[i] = &values[i]
	}
	if err := rows.Scan(targets...); err != nil {
		t.Fatalf("scanning the row: %v", err)
	}
	for i, c := range probe.cols {
		ct := types[i]
		b, isBytes := values[i].([]byte)
		got, ok := tg.dialect.FormatBinary(ct, []byte("x"))
		t.Logf("%-18s reports %-16q scans %-12T format %v", c.name, ct.DatabaseTypeName(), values[i], ok)
		if !c.binary {
			if ok {
				t.Errorf("%s: the model formats a column that is not binary, as %q", c.name, got)
			}
			continue
		}
		if !isBytes {
			t.Errorf("%s: the driver scans it as %T, and the expected type is []byte", c.name, values[i])
			continue
		}
		got, ok = tg.dialect.FormatBinary(ct, b)
		if !ok {
			t.Errorf("%s: the driver reports %q, and the model does not format it", c.name, ct.DatabaseTypeName())
			continue
		}
		want := `\x` + hex.EncodeToString(b)
		if c.name == "raw16" {
			// An Oracle RAW(16) with id in its name is a GUID and prints as a UUID
			// (D263).
			h := hex.EncodeToString(b)
			want = h[:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:]
		}
		if got != want {
			t.Errorf("%s: got %s, and the expected form is %s", c.name, got, want)
		}
	}
}
