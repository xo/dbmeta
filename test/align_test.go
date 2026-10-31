package test

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"

	_ "github.com/SAP/go-hdb/driver"
	_ "github.com/apache/arrow-go/v18/arrow/flight/flightsql/driver"
	_ "github.com/beltran/gohive/v2"
	_ "github.com/duckdb/duckdb-go/v2"
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
	_ "github.com/microsoft/go-mssqldb"
	_ "github.com/nakagami/firebirdsql"
	_ "github.com/sclgo/impala-go"
	_ "github.com/sijms/go-ora/v3"
	_ "github.com/vertica/vertica-sql-go"
	_ "github.com/xo/dbimp/avatica"
	_ "github.com/xo/dbimp/clickhouse"
	_ "github.com/xo/dbimp/databend"
	_ "github.com/xo/dbimp/drill"
	_ "github.com/xo/dbimp/druid"
	_ "github.com/xo/dbimp/exasol"
	_ "github.com/xo/dbimp/libsql"
	_ "github.com/xo/dbimp/rqlite"
	_ "github.com/xo/dbimp/trino"
	_ "github.com/ydb-platform/ydb-go-sdk/v3"
	_ "modernc.org/sqlite"

	"github.com/xo/dbmeta"
	_ "github.com/xo/dbmeta/all"
	esfixture "github.com/xo/dbmeta/models/elasticsearch/fixture"
	osfixture "github.com/xo/dbmeta/models/opensearch/fixture"
	"github.com/xo/dbmeta/test/internal/gizmosql"
)

// alignColumn is a column of the probe table and the side on which a client
// prints it. optional says that an older release can lack the type, and the
// probe then skips it.
type alignColumn struct {
	ddl      string
	right    bool
	optional bool
	// unnamed lists the drivers that report no name for the type, so that a
	// client has nothing to align by and falls back to the values.
	unnamed []string
}

// alignTarget is one dialect, the drivers that dburl names for it, and the columns
// that measure what each driver reports (D245).
type alignTarget struct {
	dialect dbmeta.Dialect
	env     string
	drivers []string
	// dsns names the DSN of each driver, and a driver with none uses env.
	dsns map[string]string
	// unnamed says that the driver reports no name and no Go type for any column,
	// so the model says nothing and a client aligns by the values.
	unnamed bool
	// setup makes what the probe reads, such as an index, and returns what removes
	// it again.
	setup func(ctx context.Context, dsn string) (func(), error)
	// prep turns the DSN into the one the driver takes, for a server that needs
	// a session first.
	prep func(ctx context.Context, dsn string) (string, error)
	// query is the whole statement of a probe whose language has no SELECT, and the
	// columns it returns are the ones of cols, in order.
	query string
	// from is what follows the expressions of a probe that selects them, such
	// as " FROM dual". A target with no table selects its expressions and
	// reads the types of the result.
	from string
	// before runs first, for a server that needs a database to be chosen.
	before []string
	// after runs at the end, and drop deletes a table that is not temporary.
	after []string
	// create makes the probe table, and add adds one column to it.
	create string
	add    string
	drop   string
	table  string
	cols   []alignColumn
}

var alignTargets = []alignTarget{
	{
		dialect: dbmeta.PostgreSQL, env: "DBMETA_POSTGRES", drivers: []string{"pgx", "postgres"},
		create: "CREATE TEMPORARY TABLE %s (id integer)", add: "ALTER TABLE %s ADD COLUMN %s %s", table: "dbmeta_align",
		cols: []alignColumn{
			{ddl: "smallint", right: true, optional: false}, {ddl: "integer", right: true, optional: false}, {ddl: "bigint", right: true, optional: false},
			{ddl: "real", right: true, optional: false}, {ddl: "double precision", right: true, optional: false}, {ddl: "numeric", right: true, optional: false},
			{ddl: "numeric(10,2)", right: true, optional: false}, {ddl: "money", right: true, optional: false}, {ddl: "oid", right: true, optional: false},
			{ddl: "xid", right: true, optional: false}, {ddl: "cid", right: true, optional: false}, {ddl: "xid8", right: true, optional: true, unnamed: []string{"postgres"}},
			{ddl: "text", right: false, optional: false}, {ddl: "varchar(10)", right: false, optional: false}, {ddl: "char(3)", right: false, optional: false},
			{ddl: "name", right: false, optional: false}, {ddl: "boolean", right: false, optional: false}, {ddl: "timestamptz", right: false, optional: false},
			{ddl: "timestamp", right: false, optional: false}, {ddl: "date", right: false, optional: false}, {ddl: "time", right: false, optional: false},
			{ddl: "interval", right: false, optional: false}, {ddl: "bytea", right: false, optional: false}, {ddl: "json", right: false, optional: false},
			{ddl: "jsonb", right: false, optional: false}, {ddl: "uuid", right: false, optional: false}, {ddl: "inet", right: false, optional: false},
			{ddl: "integer[]", right: false, optional: false}, {ddl: "regclass", right: false, optional: false}, {ddl: "bit(3)", right: false, optional: false},
		},
	},
	{
		dialect: dbmeta.MySQL, env: "DBMETA_MYSQL", drivers: []string{"mysql"},
		before: []string{"CREATE DATABASE IF NOT EXISTS dbmeta_align", "USE dbmeta_align"},
		after:  []string{"DROP DATABASE IF EXISTS dbmeta_align"},
		create: "CREATE TEMPORARY TABLE %s (id integer)", add: "ALTER TABLE %s ADD COLUMN %s %s", table: "dbmeta_align",
		cols: []alignColumn{
			{ddl: "tinyint", right: true, optional: false}, {ddl: "smallint", right: true, optional: false}, {ddl: "mediumint", right: true, optional: false},
			{ddl: "int", right: true, optional: false}, {ddl: "bigint", right: true, optional: false}, {ddl: "tinyint unsigned", right: true, optional: false},
			{ddl: "int unsigned", right: true, optional: false}, {ddl: "bigint unsigned", right: true, optional: false}, {ddl: "float", right: true, optional: false},
			{ddl: "double", right: true, optional: false}, {ddl: "decimal(10,2)", right: true, optional: false}, {ddl: "year", right: true, optional: false},
			{ddl: "bit(8)", right: false, optional: false}, {ddl: "char(3)", right: false, optional: false}, {ddl: "varchar(10)", right: false, optional: false},
			{ddl: "text", right: false, optional: false}, {ddl: "date", right: false, optional: false}, {ddl: "datetime", right: false, optional: false},
			{ddl: "time", right: false, optional: false}, {ddl: "timestamp", right: false, optional: false}, {ddl: "json", right: false, optional: true},
			{ddl: "blob", right: false, optional: false}, {ddl: "enum('a')", right: false, optional: false}, {ddl: "set('a')", right: false, optional: false},
			{ddl: "varbinary(4)", right: false, optional: false},
		},
	},
	{
		dialect: dbmeta.SQLServer, env: "DBMETA_SQLSERVER", drivers: []string{"sqlserver"},
		create: "CREATE TABLE %s (id integer)", add: "ALTER TABLE %s ADD %s %s", table: "#dbmeta_align",
		cols: []alignColumn{
			{ddl: "tinyint", right: true, optional: false}, {ddl: "smallint", right: true, optional: false}, {ddl: "int", right: true, optional: false},
			{ddl: "bigint", right: true, optional: false}, {ddl: "real", right: true, optional: false}, {ddl: "float", right: true, optional: false},
			{ddl: "decimal(10,2)", right: true, optional: false}, {ddl: "numeric(10,2)", right: true, optional: false}, {ddl: "money", right: true, optional: false},
			{ddl: "smallmoney", right: true, optional: false}, {ddl: "bit", right: false, optional: false}, {ddl: "char(3)", right: false, optional: false},
			{ddl: "varchar(10)", right: false, optional: false}, {ddl: "nchar(3)", right: false, optional: false}, {ddl: "nvarchar(10)", right: false, optional: false},
			{ddl: "text", right: false, optional: false}, {ddl: "ntext", right: false, optional: false}, {ddl: "date", right: false, optional: false},
			{ddl: "time", right: false, optional: false}, {ddl: "datetime", right: false, optional: false}, {ddl: "datetime2", right: false, optional: false},
			{ddl: "datetimeoffset", right: false, optional: false}, {ddl: "smalldatetime", right: false, optional: false},
			{ddl: "uniqueidentifier", right: false, optional: false}, {ddl: "binary(4)", right: false, optional: false},
			{ddl: "varbinary(4)", right: false, optional: false}, {ddl: "xml", right: false, optional: false}, {ddl: "sql_variant", right: false, optional: false},
		},
	},
	{
		dialect: dbmeta.Oracle, env: "DBMETA_ORACLE", drivers: []string{"oracle"},
		create: "CREATE TABLE %s (id number)", add: "ALTER TABLE %s ADD (%s %s)", table: "dbmeta_align",
		drop: "DROP TABLE %s",
		cols: []alignColumn{
			{ddl: "number", right: true, optional: false}, {ddl: "number(10,2)", right: true, optional: false}, {ddl: "integer", right: true, optional: false},
			{ddl: "float", right: true, optional: false}, {ddl: "binary_float", right: true, optional: false}, {ddl: "binary_double", right: true, optional: false},
			{ddl: "char(3)", right: false, optional: false}, {ddl: "varchar2(10)", right: false, optional: false}, {ddl: "nchar(3)", right: false, optional: false},
			{ddl: "nvarchar2(10)", right: false, optional: false}, {ddl: "date", right: false, optional: false}, {ddl: "timestamp", right: false, optional: false},
			{ddl: "timestamp with time zone", right: false, optional: false}, {ddl: "interval day to second", right: false, optional: false},
			{ddl: "interval year to month", right: false, optional: false}, {ddl: "clob", right: false, optional: false}, {ddl: "blob", right: false, optional: false},
			{ddl: "raw(4)", right: false, optional: false}, {ddl: "rowid", right: false, optional: false},
		},
	},
	{
		dialect: dbmeta.CockroachDB, env: "DBMETA_COCKROACHDB", drivers: []string{"pgx"},
		cols: []alignColumn{
			{ddl: "1::int2", right: true}, {ddl: "1::int4", right: true}, {ddl: "1::int8", right: true},
			{ddl: "1::float4", right: true}, {ddl: "1::float8", right: true}, {ddl: "1::decimal(10,2)", right: true},
			{ddl: "1::decimal", right: true}, {ddl: "1::oid", right: true}, {ddl: "'a'::text"},
			{ddl: "'a'::varchar(3)"}, {ddl: "'a'::char(3)"}, {ddl: "'a'::name"}, {ddl: "true::bool"},
			{ddl: "now()::timestamptz"}, {ddl: "now()::timestamp"}, {ddl: "now()::date"}, {ddl: "now()::time"},
			{ddl: "'1 day'::interval"}, {ddl: "'a'::bytea"}, {ddl: "'{}'::jsonb"}, {ddl: "gen_random_uuid()"},
			{ddl: "'1.2.3.4'::inet"}, {ddl: "ARRAY[1]"}, {ddl: "'1'::bit(3)"},
		},
	},
	{
		dialect: dbmeta.TiDB, env: "DBMETA_TIDB", drivers: []string{"mysql"},
		cols: []alignColumn{
			{ddl: "CAST(1 AS SIGNED)", right: true}, {ddl: "CAST(1 AS UNSIGNED)", right: true},
			{ddl: "CAST(1 AS DECIMAL(10,2))", right: true}, {ddl: "CAST(1 AS FLOAT)", right: true},
			{ddl: "CAST(1 AS DOUBLE)", right: true, optional: true}, {ddl: "1", right: true}, {ddl: "1.5", right: true},
			{ddl: "1.5e0", right: true}, {ddl: "CAST('a' AS CHAR(3))"}, {ddl: "'a'"},
			{ddl: "CAST('2020-01-01' AS DATE)"}, {ddl: "CAST('2020-01-01' AS DATETIME)"},
			{ddl: "CAST('01:00:00' AS TIME)"}, {ddl: "CAST('{}' AS JSON)"}, {ddl: "CAST('a' AS BINARY(3))"},
		},
	},
	{
		dialect: dbmeta.ClickHouse, env: "DBMETA_CLICKHOUSE", drivers: []string{"clickhouse"},
		cols: []alignColumn{
			{ddl: "CAST(1 AS Int8)", right: true}, {ddl: "CAST(1 AS Int16)", right: true},
			{ddl: "CAST(1 AS Int32)", right: true}, {ddl: "CAST(1 AS Int64)", right: true},
			{ddl: "CAST(1 AS Int128)", right: true}, {ddl: "CAST(1 AS Int256)", right: true},
			{ddl: "CAST(1 AS UInt8)", right: true}, {ddl: "CAST(1 AS UInt16)", right: true},
			{ddl: "CAST(1 AS UInt32)", right: true}, {ddl: "CAST(1 AS UInt64)", right: true},
			{ddl: "CAST(1 AS UInt128)", right: true}, {ddl: "CAST(1 AS UInt256)", right: true},
			{ddl: "CAST(1 AS Float32)", right: true}, {ddl: "CAST(1 AS Float64)", right: true},
			{ddl: "CAST(1 AS Decimal(10,2))", right: true}, {ddl: "CAST(1 AS Decimal32(2))", right: true},
			{ddl: "CAST(1 AS Decimal64(2))", right: true}, {ddl: "CAST(1 AS Decimal128(2))", right: true},
			{ddl: "CAST(1 AS Nullable(Int32))", right: true}, {ddl: "CAST(1 AS Nullable(Float64))", right: true},
			{ddl: "CAST(1 AS LowCardinality(UInt8))", right: true, optional: true},
			{ddl: "CAST(1 AS Bool)"}, {ddl: "'a'"}, {ddl: "CAST('a' AS FixedString(3))"},
			{ddl: "CAST('a' AS Nullable(String))"}, {ddl: "toDate('2020-01-01')"}, {ddl: "toDateTime('2020-01-01 00:00:00')"},
			{ddl: "toDateTime64('2020-01-01 00:00:00', 3)"}, {ddl: "toUUID('00000000-0000-0000-0000-000000000000')"},
			{ddl: "toIPv4('1.2.3.4')"}, {ddl: "[1, 2]"}, {ddl: "(1, 'a')"}, {ddl: "map('a', 1)"},
			{ddl: "CAST('a' AS Enum8('a' = 1))"},
		},
	},
	{
		dialect: dbmeta.CrateDB, env: "DBMETA_CRATEDB", drivers: []string{"pgx"},
		cols: []alignColumn{
			{ddl: "1::smallint", right: true}, {ddl: "1::integer", right: true}, {ddl: "1::bigint", right: true},
			{ddl: "1::real", right: true}, {ddl: "1::double precision", right: true}, {ddl: "1::numeric", right: true},
			{ddl: "1::numeric(10,2)", right: true, optional: true},
			{ddl: "'a'::text"}, {ddl: "true::boolean"}, {ddl: "now()::timestamp"}, {ddl: "now()::timestamptz"},
			{ddl: "'1.2.3.4'::ip", optional: true}, {ddl: "{a = 1}::object", optional: true}, {ddl: "[1, 2]"},
			{ddl: "'a'::char", optional: true}, {ddl: "'a'::varchar", optional: true},
		},
	},
	{
		dialect: dbmeta.QuestDB, env: "DBMETA_QUESTDB", drivers: []string{"pgx"},
		cols: []alignColumn{
			{ddl: "CAST(1 AS BYTE)", right: true}, {ddl: "CAST(1 AS SHORT)", right: true},
			{ddl: "CAST(1 AS INT)", right: true}, {ddl: "CAST(1 AS LONG)", right: true},
			{ddl: "CAST(1 AS FLOAT)", right: true}, {ddl: "CAST(1 AS DOUBLE)", right: true, optional: true},
			{ddl: "CAST(1 AS DECIMAL(10,2))", right: true, optional: true},
			{ddl: "CAST(1 AS LONG256)", optional: true}, {ddl: "CAST(1 AS BOOLEAN)"}, {ddl: "CAST('a' AS STRING)"},
			{ddl: "CAST('a' AS SYMBOL)"}, {ddl: "CAST('a' AS CHAR)"}, {ddl: "CAST(1 AS TIMESTAMP)"},
			{ddl: "CAST(1 AS DATE)"}, {ddl: "CAST('00000000-0000-0000-0000-000000000000' AS UUID)"},
			{ddl: "CAST('1.2.3.4' AS IPV4)", optional: true}, {ddl: "CAST('a' AS VARCHAR)", optional: true},
		},
	},
	{
		dialect: dbmeta.MemSQL, env: "DBMETA_MEMSQL", drivers: []string{"mysql"},
		cols: []alignColumn{
			{ddl: "CAST(1 AS SIGNED)", right: true}, {ddl: "CAST(1 AS UNSIGNED)", right: true},
			{ddl: "CAST(1 AS DECIMAL(10,2))", right: true}, {ddl: "CAST(1 AS DOUBLE)", right: true, optional: true},
			{ddl: "CAST(1 AS FLOAT)", right: true, optional: true}, {ddl: "1", right: true}, {ddl: "1.5", right: true},
			{ddl: "1.5e0", right: true}, {ddl: "CAST('a' AS CHAR(3))"}, {ddl: "'a'"},
			{ddl: "CAST('2020-01-01' AS DATE)"}, {ddl: "CAST('2020-01-01' AS DATETIME)"},
			{ddl: "CAST('01:00:00' AS TIME)"}, {ddl: "CAST('{}' AS JSON)", optional: true},
			{ddl: "CAST('a' AS BINARY(3))"},
		},
	},
	{
		dialect: dbmeta.Vitess, env: "DBMETA_VITESS", drivers: []string{"mysql"},
		cols: []alignColumn{
			{ddl: "CAST(1 AS SIGNED)", right: true}, {ddl: "CAST(1 AS UNSIGNED)", right: true},
			{ddl: "CAST(1 AS DECIMAL(10,2))", right: true}, {ddl: "CAST(1 AS DOUBLE)", right: true, optional: true},
			{ddl: "CAST(1 AS FLOAT)", right: true, optional: true}, {ddl: "1", right: true}, {ddl: "1.5", right: true},
			{ddl: "1.5e0", right: true}, {ddl: "CAST('a' AS CHAR(3))"}, {ddl: "'a'"},
			{ddl: "CAST('2020-01-01' AS DATE)"}, {ddl: "CAST('2020-01-01' AS DATETIME)"},
			{ddl: "CAST('01:00:00' AS TIME)"}, {ddl: "CAST('{}' AS JSON)", optional: true},
			{ddl: "CAST('a' AS BINARY(3))"},
		},
	},
	{
		dialect: dbmeta.LibSQL, env: "DBMETA_LIBSQL", drivers: []string{"libsql"},
		create: "CREATE TABLE %s (id integer)", add: "ALTER TABLE %s ADD COLUMN %s %s", drop: "DROP TABLE %s", table: "dbmeta_align",
		cols: []alignColumn{
			{ddl: "integer", right: true}, {ddl: "int", right: true}, {ddl: "bigint", right: true},
			{ddl: "smallint", right: true}, {ddl: "real", right: true}, {ddl: "double", right: true},
			{ddl: "float", right: true}, {ddl: "numeric", right: true}, {ddl: "decimal(10,2)", right: true},
			{ddl: "boolean"}, {ddl: "text"}, {ddl: "varchar(10)"}, {ddl: "blob"}, {ddl: "date"}, {ddl: "datetime"},
		},
	},
	{
		dialect: dbmeta.Rqlite, env: "DBMETA_RQLITE", drivers: []string{"rqlite"},
		create: "CREATE TABLE %s (id integer)", add: "ALTER TABLE %s ADD COLUMN %s %s", drop: "DROP TABLE %s", table: "dbmeta_align",
		cols: []alignColumn{
			{ddl: "integer", right: true}, {ddl: "int", right: true}, {ddl: "bigint", right: true},
			{ddl: "smallint", right: true}, {ddl: "real", right: true}, {ddl: "double", right: true},
			{ddl: "float", right: true}, {ddl: "numeric", right: true}, {ddl: "decimal(10,2)", right: true},
			{ddl: "boolean"}, {ddl: "text"}, {ddl: "varchar(10)"}, {ddl: "blob"}, {ddl: "date"}, {ddl: "datetime"},
		},
	},
	{
		dialect: dbmeta.GizmoSQL, env: "DBMETA_GIZMOSQL", drivers: []string{"flightsql"},
		prep: gizmosql.DSN, unnamed: true,
		cols: []alignColumn{
			{ddl: "CAST(1 AS TINYINT)", right: true}, {ddl: "CAST(1 AS SMALLINT)", right: true},
			{ddl: "CAST(1 AS INTEGER)", right: true}, {ddl: "CAST(1 AS BIGINT)", right: true},
			{ddl: "CAST(1 AS FLOAT)", right: true},
			{ddl: "CAST(1 AS DOUBLE)", right: true}, {ddl: "CAST(1 AS DECIMAL(10,2))", right: true},
			{ddl: "CAST(1 AS BOOLEAN)"}, {ddl: "CAST('a' AS VARCHAR)"}, {ddl: "CAST('2020-01-01' AS DATE)"},
			{ddl: "CAST('2020-01-01' AS TIMESTAMP)"}, {ddl: "CAST('a' AS BLOB)"}, {ddl: "CAST('01:00:00' AS TIME)"},
		},
	},
	{
		dialect: dbmeta.Databend, env: "DBMETA_DATABEND", drivers: []string{"databend"},
		cols: []alignColumn{
			{ddl: "CAST(1 AS TINYINT)", right: true}, {ddl: "CAST(1 AS SMALLINT)", right: true},
			{ddl: "CAST(1 AS INT)", right: true}, {ddl: "CAST(1 AS BIGINT)", right: true},
			{ddl: "CAST(1 AS TINYINT UNSIGNED)", right: true}, {ddl: "CAST(1 AS BIGINT UNSIGNED)", right: true},
			{ddl: "CAST(1 AS FLOAT)", right: true}, {ddl: "CAST(1 AS DOUBLE)", right: true},
			{ddl: "CAST(1 AS DECIMAL(10,2))", right: true}, {ddl: "CAST(1 AS BOOLEAN)"}, {ddl: "'a'"},
			{ddl: "CAST('2020-01-01' AS DATE)"}, {ddl: "CAST('2020-01-01 00:00:00' AS TIMESTAMP)"},
			{ddl: "PARSE_JSON('{}')"}, {ddl: "[1, 2]"}, {ddl: "TO_BINARY('a')", optional: true},
			{ddl: "CAST(1 AS Nullable(INT))", right: true, optional: true},
		},
	},
	{
		dialect: dbmeta.Vertica, env: "DBMETA_VERTICA", drivers: []string{"vertica"},
		cols: []alignColumn{
			{ddl: "1::INTEGER", right: true}, {ddl: "1::SMALLINT", right: true, optional: true},
			{ddl: "1::BIGINT", right: true, optional: true}, {ddl: "1::FLOAT", right: true},
			{ddl: "1::NUMERIC(10,2)", right: true}, {ddl: "1::DECIMAL(10,2)", right: true},
			{ddl: "1::DOUBLE PRECISION", right: true, optional: true}, {ddl: "1::MONEY", right: true, optional: true},
			{ddl: "true::BOOLEAN"}, {ddl: "'a'::VARCHAR(3)"}, {ddl: "'a'::CHAR(3)"}, {ddl: "'a'::LONG VARCHAR", optional: true},
			{ddl: "'2020-01-01'::DATE"}, {ddl: "'2020-01-01'::TIMESTAMP"}, {ddl: "'2020-01-01'::TIMESTAMPTZ"},
			{ddl: "'01:00:00'::TIME"}, {ddl: "'1 day'::INTERVAL"}, {ddl: "'a'::VARBINARY(3)"},
			{ddl: "'00000000-0000-0000-0000-000000000000'::UUID"},
		},
	},
	{
		dialect: dbmeta.Exasol, env: "DBMETA_EXASOL", drivers: []string{"exasol"}, from: " FROM DUAL",
		cols: []alignColumn{
			{ddl: "CAST(1 AS SMALLINT)", right: true}, {ddl: "CAST(1 AS INTEGER)", right: true},
			{ddl: "CAST(1 AS BIGINT)", right: true}, {ddl: "CAST(1 AS DECIMAL(10,2))", right: true},
			{ddl: "CAST(1 AS DECIMAL(36,2))", right: true}, {ddl: "CAST(1 AS DOUBLE)", right: true},
			{ddl: "CAST(1 AS BOOLEAN)"}, {ddl: "CAST('a' AS VARCHAR(3))"}, {ddl: "CAST('a' AS CHAR(3))"},
			{ddl: "CAST('2020-01-01' AS DATE)"}, {ddl: "CAST('2020-01-01 00:00:00' AS TIMESTAMP)"},
			{ddl: "CAST('2020-01-01 00:00:00' AS TIMESTAMP WITH LOCAL TIME ZONE)"},
			{ddl: "CAST('1 00:00:00' AS INTERVAL DAY TO SECOND)"}, {ddl: "CAST('1-1' AS INTERVAL YEAR TO MONTH)"},
			{ddl: "CAST('a' AS HASHTYPE(16 BYTE))", optional: true},
		},
	},
	{
		dialect: dbmeta.Firebird, env: "DBMETA_FIREBIRDSQL", drivers: []string{"firebirdsql"}, from: " FROM RDB$DATABASE",
		cols: []alignColumn{
			{ddl: "CAST(1 AS SMALLINT)", right: true}, {ddl: "CAST(1 AS INTEGER)", right: true},
			{ddl: "CAST(1 AS BIGINT)", right: true}, {ddl: "CAST(1 AS INT128)", right: true, optional: true},
			{ddl: "CAST(1 AS FLOAT)", right: true}, {ddl: "CAST(1 AS DOUBLE PRECISION)", right: true},
			{ddl: "CAST(1 AS NUMERIC(4,2))", right: true}, {ddl: "CAST(1 AS NUMERIC(9,2))", right: true},
			{ddl: "CAST(1 AS NUMERIC(18,2))", right: true}, {ddl: "CAST(1 AS DECIMAL(10,2))", right: true},
			{ddl: "CAST(1 AS DECFLOAT(16))", right: true, optional: true}, {ddl: "CAST(TRUE AS BOOLEAN)", optional: true},
			{ddl: "CAST('a' AS VARCHAR(3))"}, {ddl: "CAST('a' AS CHAR(3))"}, {ddl: "CAST('2020-01-01' AS DATE)"},
			{ddl: "CAST('01:00:00' AS TIME)"}, {ddl: "CAST('2020-01-01 00:00:00' AS TIMESTAMP)"},
			{ddl: "CAST('a' AS BLOB SUB_TYPE TEXT)"}, {ddl: "CAST('a' AS BLOB)"},
		},
	},
	{
		dialect: dbmeta.Trino, env: "DBMETA_TRINO", drivers: []string{"trino"},
		cols: []alignColumn{
			{ddl: "CAST(1 AS TINYINT)", right: true},
			{ddl: "CAST(1 AS SMALLINT)", right: true},
			{ddl: "CAST(1 AS INTEGER)", right: true},
			{ddl: "CAST(1 AS BIGINT)", right: true},
			{ddl: "CAST(1 AS REAL)", right: true},
			{ddl: "CAST(1 AS DOUBLE)", right: true},
			{ddl: "CAST(1 AS DECIMAL(10,2))", right: true},
			{ddl: "CAST(1 AS BOOLEAN)"},
			{ddl: "CAST('a' AS VARCHAR)"},
			{ddl: "CAST('a' AS CHAR(3))"},
			{ddl: "CAST('a' AS VARBINARY)"},
			{ddl: "DATE '2020-01-01'"},
			{ddl: "TIME '01:00:00'"},
			{ddl: "TIMESTAMP '2020-01-01 00:00:00'"},
			{ddl: "TIMESTAMP '2020-01-01 00:00:00 UTC'"},
			{ddl: "INTERVAL '1' DAY"},
			{ddl: "INTERVAL '1' YEAR"},
			{ddl: "JSON '{}'"},
			{ddl: "ARRAY[1]"},
			{ddl: "MAP(ARRAY[1], ARRAY[1])"},
			{ddl: "CAST(ROW(1) AS ROW(a INTEGER))"},
			{ddl: "UUID '00000000-0000-0000-0000-000000000000'"},
			{ddl: "IPADDRESS '1.2.3.4'"},
		},
	},
	{
		dialect: dbmeta.Presto, env: "DBMETA_PRESTO", drivers: []string{"trino"},
		cols: []alignColumn{
			{ddl: "CAST(1 AS TINYINT)", right: true},
			{ddl: "CAST(1 AS SMALLINT)", right: true},
			{ddl: "CAST(1 AS INTEGER)", right: true},
			{ddl: "CAST(1 AS BIGINT)", right: true},
			{ddl: "CAST(1 AS REAL)", right: true},
			{ddl: "CAST(1 AS DOUBLE)", right: true},
			{ddl: "CAST(1 AS DECIMAL(10,2))", right: true},
			{ddl: "CAST(1 AS BOOLEAN)"},
			{ddl: "CAST('a' AS VARCHAR)"},
			{ddl: "CAST('a' AS CHAR(3))"},
			{ddl: "CAST('a' AS VARBINARY)"},
			{ddl: "DATE '2020-01-01'"},
			{ddl: "TIME '01:00:00'"},
			{ddl: "TIMESTAMP '2020-01-01 00:00:00'"},
			{ddl: "INTERVAL '1' DAY"},
			{ddl: "JSON '{}'"},
			{ddl: "ARRAY[1]"},
			{ddl: "MAP(ARRAY[1], ARRAY[1])"},
		},
	},
	{
		dialect: dbmeta.HANA, env: "DBMETA_HDB", drivers: []string{"hdb"}, from: " FROM DUMMY",
		cols: []alignColumn{
			{ddl: "CAST(1 AS TINYINT)", right: true},
			{ddl: "CAST(1 AS SMALLINT)", right: true},
			{ddl: "CAST(1 AS INTEGER)", right: true},
			{ddl: "CAST(1 AS BIGINT)", right: true},
			{ddl: "CAST(1 AS REAL)", right: true},
			{ddl: "CAST(1 AS DOUBLE)", right: true},
			{ddl: "CAST(1 AS DECIMAL(10,2))", right: true},
			{ddl: "CAST(1 AS DECIMAL(20,2))", right: true}, {ddl: "CAST(1 AS DECIMAL(38,2))", right: true}, {ddl: "CAST(1 AS SMALLDECIMAL)", right: true},
			{ddl: "CAST(1 AS BOOLEAN)"},
			{ddl: "CAST('a' AS VARCHAR(3))"},
			{ddl: "CAST('a' AS NVARCHAR(3))"},
			{ddl: "CAST('2020-01-01' AS DATE)"},
			{ddl: "CAST('01:00:00' AS TIME)"},
			{ddl: "CAST('2020-01-01 00:00:00' AS TIMESTAMP)"},
			{ddl: "CAST('2020-01-01 00:00:00' AS SECONDDATE)"},
			{ddl: "CAST('a' AS VARBINARY(3))"},
			{ddl: "CAST('a' AS CLOB)"},
		},
	},
	{
		dialect: dbmeta.Hive, env: "DBMETA_HIVE", drivers: []string{"hive"},
		cols: []alignColumn{
			{ddl: "CAST(1 AS TINYINT)", right: true},
			{ddl: "CAST(1 AS SMALLINT)", right: true},
			{ddl: "CAST(1 AS INT)", right: true},
			{ddl: "CAST(1 AS BIGINT)", right: true},
			{ddl: "CAST(1 AS FLOAT)", right: true},
			{ddl: "CAST(1 AS DOUBLE)", right: true},
			{ddl: "CAST(1 AS DECIMAL(10,2))", right: true},
			{ddl: "CAST(1 AS BOOLEAN)"},
			{ddl: "CAST('a' AS STRING)"},
			{ddl: "CAST('a' AS VARCHAR(3))"},
			{ddl: "CAST('a' AS CHAR(3))"},
			{ddl: "CAST('2020-01-01' AS DATE)"},
			{ddl: "CAST('2020-01-01 00:00:00' AS TIMESTAMP)"},
			{ddl: "CAST('a' AS BINARY)"},
			{ddl: "ARRAY(1)"},
			{ddl: "MAP('a', 1)"},
			{ddl: "NAMED_STRUCT('a', 1)"},
		},
	},
	{
		dialect: dbmeta.Impala, env: "DBMETA_IMPALA", drivers: []string{"impala"},
		cols: []alignColumn{
			{ddl: "CAST(1 AS TINYINT)", right: true},
			{ddl: "CAST(1 AS SMALLINT)", right: true},
			{ddl: "CAST(1 AS INT)", right: true},
			{ddl: "CAST(1 AS BIGINT)", right: true},
			{ddl: "CAST(1 AS FLOAT)", right: true},
			{ddl: "CAST(1 AS DOUBLE)", right: true},
			{ddl: "CAST(1 AS DECIMAL(10,2))", right: true},
			{ddl: "CAST(1 AS BOOLEAN)"},
			{ddl: "CAST('a' AS STRING)"},
			{ddl: "CAST('a' AS VARCHAR(3))"},
			{ddl: "CAST('a' AS CHAR(3))"},
			{ddl: "CAST('2020-01-01' AS DATE)"},
			{ddl: "CAST('2020-01-01 00:00:00' AS TIMESTAMP)"},
		},
	},
	{
		dialect: dbmeta.Drill, env: "DBMETA_DRILL", drivers: []string{"drill"}, from: " FROM (VALUES(1))",
		cols: []alignColumn{
			{ddl: "CAST(1 AS INTEGER)", right: true},
			{ddl: "CAST(1 AS BIGINT)", right: true},
			{ddl: "CAST(1 AS FLOAT)", right: true},
			{ddl: "CAST(1 AS DOUBLE)", right: true},
			{ddl: "CAST(1 AS DECIMAL(10,2))", right: true},
			{ddl: "CAST(1 AS BOOLEAN)"},
			{ddl: "CAST('a' AS VARCHAR)"},
			{ddl: "CAST('2020-01-01' AS DATE)"},
			{ddl: "CAST('2020-01-01 00:00:00' AS TIMESTAMP)"},
			{ddl: "CAST('01:00:00' AS TIME)"},
		},
	},
	{
		dialect: dbmeta.Druid, env: "DBMETA_DRUID", drivers: []string{"druid"},
		cols: []alignColumn{
			{ddl: "CAST(1 AS BIGINT)", right: true},
			{ddl: "CAST(1 AS INTEGER)", right: true},
			{ddl: "CAST(1 AS FLOAT)", right: true},
			{ddl: "CAST(1 AS DOUBLE)", right: true},
			{ddl: "CAST(1 AS DECIMAL(10,2))", right: true},
			{ddl: "CAST(1 AS BOOLEAN)"},
			{ddl: "CAST('a' AS VARCHAR)"},
			{ddl: "TIMESTAMP '2020-01-01 00:00:00'"},
		},
	},
	{
		dialect: dbmeta.Avatica, env: "DBMETA_AVATICA", drivers: []string{"avatica"}, from: " FROM (VALUES(1))",
		cols: []alignColumn{
			{ddl: "CAST(1 AS TINYINT)", right: true},
			{ddl: "CAST(1 AS SMALLINT)", right: true},
			{ddl: "CAST(1 AS INTEGER)", right: true},
			{ddl: "CAST(1 AS BIGINT)", right: true},
			{ddl: "CAST(1 AS REAL)", right: true},
			{ddl: "CAST(1 AS DOUBLE)", right: true},
			{ddl: "CAST(1 AS DECIMAL(10,2))", right: true},
			{ddl: "CAST(1 AS NUMERIC(10,2))", right: true},
			{ddl: "CAST(1 AS BOOLEAN)"},
			{ddl: "CAST('a' AS VARCHAR(3))"},
			{ddl: "CAST('a' AS CHAR(3))"},
			{ddl: "CAST('2020-01-01' AS DATE)"},
			{ddl: "CAST('2020-01-01 00:00:00' AS TIMESTAMP)"},
			{ddl: "CAST('01:00:00' AS TIME)"},
		},
	},
	{
		dialect: dbmeta.YDB, env: "DBMETA_YDB", drivers: []string{"ydb"},
		cols: []alignColumn{
			{ddl: "CAST(1 AS Int8)", right: true},
			{ddl: "CAST(1 AS Int16)", right: true},
			{ddl: "CAST(1 AS Int32)", right: true},
			{ddl: "CAST(1 AS Int64)", right: true},
			{ddl: "CAST(1 AS Uint8)", right: true},
			{ddl: "CAST(1 AS Uint16)", right: true},
			{ddl: "CAST(1 AS Uint32)", right: true},
			{ddl: "CAST(1 AS Uint64)", right: true},
			{ddl: "CAST(1 AS Float)", right: true},
			{ddl: "CAST(1 AS Double)", right: true},
			{ddl: "CAST(1 AS Decimal(22,9))", right: true},
			{ddl: "CAST(1 AS Bool)"},
			{ddl: "CAST('a' AS Utf8)"},
			{ddl: "CAST('a' AS String)"},
			{ddl: "CAST('2020-01-01' AS Date)"},
			{ddl: "CAST('2020-01-01T00:00:00Z' AS Timestamp)"},
			{ddl: "CAST('2020-01-01T00:00:00Z' AS Datetime)"},
			{ddl: "CAST('{}' AS Json)"},
		},
	},
	{
		dialect: dbmeta.Cassandra, env: "DBMETA_CASSANDRA", drivers: []string{"cassandra"}, from: " FROM system.local",
		cols: []alignColumn{
			// A CAST of a column, because ScyllaDB 2025.1 takes no literal in a
			// selection, ScyllaDB 2026.3 refuses a type hint, and Apache
			// Cassandra 3.11 refuses a CAST of a literal.
			{ddl: "CAST(gossip_generation AS tinyint)", right: true},
			{ddl: "CAST(gossip_generation AS smallint)", right: true},
			{ddl: "gossip_generation", right: true},
			{ddl: "CAST(gossip_generation AS bigint)", right: true},
			{ddl: "CAST(gossip_generation AS varint)", right: true},
			{ddl: "CAST(gossip_generation AS float)", right: true},
			{ddl: "CAST(gossip_generation AS double)", right: true},
			{ddl: "CAST(gossip_generation AS decimal)", right: true},
			{ddl: "CAST(key AS text)"},
			{ddl: "toDate(now())"},
			{ddl: "toTimestamp(now())"},
			{ddl: "(boolean) true", optional: true},
			{ddl: "CAST(toTimestamp(now()) AS time)", optional: true},
		},
	},
	{
		dialect: dbmeta.BigQuery, env: "DBMETA_BIGQUERY", drivers: []string{"bigquery"},
		cols: []alignColumn{
			{ddl: "CAST(1 AS INT64)", right: true},
			{ddl: "CAST(1 AS FLOAT64)", right: true},
			{ddl: "CAST(1 AS NUMERIC)", right: true},
			{ddl: "CAST(1 AS BIGNUMERIC)", right: true},
			{ddl: "TRUE"},
			{ddl: "'a'"},
			{ddl: "b'a'"},
			{ddl: "DATE '2020-01-01'"},
			{ddl: "TIMESTAMP '2020-01-01 00:00:00+00'"},
			{ddl: "DATETIME '2020-01-01 00:00:00'"},
			{ddl: "TIME '01:00:00'"},
			{ddl: "JSON '{}'"},
			{ddl: "[1]"},
		},
	},
	searchTarget(dbmeta.Elasticsearch, "DBMETA_ELASTICSEARCH", "elasticsearch", esCallDSN, true),
	searchTarget(dbmeta.OpenSearch, "DBMETA_OPENSEARCH", "opensearch", osCallDSN, false),
	{
		dialect: dbmeta.Neo4j, env: "DBMETA_NEO4J", drivers: []string{"neo4j"}, unnamed: true, query: `RETURN 1 AS c01, 1.5 AS c02, 'a' AS c03, true AS c04, date('2020-01-01') AS c05, datetime('2020-01-01T00:00:00Z') AS c06, [1] AS c07, {a: 1} AS c08`,
		cols: []alignColumn{
			{ddl: "1", right: true},
			{ddl: "1.5", right: true},
			{ddl: "'a'"},
			{ddl: "true"},
			{ddl: "date('2020-01-01')"},
			{ddl: "datetime('2020-01-01T00:00:00Z')"},
			{ddl: "[1]"},
			{ddl: "{a: 1}"},
		},
	},
	{
		dialect: dbmeta.Couchbase, env: "DBMETA_COUCHBASE", drivers: []string{"couchbase"}, query: `SELECT 1 AS c01, 1.5 AS c02, 'a' AS c03, true AS c04, [1] AS c05`,
		cols: []alignColumn{
			{ddl: "1", right: true},
			{ddl: "1.5", right: true},
			{ddl: "'a'"},
			{ddl: "true"},
			{ddl: "[1]"},
		},
	},
	{
		dialect: dbmeta.SurrealDB, env: "DBMETA_SURREALDB", drivers: []string{"surrealdb"}, unnamed: true,
		query: "SELECT 1 AS c01, 1.5f AS c02, 1.5dec AS c03, 'a' AS c04, true AS c05, time::now() AS c06, [1] AS c07, {a: 1} AS c08 FROM ONLY {}",
		cols: []alignColumn{
			{ddl: "1", right: true}, {ddl: "1.5f", right: true}, {ddl: "1.5dec", right: true},
			{ddl: "'a'"}, {ddl: "true"}, {ddl: "time::now()"}, {ddl: "[1]"}, {ddl: "{a: 1}"},
		},
	},
	{
		dialect: dbmeta.InfluxDB, env: "DBMETA_INFLUXDB", drivers: []string{"influxdb"},
		cols: []alignColumn{
			{ddl: "CAST(1 AS TINYINT)", right: true}, {ddl: "CAST(1 AS SMALLINT)", right: true},
			{ddl: "CAST(1 AS INT)", right: true}, {ddl: "CAST(1 AS BIGINT)", right: true},
			{ddl: "CAST(1 AS TINYINT UNSIGNED)", right: true, optional: true}, {ddl: "CAST(1 AS SMALLINT UNSIGNED)", right: true, optional: true},
			{ddl: "CAST(1 AS INT UNSIGNED)", right: true, optional: true}, {ddl: "CAST(1 AS BIGINT UNSIGNED)", right: true, optional: true},
			{ddl: "CAST(1 AS FLOAT)", right: true}, {ddl: "CAST(1 AS DOUBLE)", right: true},
			{ddl: "CAST(1 AS DECIMAL(10,2))", right: true}, {ddl: "CAST(1 AS DECIMAL(40,2))", right: true, optional: true},
			{ddl: "CAST(1 AS BOOLEAN)"}, {ddl: "'a'"}, {ddl: "CAST('2020-01-01' AS DATE)"},
			{ddl: "CAST('2020-01-01 00:00:00' AS TIMESTAMP)"},
		},
	},
	{
		dialect: dbmeta.ArangoDB, env: "DBMETA_ARANGODB", drivers: []string{"arangodb"}, unnamed: true,
		query: "RETURN {c01: 1, c02: 1.5, c03: 'a', c04: true, c05: [1]}",
		cols:  []alignColumn{{ddl: "1", right: true}, {ddl: "1.5", right: true}, {ddl: "'a'"}, {ddl: "true"}, {ddl: "[1]"}},
	},
	{
		dialect: dbmeta.Solr, env: "DBMETA_SOLR", drivers: []string{"solr"},
		setup: func(ctx context.Context, dsn string) (func(), error) {
			doc := `{"add":{"doc":{"id":"dbmeta_align","a_i":1,"b_l":1,"c_f":1.5,"d_d":1.5,"e_s":"a","f_b":true,"g_dt":"2020-01-01T00:00:00Z"}}}`
			if _, err := solrCall(ctx, dsn, "POST", "/solr/dbmeta/update?commit=true&wt=json", doc); err != nil {
				return nil, fmt.Errorf("adding the document: %w", err)
			}
			return func() {
				//nolint:errcheck // a delete at the end is best effort
				solrCall(context.WithoutCancel(ctx), dsn, "POST", "/solr/dbmeta/update?commit=true&wt=json",
					`{"delete":{"id":"dbmeta_align"}}`)
			}, nil
		},
		query: "SELECT a_i, b_l, c_f, d_d, e_s, f_b, g_dt FROM dbmeta WHERE id = 'dbmeta_align'",
		cols: []alignColumn{
			{ddl: "int", right: true}, {ddl: "long", right: true}, {ddl: "float", right: true},
			{ddl: "double", right: true}, {ddl: "string"}, {ddl: "boolean"}, {ddl: "date"},
		},
	},
	{
		dialect: dbmeta.SQLite3, drivers: []string{"sqlite3", "sqlite"},
		dsns:   map[string]string{"sqlite3": ":memory:", "sqlite": ":memory:"},
		create: "CREATE TEMPORARY TABLE %s (id integer)", add: "ALTER TABLE %s ADD COLUMN %s %s", table: "dbmeta_align",
		cols: []alignColumn{
			{ddl: "integer", right: true, optional: false}, {ddl: "int", right: true, optional: false}, {ddl: "bigint", right: true, optional: false},
			{ddl: "smallint", right: true, optional: false}, {ddl: "tinyint", right: true, optional: false}, {ddl: "real", right: true, optional: false},
			{ddl: "double", right: true, optional: false}, {ddl: "float", right: true, optional: false}, {ddl: "numeric", right: true, optional: false},
			{ddl: "decimal(10,2)", right: true, optional: false}, {ddl: "boolean", right: false, optional: false}, {ddl: "text", right: false, optional: false},
			{ddl: "varchar(10)", right: false, optional: false}, {ddl: "char(3)", right: false, optional: false}, {ddl: "blob", right: false, optional: false},
			{ddl: "date", right: false, optional: false}, {ddl: "datetime", right: false, optional: false}, {ddl: "timestamp", right: false, optional: false},
			{ddl: "json", right: false, optional: false},
		},
	},
	{
		dialect: dbmeta.DuckDB, drivers: []string{"duckdb"}, dsns: map[string]string{"duckdb": ""},
		create: "CREATE TEMPORARY TABLE %s (id integer)", add: "ALTER TABLE %s ADD COLUMN %s %s", table: "dbmeta_align",
		cols: []alignColumn{
			{ddl: "tinyint", right: true, optional: false}, {ddl: "smallint", right: true, optional: false}, {ddl: "integer", right: true, optional: false},
			{ddl: "bigint", right: true, optional: false}, {ddl: "hugeint", right: true, optional: false}, {ddl: "utinyint", right: true, optional: false},
			{ddl: "usmallint", right: true, optional: false}, {ddl: "uinteger", right: true, optional: false}, {ddl: "ubigint", right: true, optional: false},
			{ddl: "uhugeint", right: true, optional: true}, {ddl: "float", right: true, optional: false}, {ddl: "double", right: true, optional: false},
			{ddl: "decimal(10,2)", right: true, optional: false}, {ddl: "boolean", right: false, optional: false}, {ddl: "varchar", right: false, optional: false},
			{ddl: "date", right: false, optional: false}, {ddl: "time", right: false, optional: false}, {ddl: "timestamp", right: false, optional: false},
			{ddl: "timestamptz", right: false, optional: false}, {ddl: "interval", right: false, optional: false}, {ddl: "blob", right: false, optional: false},
			{ddl: "uuid", right: false, optional: false}, {ddl: "json", right: false, optional: true}, {ddl: "integer[]", right: false, optional: false},
		},
	},
}

// TestColumnAlignIsMeasured makes a table with one column of each type, asks each
// driver of the dialect what it calls the type of each column, and holds the
// answer to what the model says (D245). The table is empty, because a driver
// reports the types of an empty result too.
func TestColumnAlignIsMeasured(t *testing.T) {
	for _, tg := range alignTargets {
		for _, driver := range tg.drivers {
			t.Run(string(tg.dialect)+"/"+driver, func(t *testing.T) {
				dsn, ok := tg.dsns[driver]
				if !ok {
					if dsn = os.Getenv(tg.env); dsn == "" {
						t.Skipf("set %s to run against a server", tg.env)
					}
				}
				if tg.setup != nil {
					undo, err := tg.setup(t.Context(), dsn)
					if err != nil {
						t.Fatalf("setting up the probe: %v", err)
					}
					defer undo()
				}
				if tg.prep != nil {
					var err error
					if dsn, err = tg.prep(t.Context(), dsn); err != nil {
						t.Fatalf("preparing the DSN: %v", err)
					}
				}
				measureAlign(t, tg, driver, dsn)
			})
		}
	}
}

func measureAlign(t *testing.T, tg alignTarget, driver, dsn string) {
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
	if tg.table == "" {
		measureSelect(t, tg, driver, conn)
		return
	}
	if tg.drop != "" {
		//nolint:errcheck // a table left by an earlier run is not a fault
		conn.ExecContext(ctx, fmt.Sprintf(tg.drop, tg.table))
		defer conn.ExecContext(ctx, fmt.Sprintf(tg.drop, tg.table)) //nolint:errcheck // a drop at the end is best effort
	}
	if _, err := conn.ExecContext(ctx, fmt.Sprintf(tg.create, tg.table)); err != nil {
		t.Fatalf("making the probe table: %v", err)
	}
	var made []string
	var want []alignColumn
	for i, c := range tg.cols {
		name := fmt.Sprintf("c%02d", i+1)
		if _, err := conn.ExecContext(ctx, fmt.Sprintf(tg.add, tg.table, name, c.ddl)); err != nil {
			if c.optional {
				t.Logf("skipping %s, which this release lacks: %v", c.ddl, err)
				continue
			}
			t.Fatalf("adding a column of type %s: %v", c.ddl, err)
		}
		made = append(made, name)
		want = append(want, c)
	}
	if _, err := conn.ExecContext(ctx, fmt.Sprintf("INSERT INTO %s (id) VALUES (1)", tg.table)); err != nil {
		t.Fatalf("adding a row: %v", err)
	}
	rows, err := conn.QueryContext(ctx, "SELECT "+strings.Join(made, ", ")+" FROM "+tg.table)
	if err != nil {
		t.Fatalf("reading the probe table: %v", err)
	}
	defer rows.Close()
	rows.Next()
	cts, err := rows.ColumnTypes()
	if err != nil {
		t.Fatalf("reading the column types: %v", err)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("reading the rows: %v", err)
	}
	byName := make(map[string]*sql.ColumnType, len(cts))
	for _, ct := range cts {
		byName[strings.ToLower(ct.Name())] = ct
	}
	for i, name := range made {
		ct, ok := byName[name]
		if !ok {
			t.Errorf("the result has no column %s", name)
			continue
		}
		checkAlign(t, tg, driver, want[i], ct)
	}
}

// measureSelect is the probe of a server that has no table to add columns to, or
// that reports the types of a result only: each column is an expression, and a
// column that this release cannot evaluate and is optional is skipped.
func measureSelect(t *testing.T, tg alignTarget, driver string, conn *sql.Conn) {
	t.Helper()
	ctx := context.WithoutCancel(t.Context())
	var exprs []string
	var want []alignColumn
	for _, c := range tg.cols {
		if c.optional {
			if err := probeRuns(ctx, conn, "SELECT "+c.ddl+tg.from); err != nil {
				t.Logf("skipping %s, which this release lacks: %v", c.ddl, err)
				continue
			}
		}
		exprs = append(exprs, fmt.Sprintf("%s AS c%02d", c.ddl, len(exprs)+1))
		want = append(want, c)
	}
	stmt := "SELECT " + strings.Join(exprs, ", ") + tg.from
	if tg.query != "" {
		stmt = tg.query
	}
	rows, err := conn.QueryContext(ctx, stmt)
	if err != nil {
		t.Fatalf("reading the types: %v", err)
	}
	defer rows.Close()
	cts, err := rows.ColumnTypes()
	if err != nil {
		t.Fatalf("reading the column types: %v", err)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("reading the rows: %v", err)
	}
	if len(cts) != len(want) {
		t.Fatalf("expected %d columns, got %d", len(want), len(cts))
	}
	for i, ct := range cts {
		checkAlign(t, tg, driver, want[i], ct)
	}
}

// checkAlign holds one column to what the model says.
func checkAlign(t *testing.T, tg alignTarget, driver string, c alignColumn, ct *sql.ColumnType) {
	t.Helper()
	got, known := tg.dialect.ColumnAlign(ct)
	t.Logf("%-28s reports %-24q scans %-18v aligns %v", c.ddl, ct.DatabaseTypeName(), ct.ScanType(), got)
	switch {
	case tg.unnamed, slices.Contains(c.unnamed, driver):
		if known {
			t.Errorf("%s: the driver now names the type %q, so the exception can go", c.ddl, ct.DatabaseTypeName())
		}
	case !known:
		t.Errorf("%s: the model records no alignment for %q", c.ddl, ct.DatabaseTypeName())
	case (got == dbmeta.AlignRight) != c.right:
		t.Errorf("%s: the driver reports %q, which aligns %v, and a client wants right=%v",
			c.ddl, ct.DatabaseTypeName(), got, c.right)
	}
}

// searchTypes are the field types that the probe of an Elasticsearch product maps,
// one field of each. A field is named c01 and up, so that a result lists them in
// the order of the columns of the probe. right says that the type is a number.
func searchTypes(unsignedLong bool) []alignColumn {
	cols := []alignColumn{
		{ddl: "byte", right: true}, {ddl: "short", right: true}, {ddl: "integer", right: true},
		{ddl: "long", right: true}, {ddl: "float", right: true}, {ddl: "half_float", right: true},
		{ddl: "scaled_float", right: true}, {ddl: "double", right: true},
	}
	if unsignedLong {
		cols = append(cols, alignColumn{ddl: "unsigned_long", right: true})
	}
	return append(cols, alignColumn{ddl: "boolean"}, alignColumn{ddl: "keyword"}, alignColumn{ddl: "text"},
		alignColumn{ddl: "date"}, alignColumn{ddl: "ip"}, alignColumn{ddl: "binary"})
}

// searchTarget is the probe of a product with the SQL interface of Elasticsearch,
// which has no SELECT of an expression without an index, so the probe makes an
// index with one field of each type and reads it.
func searchTarget(d dbmeta.Dialect, env, driver string,
	call func(ctx context.Context, dsn, method, path, body string) (int, []byte, error), unsignedLong bool) alignTarget {
	cols := searchTypes(unsignedLong)
	var mapping strings.Builder
	var names []string
	mapping.WriteString(`{"mappings":{"properties":{`)
	for i, c := range cols {
		name := fmt.Sprintf("c%02d", i+1)
		names = append(names, name)
		if i > 0 {
			mapping.WriteString(",")
		}
		extra := ""
		if c.ddl == "scaled_float" {
			extra = `,"scaling_factor":100`
		}
		fmt.Fprintf(&mapping, `%q:{"type":%q%s}`, name, c.ddl, extra)
	}
	mapping.WriteString(`}}}`)
	return alignTarget{
		dialect: d, env: env, drivers: []string{driver},
		setup: alignIndex(call, mapping.String()),
		query: "SELECT " + strings.Join(names, ", ") + " FROM dbmeta_align",
		cols:  cols,
	}
}

// esCallDSN and osCallDSN send one request as the user of the DSN, for the two
// products with the same HTTP interface.
func esCallDSN(ctx context.Context, dsn, method, path, body string) (int, []byte, error) {
	return esCall(ctx, dsn, esfixture.Request{Method: method, Path: path, Body: body})
}

func osCallDSN(ctx context.Context, dsn, method, path, body string) (int, []byte, error) {
	return osCall(ctx, dsn, osfixture.Request{Method: method, Path: path, Body: body})
}

// alignIndex returns a setup that makes the index dbmeta_align with the mapping,
// removes one that an earlier run left, and returns the function that removes it.
func alignIndex(call func(ctx context.Context, dsn, method, path, body string) (int, []byte, error), mapping string) func(context.Context, string) (func(), error) {
	return func(ctx context.Context, dsn string) (func(), error) {
		//nolint:errcheck // an index that is not there is not a fault
		call(ctx, dsn, "DELETE", "/dbmeta_align", "")
		status, body, err := call(ctx, dsn, "PUT", "/dbmeta_align", mapping)
		if err != nil {
			return nil, fmt.Errorf("making the index: %w", err)
		}
		if status >= 400 {
			return nil, fmt.Errorf("making the index: status %d: %s", status, body)
		}
		return func() { call(context.WithoutCancel(ctx), dsn, "DELETE", "/dbmeta_align", "") }, nil //nolint:errcheck // a drop at the end is best effort
	}
}

// probeRuns runs the statement and reports whether the server accepts it.
func probeRuns(ctx context.Context, conn *sql.Conn, stmt string) error {
	rows, err := conn.QueryContext(ctx, stmt)
	if err != nil {
		return fmt.Errorf("running the probe: %w", err)
	}
	defer rows.Close()
	return rows.Err()
}
