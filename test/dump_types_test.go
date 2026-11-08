package test

import (
	"database/sql"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/xo/dbmeta"
	"github.com/xo/dbmeta/dump"
	"github.com/xo/dbmeta/models/mysql"
)

// typeCase is one column of a table that the dump type tests build.
type typeCase struct {
	name string
	// def is the declared type, as the server accepts it.
	def string
	// kind is the kind that the model must read from the column.
	kind dump.Kind
	// none says that the product has no type for the kind, so rendering must
	// return ErrNoEquivalent.
	none bool
	// lossy says that the rendering is at the Lossy level.
	lossy bool
	// back is the type that the copy reads as, when it differs from the source,
	// which is the case for a type that renders as a wider one.
	back *dump.Type
}

// roundTripTypes reads the types of real columns, checks their kinds, renders
// each one back in the same dialect, creates a table from the rendered types,
// and reads that table again. The kinds must agree, and every rendering must be
// at the level the case says. See D274.
//
// The tables are named dump_source and dump_copy. qualify puts a schema in
// front of the name where the product has one.
func roundTripTypes(t *testing.T, db *sql.DB, m *dbmeta.Meta, cases []typeCase, qualify func(string) string, args func(table string) dbmeta.Args) {
	t.Helper()
	d := m.Dialect()
	defs := make([]string, len(cases))
	for i, c := range cases {
		defs[i] = c.name + " " + c.def
	}
	for _, name := range []string{"dump_copy", "dump_source"} {
		_, _ = db.ExecContext(t.Context(), "DROP TABLE IF EXISTS "+qualify(name))
	}
	mustExec(t, db, "CREATE TABLE "+qualify("dump_source")+" ("+strings.Join(defs, ", ")+")")
	t.Cleanup(func() {
		for _, name := range []string{"dump_copy", "dump_source"} {
			_, _ = db.ExecContext(t.Context(), "DROP TABLE IF EXISTS "+qualify(name))
		}
	})
	source := readDumpTypes(t, m, db, args("dump_source"))

	var rendered []string
	for _, c := range cases {
		typ, ok := source[c.name]
		if !ok {
			t.Errorf("the column %s was not read", c.name)
			continue
		}
		if typ.Kind != c.kind {
			t.Errorf("%s (%s) has the kind %s, want %s", c.name, typ.Raw, typ.Kind, c.kind)
		}
		r, err := dump.RenderType(d, typ)
		if c.none {
			if !errors.Is(err, dump.ErrNoEquivalent) {
				t.Errorf("rendering %s (%s) gave %v, want ErrNoEquivalent", c.name, typ.Raw, err)
			}
			continue
		}
		if err != nil {
			t.Errorf("rendering %s (%s): %v", c.name, typ.Raw, err)
			continue
		}
		if want := c.lossy; (r.Level == dump.Lossy) != want || r.Level == dump.Text {
			t.Errorf("%s (%s) rendered as %q at %s, lossy is %v", c.name, typ.Raw, r.SQL, r.Level, want)
		}
		if (r.Level != dump.Exact) != (r.Note != "") {
			t.Errorf("%s: the note %q does not match the level %s", c.name, r.Note, r.Level)
		}
		rendered = append(rendered, c.name+" "+r.SQL)
	}
	mustExec(t, db, "CREATE TABLE "+qualify("dump_copy")+" ("+strings.Join(rendered, ", ")+")")
	copied := readDumpTypes(t, m, db, args("dump_copy"))
	for _, c := range cases {
		got, ok := copied[c.name]
		if !ok {
			continue
		}
		want := source[c.name]
		if c.back != nil {
			want = *c.back
		}
		got.Raw, want.Raw = "", ""
		if c.lossy {
			if got.Kind == dump.Other {
				t.Errorf("%s was rendered and reads back as the kind other", c.name)
			}
			continue
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s was rendered and reads back as %+v, want %+v", c.name, got, want)
		}
	}
}

// readDumpTypes reads the columns of a table and parses the type of each one.
func readDumpTypes(t *testing.T, m *dbmeta.Meta, db *sql.DB, args dbmeta.Args) map[string]dump.Type {
	t.Helper()
	out := map[string]dump.Type{}
	for c, err := range dbmeta.Columns.Each(t.Context(), m, db, args) {
		if err != nil {
			t.Fatalf("reading the columns of %s: %v", args.Parent, err)
		}
		typ, err := dump.ParseType(m.Dialect(), c.DataType)
		if err != nil {
			t.Fatalf("parsing the type of %s: %v", c.Name, err)
		}
		out[c.Name] = typ
	}
	return out
}

// mustExec runs a statement and fails the test on an error.
func mustExec(t *testing.T, db *sql.DB, stmt string) {
	t.Helper()
	if _, err := db.ExecContext(t.Context(), stmt); err != nil {
		t.Fatalf("%s: %v", stmt, err)
	}
}

// TestPostgreSQLTypesRoundTrip is the round trip for PostgreSQL, on both drivers.
func TestPostgreSQLTypesRoundTrip(t *testing.T) {
	interval := dump.Type{Kind: dump.Interval}
	cases := []typeCase{
		{name: "c_smallint", def: "smallint", kind: dump.Integer},
		{name: "c_integer", def: "integer", kind: dump.Integer},
		{name: "c_bigint", def: "bigint", kind: dump.Integer},
		{name: "c_real", def: "real", kind: dump.Float},
		{name: "c_double", def: "double precision", kind: dump.Float},
		{name: "c_numeric", def: "numeric", kind: dump.Decimal},
		{name: "c_numeric_ps", def: "numeric(10,2)", kind: dump.Decimal},
		{name: "c_char", def: "char(5)", kind: dump.String},
		{name: "c_varchar", def: "varchar(20)", kind: dump.String},
		{name: "c_text", def: "text", kind: dump.String},
		{name: "c_bytea", def: "bytea", kind: dump.Binary},
		{name: "c_boolean", def: "boolean", kind: dump.Boolean},
		{name: "c_date", def: "date", kind: dump.Date},
		{name: "c_time", def: "time", kind: dump.TimeOfDay},
		{name: "c_time3", def: "time(3)", kind: dump.TimeOfDay},
		{name: "c_timetz", def: "time with time zone", kind: dump.TimeOfDayOffset},
		{name: "c_timestamp", def: "timestamp", kind: dump.LocalTimestamp},
		{name: "c_timestamp3", def: "timestamp(3)", kind: dump.LocalTimestamp},
		{name: "c_timestamptz", def: "timestamp with time zone", kind: dump.Timestamp},
		{name: "c_interval", def: "interval", kind: dump.Interval},
		{name: "c_interval_ym", def: "interval year to month", kind: dump.Interval, back: &interval},
		{name: "c_uuid", def: "uuid", kind: dump.UUID},
		{name: "c_json", def: "json", kind: dump.JSON},
		{name: "c_jsonb", def: "jsonb", kind: dump.JSON},
		{name: "c_inet", def: "inet", kind: dump.IPAddress},
		{name: "c_cidr", def: "cidr", kind: dump.IPAddress},
		{name: "c_int_array", def: "integer[]", kind: dump.Array},
		{name: "c_varchar_array", def: "varchar(10)[]", kind: dump.Array},
		{name: "c_money", def: "money", kind: dump.Other, none: true},
		{name: "c_range", def: "int4range", kind: dump.Range, none: true},
	}
	for _, driver := range postgresDrivers {
		t.Run(driver, func(t *testing.T) {
			db := openPostgresWith(t, driver)
			m := metaFor(t, dbmeta.PostgreSQL, db)
			mustExec(t, db, "DROP SCHEMA IF EXISTS dump_types CASCADE")
			mustExec(t, db, "CREATE SCHEMA dump_types")
			t.Cleanup(func() { _, _ = db.ExecContext(t.Context(), "DROP SCHEMA IF EXISTS dump_types CASCADE") })
			roundTripTypes(t, db, m, cases,
				func(name string) string { return "dump_types." + name },
				func(table string) dbmeta.Args { return dbmeta.Args{Schema: "dump_types", Parent: table} })
		})
	}
}

// TestSQLiteTypesRoundTrip is the round trip for SQLite, on both drivers.
func TestSQLiteTypesRoundTrip(t *testing.T) {
	cases := []typeCase{
		{name: "c_integer", def: "integer", kind: dump.Integer, back: &dump.Type{Kind: dump.Integer, Bits: 64}},
		{name: "c_boolean", def: "boolean", kind: dump.Boolean},
		{name: "c_tinyint", def: "tinyint", kind: dump.Integer, back: &dump.Type{Kind: dump.Integer, Bits: 64}},
		{name: "c_real", def: "real", kind: dump.Float},
		{name: "c_decimal", def: "decimal(10,2)", kind: dump.Decimal, lossy: true},
		{name: "c_char", def: "char(5)", kind: dump.String},
		{name: "c_varchar", def: "varchar(20)", kind: dump.String},
		{name: "c_text", def: "text", kind: dump.String},
		{name: "c_blob", def: "blob", kind: dump.Binary},
		{name: "c_date", def: "date", kind: dump.Date},
		{name: "c_datetime", def: "datetime", kind: dump.LocalTimestamp},
		{name: "c_timestamp", def: "timestamp", kind: dump.Timestamp},
		{name: "c_json", def: "json", kind: dump.JSON},
		{name: "c_uuid", def: "uuid", kind: dump.UUID, none: true},
		{name: "c_unknown", def: "foo", kind: dump.Other, none: true},
	}
	for _, driver := range sqliteDrivers {
		t.Run(driver, func(t *testing.T) {
			db := openSQLiteWith(t, driver)
			m := metaFor(t, dbmeta.SQLite3, db)
			roundTripTypes(t, db, m, cases,
				func(name string) string { return name },
				func(table string) dbmeta.Args { return dbmeta.Args{Parent: table} })
		})
	}
}

// TestDuckDBTypesRoundTrip is the round trip for DuckDB.
func TestDuckDBTypesRoundTrip(t *testing.T) {
	cases := []typeCase{
		{name: "c_boolean", def: "BOOLEAN", kind: dump.Boolean},
		{name: "c_tinyint", def: "TINYINT", kind: dump.Integer},
		{name: "c_smallint", def: "SMALLINT", kind: dump.Integer},
		{name: "c_integer", def: "INTEGER", kind: dump.Integer},
		{name: "c_bigint", def: "BIGINT", kind: dump.Integer},
		{name: "c_utinyint", def: "UTINYINT", kind: dump.Integer},
		{name: "c_ubigint", def: "UBIGINT", kind: dump.Integer},
		{name: "c_hugeint", def: "HUGEINT", kind: dump.BigInteger},
		{name: "c_float", def: "FLOAT", kind: dump.Float},
		{name: "c_double", def: "DOUBLE", kind: dump.Float},
		{name: "c_decimal", def: "DECIMAL(10,2)", kind: dump.Decimal},
		{name: "c_varchar", def: "VARCHAR", kind: dump.String},
		{name: "c_blob", def: "BLOB", kind: dump.Binary},
		{name: "c_date", def: "DATE", kind: dump.Date},
		{name: "c_time", def: "TIME", kind: dump.TimeOfDay},
		{name: "c_timestamp", def: "TIMESTAMP", kind: dump.LocalTimestamp},
		{name: "c_timestamp_ms", def: "TIMESTAMP_MS", kind: dump.LocalTimestamp, back: &dump.Type{Kind: dump.LocalTimestamp}},
		{name: "c_timestamp_ns", def: "TIMESTAMP_NS", kind: dump.LocalTimestamp, back: &dump.Type{Kind: dump.LocalTimestamp, Precision: 9}},
		{name: "c_timestamptz", def: "TIMESTAMPTZ", kind: dump.Timestamp},
		{name: "c_interval", def: "INTERVAL", kind: dump.Interval},
		{name: "c_uuid", def: "UUID", kind: dump.UUID},
		{name: "c_json", def: "JSON", kind: dump.JSON},
		{name: "c_array", def: "INTEGER[]", kind: dump.Array},
		{name: "c_struct", def: "STRUCT(a INTEGER, b VARCHAR)", kind: dump.Tuple, none: true},
	}
	db := openDuckDB(t)
	m := metaFor(t, dbmeta.DuckDB, db)
	roundTripTypes(t, db, m, cases,
		func(name string) string { return name },
		func(table string) dbmeta.Args { return dbmeta.Args{Parent: table} })
}

// TestMySQLTypesRoundTrip is the round trip for MySQL and MariaDB.
func TestMySQLTypesRoundTrip(t *testing.T) {
	db := openMySQL(t)
	m := metaFor(t, dbmeta.MySQL, db)
	// MariaDB keeps a JSON column as longtext with a json_valid check, so the
	// text of the type says string. A snapshot reads the check to find the
	// idiom. See docs/DUMP.md.
	jsonKind, jsonBack := dump.JSON, dump.Type{Kind: dump.JSON}
	if m.Version().Has(mysql.MariaDB) {
		jsonKind, jsonBack = dump.String, dump.Type{Kind: dump.String}
	}
	unsignedYear := dump.Type{Kind: dump.Integer, Bits: 16, Unsigned: true}
	cases := []typeCase{
		{name: "c_boolean", def: "tinyint(1)", kind: dump.Boolean},
		{name: "c_bit1", def: "bit(1)", kind: dump.Boolean, back: &dump.Type{Kind: dump.Boolean}},
		{name: "c_tinyint", def: "tinyint", kind: dump.Integer},
		{name: "c_smallint", def: "smallint", kind: dump.Integer},
		{name: "c_int", def: "int", kind: dump.Integer},
		{name: "c_bigint", def: "bigint", kind: dump.Integer},
		{name: "c_uint", def: "int unsigned", kind: dump.Integer},
		{name: "c_ubigint", def: "bigint unsigned", kind: dump.Integer},
		{name: "c_float", def: "float", kind: dump.Float},
		{name: "c_double", def: "double", kind: dump.Float},
		{name: "c_decimal", def: "decimal(10,2)", kind: dump.Decimal},
		{name: "c_char", def: "char(5)", kind: dump.String},
		{name: "c_varchar", def: "varchar(20)", kind: dump.String},
		{name: "c_text", def: "text", kind: dump.String},
		{name: "c_enum", def: "enum('a','it''s')", kind: dump.String},
		{name: "c_binary", def: "binary(16)", kind: dump.Binary},
		{name: "c_varbinary", def: "varbinary(20)", kind: dump.Binary},
		{name: "c_blob", def: "blob", kind: dump.Binary},
		{name: "c_date", def: "date", kind: dump.Date},
		{name: "c_time", def: "time", kind: dump.TimeOfDay},
		{name: "c_time3", def: "time(3)", kind: dump.TimeOfDay},
		{name: "c_datetime", def: "datetime", kind: dump.LocalTimestamp},
		{name: "c_datetime3", def: "datetime(3)", kind: dump.LocalTimestamp},
		{name: "c_timestamp", def: "timestamp null", kind: dump.Timestamp, lossy: true},
		{name: "c_year", def: "year", kind: dump.Integer, back: &unsignedYear},
		{name: "c_json", def: "json", kind: jsonKind, back: &jsonBack},
		{name: "c_set", def: "set('x','y')", kind: dump.Set, none: true},
		{name: "c_point", def: "point", kind: dump.Geometry, none: true},
	}
	roundTripTypes(t, db, m, cases,
		func(name string) string { return name },
		func(table string) dbmeta.Args { return dbmeta.Args{Parent: table} })
}

// metaFor builds the metadata of a dialect for a connection.
func metaFor(t *testing.T, d dbmeta.Dialect, db *sql.DB) *dbmeta.Meta {
	t.Helper()
	versions, err := d.Version(t.Context(), db)
	if err != nil {
		t.Fatalf("reading the version: %v", err)
	}
	m, err := dbmeta.New(d, versions)
	if err != nil {
		t.Fatalf("building the meta: %v", err)
	}
	return m
}
