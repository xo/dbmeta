package sqlite3

import (
	"database/sql"

	"github.com/xo/dbmeta"
)

// Tables, columns, indexes, constraints and triggers.

// indexType is the word for the kind of the entry i of pragma_index_list.
const indexType = `CASE i.origin WHEN 'c' THEN 'btree' WHEN 'u' THEN 'unique constraint'` +
	` WHEN 'pk' THEN 'primary key' ELSE i.origin END`

// tableType is the word for the kind of the entry m of sqlite_schema.
const tableType = `CASE m.type WHEN 'view' THEN 'view'` +
	` WHEN 'table' THEN CASE WHEN m.sql LIKE 'CREATE VIRTUAL TABLE%'` +
	` THEN 'virtual' ELSE 'table' END ELSE m.type END`

// indexPredicate is the WHERE clause of a partial index, cut from the
// statement that made it. SQLite keeps the statement as written, and it has no
// column for the predicate. The statement is searched with its tabs and line
// breaks turned to spaces, which keeps every offset, and the text is taken from
// the original. The first WHERE ends the key list, because SQLite takes no
// subquery in an index and a clause in the keys is not possible.
const indexPredicate = `CASE WHEN i."partial" = 1 THEN TRIM(SUBSTR(s.sql, INSTR(UPPER(REPLACE(REPLACE(REPLACE(` +
	`s.sql, CHAR(10), ' '), CHAR(13), ' '), CHAR(9), ' ')), ' WHERE ') + 7)) END`

func registerRelations() {
	// \dn and \l are the same answer here. An attached database is what
	// SQLite calls a schema, and it is also the only thing it calls a
	// database, so both queries read pragma_database_list.
	dbmeta.Schemas.Register(dbmeta.SQLite3, &dbmeta.Binding[dbmeta.Schema]{
		Stmt: dbmeta.Stmt{
			always(`SELECT '' AS "catalog"`),
			always(`, d.name AS "name"`),
			always(`, '' AS "owner"`),
			always(`, NULL AS "comment"`),
			always(`FROM pragma_database_list d`),
			always(`WHERE (@name = '' OR d.name LIKE @name)`),
			always(`ORDER BY d.seq`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: "always empty: SQLite has no catalog above a schema"},
			{Name: "name", Desc: "main, temp, or the name an ATTACH gave a file"},
			{Name: "owner", Desc: "always empty: SQLite has no users"},
			{Name: "comment", Desc: "always absent: SQLite records no comment on anything"},
		},
		Params: []dbmeta.Param{
			{Name: "name", Desc: "schema name pattern, empty for every schema", Default: ""},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Schema, error) {
			var v dbmeta.Schema
			err := rows.Scan(&v.Catalog, &v.Name, &v.Owner, &v.Comment)
			return v, err
		},
	})

	// \l. The file is the closest thing to a location, and an in memory
	// database reports it empty.
	dbmeta.Databases.Register(dbmeta.SQLite3, &dbmeta.Binding[dbmeta.Database]{
		Stmt: dbmeta.Stmt{
			always(`SELECT d.name AS "name"`),
			always(`, '' AS "owner"`),
			always(`, (SELECT encoding FROM pragma_encoding) AS "encoding"`),
			always(`, 'BINARY' AS "collate"`),
			always(`, 'BINARY' AS "ctype"`),
			always(`, NULL AS "access"`),
			always(`, NULLIF(d.file, '') AS "tablespace"`),
			always(`, NULL AS "size"`),
			always(`, NULL AS "comment"`),
			always(`FROM pragma_database_list d`),
			always(`WHERE (@name = '' OR d.name LIKE @name)`),
			always(`ORDER BY d.seq`),
		},
		Fields: []dbmeta.Field{
			{Name: "name"},
			{Name: "owner", Desc: "always empty: SQLite has no users"},
			{Name: "encoding", Desc: "the text encoding of the main database"},
			{Name: "collate", Desc: "always BINARY: SQLite compares text by bytes unless a column says otherwise"},
			{Name: "ctype", Desc: "always BINARY, for the same reason"},
			{Name: "access", Desc: "always absent: SQLite has no grants"},
			{
				Name: "tablespace",
				Desc: "the file this database is stored in, absent for one held in memory",
			},
			{Name: "size", Desc: "always absent: read page_count and page_size to compute it"},
			{Name: "comment", Desc: "always absent"},
		},
		Params: []dbmeta.Param{
			{Name: "name", Desc: "database name pattern, empty for every one", Default: ""},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Database, error) {
			var v dbmeta.Database
			err := rows.Scan(&v.Name, &v.Owner, &v.Encoding, &v.Collate, &v.CType,
				&v.Access, &v.Tablespace, &v.Size, &v.Comment)
			return v, err
		},
	})

	// \d, \dt, \dv. A virtual table is reported as its own type rather than
	// as a table, because it reads data SQLite does not store and a caller
	// that treats it as a table will be surprised by what it cannot do.
	dbmeta.Tables.Register(dbmeta.SQLite3, &dbmeta.Binding[dbmeta.Table]{
		Stmt: dbmeta.Stmt{
			always(`SELECT '' AS "catalog"`),
			always(`, ` + mainSchema),
			always(`, m.name AS "name"`),
			always(`, ` + tableType + ` AS "type"`),
			always(`, NULL AS "comment"`),
			// sqlite_schema is the main database, and a temporary table is
			// in sqlite_temp_schema, which this query does not read.
			always(`, CASE m.type WHEN 'table' THEN 'permanent' END AS "persistence"`),
			always(`FROM sqlite_schema m`),
			always(`WHERE m.type IN ('table', 'view')`),
			notSystem("m.name"),
			always(`AND (@schema = '' OR 'main' LIKE @schema)`),
			always(`AND (@name = '' OR m.name LIKE @name)`),
			always(`AND (@types = '' OR ` + dbmeta.InList(`@types`, tableType) + `)`),
			always(`ORDER BY m.name`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: "always empty: SQLite has no catalog above a schema"},
			{Name: "schema", Desc: "always main: sqlite_schema describes the main database"},
			{Name: "name"},
			{
				Name: "type",
				Desc: "table, view, or virtual for a table backed by a module such as fts5",
			},
			{Name: "comment", Desc: "always absent: SQLite records no comment on anything"},
			{
				Name: "persistence",
				Desc: "permanent for a table, because this query reads the main database and a temporary table is in the temp one. Absent for a view",
			},
		},
		Params: append(schemaNameSystem("table"), dbmeta.TypesParam()),
		Scan: func(rows *sql.Rows) (dbmeta.Table, error) {
			var v dbmeta.Table
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Name, &v.Type, &v.Comment, &v.Persistence)
			return v, err
		},
	})

	// \d name. pragma_table_xinfo is joined to sqlite_schema so that one
	// statement answers for every table, and it is the xinfo form rather than
	// info so that a generated column and a virtual table's hidden column
	// appear.
	dbmeta.Columns.Register(dbmeta.SQLite3, &dbmeta.Binding[dbmeta.Column]{
		Stmt: dbmeta.Stmt{
			always(`SELECT '' AS "catalog"`),
			always(`, ` + mainSchema),
			always(`, m.name AS "table"`),
			always(`, c.name AS "name"`),
			always(`, c.cid + 1 AS "ordinal"`),
			// A declared type is optional in SQLite and an empty string means
			// the column has none, which is a fact rather than a missing one.
			always(`, c."type" AS "data_type"`),
			always(`, c."notnull" = 0 AS "nullable"`),
			always(`, c.dflt_value AS "default"`),
			// Free: pragma_table_xinfo already reports the position of the
			// column within the primary key, and zero means it is not in one.
			always(`, c.pk > 0 AS "primary_key"`),
			// INTEGER PRIMARY KEY is the rowid, and it is the only column
			// SQLite fills by itself.
			always(`, CASE WHEN c.pk = 1 AND UPPER(c."type") = 'INTEGER'` +
				` THEN 'rowid' ELSE NULL END AS "identity"`),
			// hidden is 2 for a virtual generated column and 3 for a stored
			// one. 1 is a hidden column of a virtual table, which is not
			// generated.
			always(`, CASE c.hidden WHEN 2 THEN 'virtual' WHEN 3 THEN 'stored'` +
				` ELSE NULL END AS "generated"`),
			always(`, NULL AS "comment"`),
			always(`, NULL AS "collation"`),
			always(`FROM sqlite_schema m JOIN pragma_table_xinfo(m.name) c`),
			always(`WHERE m.type IN ('table', 'view')`),
			notSystem("m.name"),
			always(`AND (@schema = '' OR 'main' LIKE @schema)`),
			always(`AND (@parent = '' OR m.name LIKE @parent)`),
			always(`AND (@name = '' OR c.name LIKE @name)`),
			always(`ORDER BY m.name, c.cid`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog"}, {Name: "schema"}, {Name: "table"}, {Name: "name"},
			{Name: "ordinal", Desc: "one based, where the pragma counts from zero"},
			{
				Name: "data_type",
				Desc: "the declared type, which SQLite does not enforce and which is empty when the column declared none",
			},
			{Name: "nullable"},
			{Name: "default", Desc: "the default as written, so a string default keeps its quotes"},
			{Name: "primary_key", Desc: "whether the column is part of the primary key"},
			{
				Name: "identity",
				Desc: "rowid for an INTEGER PRIMARY KEY, which SQLite fills by itself, and absent otherwise",
			},
			{Name: "generated", Desc: "virtual or stored for a generated column"},
			{Name: "comment", Desc: "always absent"},
			{Name: "collation", Desc: "always absent: pragma_table_xinfo does not report the COLLATE of a column, which SQLite keeps only in the DDL text"},
		},
		Params: schemaParentName("column"),
		Scan: func(rows *sql.Rows) (dbmeta.Column, error) {
			var v dbmeta.Column
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Table, &v.Name, &v.Ordinal,
				&v.DataType, &v.Nullable, &v.Default, &v.PrimaryKey, &v.Identity,
				&v.Generated, &v.Comment, &v.Collation)
			return v, err
		},
	})

	// \di. pragma_index_list carries what sqlite_schema cannot: whether the
	// index is unique, and whether SQLite created it for a constraint. An
	// index made for a UNIQUE or PRIMARY KEY clause has no row in
	// sqlite_schema.sql, so the pragma is the only place it is described.
	dbmeta.Indexes.Register(dbmeta.SQLite3, &dbmeta.Binding[dbmeta.Index]{
		Stmt: dbmeta.Stmt{
			always(`SELECT '' AS "catalog"`),
			always(`, ` + mainSchema),
			always(`, m.name AS "table"`),
			always(`, i.name AS "name"`),
			dbmeta.Choice{
				{Query: `, ` + indexType + ` AS "type"`},
				// libSQL has one method of its own, the DiskANN vector
				// index, which its type=diskann option names.
				{Key: LibSQL, Query: `, CASE WHEN i.name IN (` + vectorIndexes + `)` +
					` THEN 'diskann' ELSE ` + indexType + ` END AS "type"`},
			},
			always(`, i."unique" = 1 AS "unique"`),
			always(`, i.origin = 'pk' AS "primary"`),
			always(`, NULL AS "comment"`),
			always(`, 'permanent' AS "persistence"`),
			always(`, ` + indexPredicate + ` AS "predicate"`),
			// The primary key index of a WITHOUT ROWID table is the table. Its
			// entries carry no rowid, and the entries of every other index
			// end with one, so the pragma says which is which for one index
			// at a cost that follows the index.
			always(`, i.origin = 'pk' AND NOT EXISTS (SELECT 1 FROM pragma_index_xinfo(i.name) x` +
				` WHERE x.cid = -1) AS "clustered"`),
			always(`, s.sql AS "definition"`),
			dbmeta.Choice{
				{Query: `, 'btree' AS "using"`},
				{Key: LibSQL, Query: `, CASE WHEN i.name IN (` + vectorIndexes + `)` +
					` THEN 'diskann' ELSE 'btree' END AS "using"`},
			},
			always(`, CASE i.origin WHEN 'pk' THEN 'p' WHEN 'u' THEN 'u' END AS "constraint_type"`),
			always(`FROM sqlite_schema m JOIN pragma_index_list(m.name) i`),
			always(`LEFT JOIN sqlite_schema s ON s.type = 'index' AND s.name = i.name`),
			always(`WHERE m.type = 'table'`),
			notSystem("m.name"),
			always(`AND (@schema = '' OR 'main' LIKE @schema)`),
			always(`AND (@parent = '' OR m.name LIKE @parent)`),
			always(`AND (@name = '' OR i.name LIKE @name)`),
			always(`ORDER BY m.name, i.name`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog"}, {Name: "schema"}, {Name: "table"}, {Name: "name"},
			{
				Name: "type",
				Desc: "btree for an index someone created, and the constraint kind for one SQLite created itself. On libSQL, diskann for a vector index",
			},
			{Name: "unique"}, {Name: "primary"},
			{Name: "comment", Desc: "always absent"},
			{Name: "persistence", Desc: "always permanent: this query reads the main database"},
			{
				Name: "predicate",
				Desc: "the text after WHERE in the CREATE INDEX statement of a partial index, as written, and absent for an index that covers every row",
			},
			{
				Name: "clustered",
				Desc: "true for the primary key index of a WITHOUT ROWID table, which holds the whole row, and false for every other index",
			},
			{
				Name: "definition",
				Desc: "the CREATE INDEX statement as written. Absent for an index SQLite made for a PRIMARY KEY or UNIQUE clause, which has no statement",
			},
			{Name: "using", Desc: "always btree, and on libSQL diskann for a vector index"},
			{
				Name: "constraint_type",
				Desc: "p for the index of a primary key and u for the index of a UNIQUE clause, as pg_constraint spells them, and absent for an index someone created",
			},
		},
		Params: schemaParentName("index"),
		Scan: func(rows *sql.Rows) (dbmeta.Index, error) {
			var v dbmeta.Index
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Table, &v.Name, &v.Type,
				&v.Unique, &v.Primary, &v.Comment, &v.Persistence, &v.Predicate,
				&v.Clustered, &v.Definition, &v.Using, &v.ConstraintType)
			return v, err
		},
	})

	// \d name. index_xinfo rather than index_info, so that the rowid columns
	// SQLite appends to every index appear. They are filtered out here by
	// key = 1, which is what index_info returns, but the pragma
	// also carries the collation and the direction and index_info does not.
	dbmeta.IndexColumns.Register(dbmeta.SQLite3, &dbmeta.Binding[dbmeta.IndexColumn]{
		Stmt: dbmeta.Stmt{
			always(`SELECT ` + mainSchema),
			always(`, m.tbl_name AS "table"`),
			always(`, m.name AS "index"`),
			// cid is -1 for the rowid and -2 for an expression, and the name
			// is NULL for both.
			always(`, x.name AS "name"`),
			always(`, x.seqno + 1 AS "ordinal"`),
			always(`, CASE WHEN x.cid = -2 THEN 'expression' ELSE NULL END AS "expression"`),
			always(`, x.desc = 1 AS "descending"`),
			always(`FROM sqlite_schema m JOIN pragma_index_xinfo(m.name) x`),
			always(`WHERE m.type = 'index'`),
			// key = 0 is a column the index carries to reach the row, not a
			// column the index is on
			always(`AND x.key = 1`),
			notSystem("m.tbl_name"),
			always(`AND (@schema = '' OR 'main' LIKE @schema)`),
			always(`AND (@parent = '' OR m.tbl_name LIKE @parent)`),
			always(`AND (@name = '' OR m.name LIKE @name)`),
			always(`ORDER BY m.name, x.seqno`),
		},
		Fields: []dbmeta.Field{
			{Name: "schema"}, {Name: "table"}, {Name: "index"},
			{Name: "name", Desc: "absent for a column the index computes rather than stores"},
			{Name: "ordinal", Desc: "one based, where the pragma counts from zero"},
			{
				Name: "expression",
				Desc: "the word expression when the index is on one, because SQLite does not publish its text",
			},
			{Name: "descending"},
		},
		Params: schemaParentName("index"),
		Scan: func(rows *sql.Rows) (dbmeta.IndexColumn, error) {
			var v dbmeta.IndexColumn
			err := rows.Scan(&v.Schema, &v.Table, &v.Index, &v.Name, &v.Ordinal,
				&v.Expression, &v.Descending)
			return v, err
		},
	})

	registerConstraints()

	// \d name. A trigger keeps its whole CREATE statement and nothing else,
	// so the definition is the DDL and there is no separate event or timing.
	dbmeta.Triggers.Register(dbmeta.SQLite3, &dbmeta.Binding[dbmeta.Trigger]{
		Stmt: dbmeta.Stmt{
			always(`SELECT ` + mainSchema),
			always(`, m.tbl_name AS "table"`),
			always(`, m.name AS "name"`),
			always(`, 'enabled' AS "enabled"`),
			always(`, m.sql AS "definition"`),
			always(`, NULL AS "comment"`),
			always(`FROM sqlite_schema m`),
			always(`WHERE m.type = 'trigger'`),
			notSystem("m.tbl_name"),
			always(`AND (@schema = '' OR 'main' LIKE @schema)`),
			always(`AND (@parent = '' OR m.tbl_name LIKE @parent)`),
			always(`AND (@name = '' OR m.name LIKE @name)`),
			always(`ORDER BY m.tbl_name, m.name`),
		},
		Fields: []dbmeta.Field{
			{Name: "schema"}, {Name: "table"}, {Name: "name"},
			{
				Name: "enabled",
				Desc: "always enabled: SQLite cannot disable one trigger, only every trigger at once",
			},
			{Name: "definition", Desc: "the CREATE TRIGGER statement as written"},
			{Name: "comment", Desc: "always absent"},
		},
		Params: schemaParentName("trigger"),
		Scan: func(rows *sql.Rows) (dbmeta.Trigger, error) {
			var v dbmeta.Trigger
			err := rows.Scan(&v.Schema, &v.Table, &v.Name, &v.Enabled, &v.Definition, &v.Comment)
			return v, err
		},
	})
}

// registerConstraints backs the constraint part of \d name.
//
// It is three queries joined by UNION ALL, because SQLite keeps each kind in a
// different pragma and none of them names a constraint the way the others do.
//
// A check constraint is missing. SQLite records it only inside the CREATE
// TABLE text in sqlite_schema.sql, and dbmeta does not parse DDL. Returning
// the three kinds it can read exactly is worth more than refusing all four,
// and this is the one place a dbmeta query answers incompletely on purpose.
// See docs/COVERAGE.md.
func registerConstraints() {
	dbmeta.Constraints.Register(dbmeta.SQLite3, &dbmeta.Binding[dbmeta.Constraint]{
		Stmt: dbmeta.Stmt{
			// the primary key, named the way SQLite names its index
			always(`SELECT ` + mainSchema),
			always(`, m.name AS "table"`),
			always(`, 'pk_' || m.name AS "name"`),
			always(`, 'primary key' AS "type"`),
			always(`, 'PRIMARY KEY (' || (SELECT GROUP_CONCAT(x.name, ', ') FROM` +
				` (SELECT k.name FROM pragma_table_xinfo(m.name) k WHERE k.pk > 0 ORDER BY k.pk) x)` +
				` || ')' AS "definition"`),
			always(`, FALSE AS "deferrable"`),
			always(`, FALSE AS "deferred"`),
			always(`, NULL AS "comment"`),
			always(`FROM sqlite_schema m JOIN pragma_table_xinfo(m.name) c`),
			always(`WHERE m.type = 'table' AND c.pk > 0`),
			notSystem("m.name"),
			always(`AND (@schema = '' OR 'main' LIKE @schema)`),
			always(`AND (@parent = '' OR m.name LIKE @parent)`),
			always(`AND (@name = '' OR ('pk_' || m.name) LIKE @name)`),
			always(`GROUP BY m.name`),

			// every unique index, whether SQLite made it for a UNIQUE clause
			// or someone created it
			always(`UNION ALL`),
			always(`SELECT 'main', m.name, i.name, 'unique'`),
			always(`, 'UNIQUE (' || (SELECT GROUP_CONCAT(x.name, ', ') FROM` +
				` (SELECT k.name FROM pragma_index_xinfo(i.name) k WHERE k.key = 1 ORDER BY k.seqno) x) || ')'`),
			always(`, FALSE, FALSE, NULL`),
			always(`FROM sqlite_schema m JOIN pragma_index_list(m.name) i`),
			always(`WHERE m.type = 'table' AND i."unique" = 1 AND i.origin <> 'pk'`),
			notSystem("m.name"),
			always(`AND (@schema = '' OR 'main' LIKE @schema)`),
			always(`AND (@parent = '' OR m.name LIKE @parent)`),
			always(`AND (@name = '' OR i.name LIKE @name)`),

			// every foreign key. SQLite does not name one, so the name is
			// built from the table and the position, which is what it is
			// reported by everywhere else. pragma_foreign_key_list has a row
			// for each column of a key, so the first column's row stands for
			// the key, and the definition gathers the columns in order. A
			// key that names no columns references the primary key, and
			// its definition names none, as it was written.
			always(`UNION ALL`),
			always(`SELECT 'main', m.name, 'fk_' || m.name || '_' || f.id, 'foreign key'`),
			always(`, 'FOREIGN KEY (' || (SELECT GROUP_CONCAT(x."from", ', ') FROM` +
				` (SELECT k."from" FROM pragma_foreign_key_list(m.name) k WHERE k.id = f.id ORDER BY k.seq) x)` +
				` || ') REFERENCES ' || f."table"` +
				` || COALESCE('(' || (SELECT GROUP_CONCAT(x."to", ', ') FROM` +
				` (SELECT k."to" FROM pragma_foreign_key_list(m.name) k WHERE k.id = f.id ORDER BY k.seq) x) || ')', '')` +
				` || CASE WHEN f.on_update <> 'NO ACTION' THEN ' ON UPDATE ' || f.on_update ELSE '' END` +
				` || CASE WHEN f.on_delete <> 'NO ACTION' THEN ' ON DELETE ' || f.on_delete ELSE '' END`),
			always(`, FALSE, FALSE, NULL`),
			always(`FROM sqlite_schema m JOIN pragma_foreign_key_list(m.name) f`),
			always(`WHERE m.type = 'table' AND f.seq = 0`),
			notSystem("m.name"),
			always(`AND (@schema = '' OR 'main' LIKE @schema)`),
			always(`AND (@parent = '' OR m.name LIKE @parent)`),
			always(`AND (@name = '' OR ('fk_' || m.name || '_' || f.id) LIKE @name)`),
			always(`ORDER BY 2, 4, 3`),
		},
		Fields: []dbmeta.Field{
			{Name: "schema"}, {Name: "table"},
			{
				Name: "name",
				Desc: "SQLite names only an index, so a primary key and a foreign key are named after the table",
			},
			{
				Name: "type",
				Desc: "primary key, unique or foreign key. A check constraint is never listed: SQLite keeps it only as DDL text",
			},
			{Name: "definition", Desc: "the constraint as SQL, such as PRIMARY KEY (a, b) or FOREIGN KEY (a) REFERENCES t(b), as psql prints it"},
			{Name: "deferrable", Desc: "always false: SQLite defers by connection, not by constraint"},
			{Name: "deferred", Desc: "always false: pragma_foreign_key_list does not report DEFERRABLE INITIALLY DEFERRED, which SQLite keeps only in the DDL text"},
			{Name: "comment", Desc: "always absent"},
		},
		Params: schemaParentName("constraint"),
		Scan: func(rows *sql.Rows) (dbmeta.Constraint, error) {
			var v dbmeta.Constraint
			err := rows.Scan(&v.Schema, &v.Table, &v.Name, &v.Type, &v.Definition,
				&v.Deferrable, &v.Deferred, &v.Comment)
			return v, err
		},
	})
}
