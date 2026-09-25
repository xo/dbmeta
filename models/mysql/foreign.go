package mysql

import (
	"database/sql"

	"github.com/xo/dbmeta"
)

// Aggregates and foreign data.
//
// Every query in this file reads a table in the mysql schema, except the one
// for foreign tables. MariaDB does not publish that schema through
// information_schema, and it does not grant it to an ordinary user, so these
// queries need SELECT on the mysql schema and fail without it. PostgreSQL
// makes the same information readable by everyone, so this is a difference a
// caller has to handle rather than a fault. See docs/COVERAGE.md.

func registerForeign() {
	registerAggregates()
	registerForeignData()
}

// registerAggregates backs \da. MariaDB keeps aggregates in two places and
// neither is a view. A SQL aggregate, written with CREATE AGGREGATE FUNCTION,
// is a row in mysql.proc whose aggregate column reads GROUP. A compiled
// aggregate, loaded with CREATE AGGREGATE FUNCTION ... SONAME, is a row in
// mysql.func whose type column reads aggregate. information_schema.ROUTINES
// lists the first kind and cannot tell it from a plain function, which is why
// this query does not use it.
func registerAggregates() {
	dbmeta.Aggregates.Register(dbmeta.MySQL, &dbmeta.Binding[dbmeta.Function]{
		Stmt: dbmeta.Stmt{
			{frag(mariaAgg, `SELECT 'def' AS "catalog"`)},
			{frag(mariaAgg, `, p.db AS "schema"`)},
			{frag(mariaAgg, `, p.name AS "name"`)},
			{frag(mariaAgg, `, p.name AS "id"`)},
			{frag(mariaAgg, `, 'agg' AS "kind"`)},
			{frag(mariaAgg, `, CONVERT(p.returns USING utf8mb4) AS "result_type"`)},
			{frag(mariaAgg, `, CONVERT(p.param_list USING utf8mb4) AS "arg_types"`)},
			{frag(mariaAgg, `, LOWER(p.is_deterministic) AS "volatility"`)},
			{frag(mariaAgg, `, '' AS "parallel"`)},
			{frag(mariaAgg, `, p.definer AS "owner"`)},
			{frag(mariaAgg, `, LOWER(p.security_type) AS "security"`)},
			{frag(mariaAgg, `, NULL AS "access"`)},
			{frag(mariaAgg, `, 'sql' AS "language"`)},
			{frag(mariaAgg, `, CONVERT(p.body USING utf8mb4) AS "source"`)},
			{frag(mariaAgg, `, NULLIF(CONVERT(p.comment USING utf8mb4), '') AS "comment"`)},
			{frag(mariaAgg, `FROM mysql.proc p`)},
			{frag(mariaAgg, `WHERE p.aggregate = 'GROUP'`)},
			{frag(mariaAgg, `AND (@with_system OR p.db NOT IN (`+systemSchemas+`))`)},
			{frag(mariaAgg, `AND (@schema = '' OR p.db LIKE @schema)`)},
			{frag(mariaAgg, `AND (@name = '' OR p.name LIKE @name)`)},
			// a compiled aggregate is global, so it has no schema to filter on
			// and it appears whatever @schema asks for
			{frag(mariaAgg, `UNION ALL`)},
			{frag(mariaAgg, `SELECT 'def', '', f.name, f.name, 'agg'`)},
			{frag(mariaAgg, `, CASE f.ret WHEN 0 THEN 'string' WHEN 1 THEN 'real' WHEN 2 THEN 'int'`+
				` WHEN 3 THEN 'row' WHEN 4 THEN 'decimal' ELSE CAST(f.ret AS CHAR) END`)},
			{frag(mariaAgg, `, NULL, '', '', '', '', NULL, 'c', f.dl, NULL`)},
			{frag(mariaAgg, `FROM mysql.func f`)},
			{frag(mariaAgg, `WHERE f.type = 'aggregate'`)},
			{frag(mariaAgg, `AND (@name = '' OR f.name LIKE @name)`)},
			{frag(mariaAgg, `ORDER BY 2, 3`)},
		},
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: "always def: MariaDB has one catalog"},
			{Name: "schema", Desc: "empty for a compiled aggregate, which is global"},
			{Name: "name"},
			{Name: "id", Desc: "the name: neither product overloads a routine"},
			{Name: "kind", Desc: "always agg"},
			{Name: "result_type"},
			{Name: "arg_types", Desc: "absent for a compiled aggregate, which declares none"},
			{Name: "volatility", Desc: "yes or no, from is_deterministic, empty for a compiled aggregate"},
			{Name: "parallel", Desc: "always empty: MariaDB has no parallel safety marking"},
			{Name: "owner", Desc: "the definer, empty for a compiled aggregate"},
			{Name: "security", Desc: "empty for a compiled aggregate"},
			{Name: "access", Desc: "always absent: MariaDB grants on the schema, not the routine"},
			{Name: "language", Desc: "sql for a stored aggregate, c for a compiled one"},
			{Name: "source", Desc: "the body, or the library name for a compiled aggregate"},
			{Name: "comment"},
		},
		Params: schemaNameSystem("aggregate"),
		Scan: func(rows *sql.Rows) (dbmeta.Function, error) {
			var v dbmeta.Function
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Name, &v.ID, &v.Kind, &v.ResultType,
				&v.ArgTypes, &v.Volatility, &v.Parallel, &v.Owner, &v.Security,
				&v.Access, &v.Language, &v.Source, &v.Comment)
			return v, err
		},
	})
}

// registerForeignData backs \des, \deu and \det. MariaDB records a foreign
// server with CREATE SERVER and keeps it in mysql.servers. It has no catalog
// of wrappers, so \dew stays unanswered.
func registerForeignData() {
	// \des. The options column is built to read like the one psql prints,
	// because mysql.servers spreads the same settings over columns.
	dbmeta.ForeignServers.Register(dbmeta.MySQL, &dbmeta.Binding[dbmeta.ForeignServer]{
		Stmt: dbmeta.Stmt{
			{{Query: `SELECT s.Server_name AS "name"`}},
			{{Query: `, s.Owner AS "owner"`}},
			{{Query: `, s.Wrapper AS "wrapper"`}},
			{{Query: `, NULL AS "type"`}},
			{{Query: `, NULL AS "version"`}},
			{{Query: `, NULL AS "access"`}},
			{{Query: `, CONCAT_WS(', '` +
				`, NULLIF(CONCAT('host ', s.Host), 'host ')` +
				`, NULLIF(CONCAT('port ', s.Port), 'port 0')` +
				`, NULLIF(CONCAT('dbname ', s.Db), 'dbname ')` +
				`, NULLIF(CONCAT('socket ', s.Socket), 'socket ')` +
				`) AS "options"`}},
			{{Query: `, NULL AS "comment"`}},
			{{Query: `FROM mysql.servers s`}},
			{{Query: `WHERE (@name = '' OR s.Server_name LIKE @name)`}},
			{{Query: `ORDER BY 1`}},
		},
		Fields: []dbmeta.Field{
			{Name: "name"},
			{Name: "owner", Desc: "often empty: CREATE SERVER does not require an owner"},
			{Name: "wrapper", Desc: "the wrapper named by CREATE SERVER, such as mysql"},
			{Name: "type", Desc: "always absent: MariaDB records no server type"},
			{Name: "version", Desc: "always absent: MariaDB records no server version"},
			{Name: "access", Desc: "always absent: MariaDB grants no privilege on a server"},
			{Name: "options", Desc: "host, port, dbname and socket, joined the way psql prints them"},
			{Name: "comment", Desc: "always absent: MariaDB has no comment on a server"},
		},
		Params: []dbmeta.Param{{Name: "name", Desc: "server name pattern, empty for every server", Default: ""}},
		Scan: func(rows *sql.Rows) (dbmeta.ForeignServer, error) {
			var v dbmeta.ForeignServer
			err := rows.Scan(&v.Name, &v.Owner, &v.Wrapper, &v.Type, &v.Version,
				&v.Access, &v.Options, &v.Comment)
			return v, err
		},
	})

	// \deu. A MariaDB server carries one credential, and every local user
	// reaches the remote server with it. That is the mapping PostgreSQL writes
	// for PUBLIC, so the name column is the remote user and the mapping covers
	// everyone.
	dbmeta.UserMappings.Register(dbmeta.MySQL, &dbmeta.Binding[dbmeta.UserMapping]{
		Stmt: dbmeta.Stmt{
			{{Query: `SELECT s.Server_name AS "server"`}},
			{{Query: `, NULLIF(s.Username, '') AS "name"`}},
			{{Query: `, NULL AS "options"`}},
			{{Query: `FROM mysql.servers s`}},
			{{Query: `WHERE (@name = '' OR s.Server_name LIKE @name)`}},
			{{Query: `ORDER BY 1`}},
		},
		Fields: []dbmeta.Field{
			{Name: "server"},
			{Name: "name", Desc: "the remote user; it applies to every local user, as PUBLIC does"},
			{Name: "options", Desc: "always absent: the password is the only other setting and it is not read"},
		},
		Params: []dbmeta.Param{{Name: "name", Desc: "server name pattern, empty for every server", Default: ""}},
		Scan: func(rows *sql.Rows) (dbmeta.UserMapping, error) {
			var v dbmeta.UserMapping
			err := rows.Scan(&v.Server, &v.Name, &v.Options)
			return v, err
		},
	})

	// \det. MariaDB reaches remote data through a storage engine rather than
	// through a named server, and information_schema does not publish the
	// CONNECTION setting that names the server, so the server column holds the
	// engine. The list of engines is fixed here because it is the set that
	// reads data this server does not hold.
	dbmeta.ForeignTables.Register(dbmeta.MySQL, &dbmeta.Binding[dbmeta.ForeignTable]{
		Stmt: dbmeta.Stmt{
			{{Query: `SELECT t.table_schema AS "schema"`}},
			{{Query: `, t.table_name AS "name"`}},
			{{Query: `, t.engine AS "server"`}},
			{{Query: `, NULLIF(t.create_options, '') AS "options"`}},
			{{Query: `, NULLIF(t.table_comment, '') AS "comment"`}},
			{{Query: `FROM information_schema.TABLES t`}},
			{{Query: `WHERE t.engine IN (` + foreignEngines + `)`}},
			{{Query: `AND (@with_system OR t.table_schema NOT IN (` + systemSchemas + `))`}},
			{{Query: `AND (@schema = '' OR t.table_schema LIKE @schema)`}},
			{{Query: `AND (@name = '' OR t.table_name LIKE @name)`}},
			{{Query: `ORDER BY 1, 2`}},
		},
		Fields: []dbmeta.Field{
			{Name: "schema"},
			{Name: "name"},
			{Name: "server", Desc: "the engine that reaches the remote data, not a server name: " +
				"information_schema does not publish the CONNECTION setting"},
			{Name: "options", Desc: "the create options, which do not include CONNECTION"},
			{Name: "comment"},
		},
		Params: schemaNameSystem("table"),
		Scan: func(rows *sql.Rows) (dbmeta.ForeignTable, error) {
			var v dbmeta.ForeignTable
			err := rows.Scan(&v.Schema, &v.Name, &v.Server, &v.Options, &v.Comment)
			return v, err
		},
	})
}

// foreignEngines are the storage engines that read data this server does not
// hold. A table on one of them is what psql calls a foreign table.
const foreignEngines = `'FEDERATED', 'FEDERATEDX', 'CONNECT', 'SPIDER', 'OQGRAPH', 'S3'`
