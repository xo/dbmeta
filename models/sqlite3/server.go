package sqlite3

import (
	"database/sql"
	"strings"

	"github.com/xo/dbmeta"
)

// Functions, collations and settings.

func registerServer() {
	// \df. pragma_function_list lists every function the library has, built
	// in and loaded alike, once per argument count. The count is folded away
	// here, because psql lists a function once and names its arguments, and
	// SQLite does not name them at all.
	//
	// The kind comes from the type column: s is scalar, a is an aggregate,
	// and w is a function usable over a window. A caller wanting only
	// aggregates cannot have them, and \da is unsupported for the reason
	// recorded in COVERAGE.md.
	dbmeta.Functions.Register(dbmeta.SQLite3, &dbmeta.Binding[dbmeta.Function]{
		Stmt: dbmeta.Stmt{
			always(`SELECT '' AS "catalog"`),
			always(`, '' AS "schema"`),
			always(`, f.name AS "name"`),
			always(`, f.name AS "id"`),
			always(`, CASE f.type WHEN 's' THEN 'func' WHEN 'a' THEN 'agg'` +
				` WHEN 'w' THEN 'window' ELSE f.type END AS "kind"`),
			always(`, '' AS "result_type"`),
			// -1 means the function takes any number of arguments
			always(`, GROUP_CONCAT(CASE WHEN f.narg < 0 THEN 'variadic'` +
				` ELSE CAST(f.narg AS TEXT) END, ', ') AS "arg_types"`),
			always(`, '' AS "volatility"`),
			always(`, '' AS "parallel"`),
			always(`, '' AS "owner"`),
			always(`, '' AS "security"`),
			always(`, NULL AS "access"`),
			always(`, CASE f.builtin WHEN 1 THEN 'c' ELSE 'extension' END AS "language"`),
			always(`, NULL AS "source"`),
			always(`, NULL AS "comment"`),
			always(`FROM pragma_function_list f`),
			always(`WHERE (@name = '' OR f.name LIKE @name)`),
			always(`AND (@schema = '' OR @schema = '')`),
			always(`AND (@with_system OR f.builtin = 0)`),
			always(`GROUP BY f.name, f.type, f.builtin`),
			always(`ORDER BY f.name`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: "always empty: a function belongs to the library, not a database"},
			{Name: "schema", Desc: "always empty, for the same reason"},
			{Name: "name"},
			{Name: "id", Desc: "the name: SQLite has no other identifier for a function"},
			{
				Name: "kind",
				Desc: "func, agg, or window for one usable over a window. SQLite reports most of its aggregates as window, so this cannot separate sum from row_number",
			},
			{Name: "result_type", Desc: "always empty: SQLite functions have no declared result type"},
			{
				Name: "arg_types",
				Desc: "the argument counts this name accepts, or variadic, because SQLite does not name or type its arguments",
			},
			{Name: "volatility", Desc: "always empty: not published"},
			{Name: "parallel", Desc: "always empty: SQLite has no parallel safety marking"},
			{Name: "owner", Desc: "always empty: SQLite has no users"},
			{Name: "security", Desc: "always empty"},
			{Name: "access", Desc: "always absent: SQLite has no grants"},
			{Name: "language", Desc: "c for one built into the library, extension for one the caller registered"},
			{Name: "source", Desc: "always absent: a SQLite function is compiled"},
			{Name: "comment", Desc: "always absent"},
		},
		Params: []dbmeta.Param{
			{Name: "schema", Desc: "ignored: a SQLite function is not in a schema", Default: ""},
			{Name: "name", Desc: "function name pattern, empty for every function", Default: ""},
			{
				Name:    "with_system",
				Desc:    "include the functions built into the library, which is most of them",
				Default: false,
			},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Function, error) {
			var v dbmeta.Function
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Name, &v.ID, &v.Kind, &v.ResultType,
				&v.ArgTypes, &v.Volatility, &v.Parallel, &v.Owner, &v.Security,
				&v.Access, &v.Language, &v.Source, &v.Comment)
			return v, err
		},
	})

	// \dO. SQLite ships three collations and a caller can register more.
	dbmeta.Collations.Register(dbmeta.SQLite3, &dbmeta.Binding[dbmeta.Collation]{
		Stmt: dbmeta.Stmt{
			always(`SELECT '' AS "schema"`),
			always(`, c.name AS "name"`),
			always(`, NULL AS "provider"`),
			always(`, c.name AS "collate"`),
			always(`, c.name AS "ctype"`),
			always(`, NULL AS "locale"`),
			always(`, TRUE AS "deterministic"`),
			always(`, NULL AS "comment"`),
			always(`FROM pragma_collation_list c`),
			always(`WHERE (@name = '' OR c.name LIKE @name)`),
			always(`ORDER BY c.name`),
		},
		Fields: []dbmeta.Field{
			{Name: "schema", Desc: "always empty: a collation belongs to the library"},
			{Name: "name", Desc: "BINARY, NOCASE, RTRIM, or one the caller registered"},
			{Name: "provider", Desc: "always absent: SQLite has one provider and does not name it"},
			{Name: "collate"}, {Name: "ctype"},
			{Name: "locale", Desc: "always absent: a SQLite collation has no locale"},
			{Name: "deterministic", Desc: "always true: SQLite requires it"},
			{Name: "comment", Desc: "always absent"},
		},
		Params: []dbmeta.Param{
			{Name: "name", Desc: "collation name pattern, empty for every one", Default: ""},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Collation, error) {
			var v dbmeta.Collation
			err := rows.Scan(&v.Schema, &v.Name, &v.Provider, &v.Collate, &v.CType,
				&v.Locale, &v.Deterministic, &v.Comment)
			return v, err
		},
	})

	registerSettings()
}

// setting is one pragma this query reads, with how to read it.
type setting struct {
	name string
	// column is the column the pragma returns, when it is not named after the
	// pragma. busy_timeout returns a column called timeout, and it is the
	// only one in this list that does.
	column string
	// text is true when the pragma returns a string rather than a number.
	text bool
	// context says what the value belongs to, the way psql labels a setting.
	context string
	desc    string
}

// settings are the pragmas that hold one value and describe the database or
// the connection.
//
// The list is written out rather than taken from pragma_pragma_list, because
// that pragma gives names and no values, and there is no way to ask SQLite for
// the value of a pragma it names. Reading a value means naming the pragma in
// the SQL, so the set of settings dbmeta reports is the set below.
//
// Left out on purpose: a pragma that returns many rows, such as
// database_list, one that does work rather than reporting, such as optimize or
// integrity_check, and one that is write only. compile_options is left out
// because a build flag is not a setting a caller can change, which is what
// \dconfig means.
//
// Left out because they are not table valued: mmap_size, wal_autocheckpoint,
// wal_checkpoint, case_sensitive_like and temp_store_directory. A pragma
// appears in pragma_pragma_list whether or not it can be read as a table, and
// only running it says which. Every name below was run against the pinned
// driver, and the integration test runs the whole statement, so a build that
// omits one of these fails there rather than in a caller.
var settings = []setting{
	{name: "analysis_limit", context: "connection", desc: "rows ANALYZE reads per index, 0 for all of them"},
	{name: "application_id", context: "database", desc: "the identifier an application wrote into the file"},
	{name: "auto_vacuum", context: "database", desc: "0 none, 1 full, 2 incremental"},
	{name: "automatic_index", context: "connection"},
	{
		name: "busy_timeout", column: "timeout", context: "connection",
		desc: "milliseconds to wait for a lock",
	},
	{name: "cache_size", context: "connection", desc: "pages, or kibibytes when negative"},
	{name: "cache_spill", context: "connection"},
	{name: "cell_size_check", context: "connection"},
	{name: "checkpoint_fullfsync", context: "connection"},
	{name: "data_version", context: "database", desc: "changes when another connection writes"},
	{name: "defer_foreign_keys", context: "connection"},
	{name: "encoding", text: true, context: "database", desc: "the text encoding of the file"},
	{name: "foreign_keys", context: "connection", desc: "off by default, which surprises people"},
	{name: "freelist_count", context: "database", desc: "pages the file holds and does not use"},
	{name: "fullfsync", context: "connection"},
	{name: "hard_heap_limit", context: "connection", desc: "bytes"},
	{name: "ignore_check_constraints", context: "connection"},
	{name: "journal_mode", text: true, context: "database"},
	{name: "journal_size_limit", context: "database", desc: "bytes"},
	{name: "legacy_alter_table", context: "connection"},
	{name: "locking_mode", text: true, context: "connection"},
	{name: "max_page_count", context: "database", desc: "pages"},
	{name: "page_count", context: "database", desc: "pages the file holds"},
	{name: "page_size", context: "database", desc: "bytes"},
	{name: "query_only", context: "connection"},
	{name: "read_uncommitted", context: "connection"},
	{name: "recursive_triggers", context: "connection"},
	{name: "reverse_unordered_selects", context: "connection"},
	{name: "schema_version", context: "database", desc: "changes when the schema changes"},
	{name: "secure_delete", context: "database"},
	{name: "soft_heap_limit", context: "connection", desc: "bytes"},
	{name: "synchronous", context: "database", desc: "0 off, 1 normal, 2 full, 3 extra"},
	{name: "temp_store", context: "connection", desc: "0 default, 1 file, 2 memory"},
	{name: "threads", context: "connection", desc: "worker threads a query may use"},
	{name: "trusted_schema", context: "connection"},
	{name: "user_version", context: "database", desc: "the number an application wrote into the file"},
}

// registerSettings backs \dconfig.
//
// One SELECT per pragma, joined by UNION ALL. That is the only way to read a
// value: a pragma is a function of no arguments and SQLite offers nothing that
// returns every pragma and its value together. The statement is long and it is
// built once, at init, from the table above.
func registerSettings() {
	var b strings.Builder
	for i, s := range settings {
		if i > 0 {
			b.WriteString("\nUNION ALL SELECT ")
		} else {
			b.WriteString("SELECT ")
		}
		column := s.column
		if column == "" {
			column = s.name
		}
		b.WriteString("'" + s.name + `' AS "name", `)
		if s.text {
			b.WriteString(column)
		} else {
			b.WriteString("CAST(" + column + " AS TEXT)")
		}
		b.WriteString(` AS "value", `)
		kind := "number"
		if s.text {
			kind = "text"
		}
		b.WriteString(`'` + kind + `' AS "type", '` + s.context + `' AS "context"`)
		b.WriteString(`, NULL AS "access" FROM pragma_` + s.name)
	}
	stmt := dbmeta.Stmt{
		// The columns are named rather than starred, because their order has
		// to match the field list and a reader should not have to find the
		// inner SELECT to check it.
		always(`SELECT "name", "value", "type", "context", "access" FROM (`),
		always(b.String()),
		always(`) WHERE (@name = '' OR "name" LIKE @name)`),
		always(`ORDER BY "name"`),
	}
	fieldList := []dbmeta.Field{
		{Name: "name", Desc: "the pragma"},
		{Name: "value", Desc: "the value now, as text"},
		{Name: "type", Desc: "number or text"},
		{
			Name: "context",
			Desc: "database for a setting stored in the file, connection for one that lasts as long as the handle",
		},
		{Name: "access", Desc: "always absent: SQLite has no grants"},
	}
	for i := range fieldList {
		if fieldList[i].Name != "name" {
			continue
		}
		var names []string
		for _, s := range settings {
			names = append(names, s.name)
		}
		fieldList[i].Desc = "the pragma, one of: " + strings.Join(names, ", ")
	}
	dbmeta.Settings.Register(dbmeta.SQLite3, &dbmeta.Binding[dbmeta.Setting]{
		Stmt:   stmt,
		Fields: fieldList,
		Params: []dbmeta.Param{
			{Name: "name", Desc: "setting name pattern, empty for every setting", Default: ""},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Setting, error) {
			var v dbmeta.Setting
			err := rows.Scan(&v.Name, &v.Value, &v.Type, &v.Context, &v.Access)
			return v, err
		},
	})
}
