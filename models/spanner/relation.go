package spanner

import (
	"database/sql"

	"github.com/xo/dbmeta"
)

// Schemas, tables, columns, indexes, views, sequences and the current schema.

// tableType is the word for the kind of the relation t. Spanner says BASE TABLE,
// VIEW and SYNONYM.
const tableType = `CASE t.table_type WHEN 'BASE TABLE' THEN 'table' ELSE LOWER(t.table_type) END`

// tableOptions is Table.Options. Each part is name=value, and a part that the
// relation has no source for is NULL, which ARRAY_TO_STRING leaves out. The
// first four come from TABLES: the parent table of an interleaved table, how it
// is interleaved, what happens to it when a parent row goes, and the policy that
// deletes old rows. The next is the options of the table from TABLE_OPTIONS, such
// as locality_group, which tableJoins aggregates as o.options. The last names the
// table that a synonym stands for, which tableJoins reads as y.target.
const tableOptions = `NULLIF(ARRAY_TO_STRING(` +
	`['parent_table=' || t.parent_table_name` +
	`, 'interleave_type=' || t.interleave_type` +
	`, 'on_delete=' || t.on_delete_action` +
	`, 'row_deletion_policy=' || t.row_deletion_policy_expression` +
	`, o.options, 'synonym_for=' || y.target], ', '), '')`

// tableJoins reads the options and the synonyms of every table once, and joins
// them to the table t. A subquery that names t runs for each table, and a
// catalog of thousands of tables then costs the square of that.
const tableJoins = `LEFT JOIN (SELECT table_catalog, table_schema, table_name` +
	`, STRING_AGG(option_name || '=' || option_value, ', ' ORDER BY option_name) AS options` +
	` FROM information_schema.table_options GROUP BY table_catalog, table_schema, table_name) o` +
	` ON o.table_catalog = t.table_catalog AND o.table_schema = t.table_schema AND o.table_name = t.table_name` +
	` LEFT JOIN (SELECT synonym_catalog, synonym_schema, synonym_name, MIN(table_name) AS target` +
	` FROM information_schema.table_synonyms GROUP BY synonym_catalog, synonym_schema, synonym_name) y` +
	` ON y.synonym_catalog = t.table_catalog AND y.synonym_schema = t.table_schema AND y.synonym_name = t.table_name`

// indexOptions is Index.Options. The parts are the parent table of an
// interleaved index, the word managed for an index that Spanner made to back a
// foreign key, and the options of the index, such as distance_type, which
// indexJoins aggregates as o.options.
const indexOptions = `NULLIF(ARRAY_TO_STRING(` +
	`[IF(i.parent_table_name = '', NULL, 'interleave_in=' || i.parent_table_name)` +
	`, IF(i.spanner_is_managed, 'managed=true', NULL), o.options], ', '), '')`

// indexJoins reads the options of every index once, as tableJoins does.
const indexJoins = `LEFT JOIN (SELECT table_catalog, table_schema, table_name, index_name` +
	`, STRING_AGG(option_name || '=' || option_value, ', ' ORDER BY option_name) AS options` +
	` FROM information_schema.index_options` +
	` GROUP BY table_catalog, table_schema, table_name, index_name) o` +
	` ON o.table_catalog = i.table_catalog AND o.table_schema = i.table_schema` +
	` AND o.table_name = i.table_name AND o.index_name = i.index_name`

func registerRelations() {
	// \dn. SCHEMATA lists the default schema, which is named empty, the named
	// schemas, and the two Spanner keeps.
	dbmeta.Schemas.Register(dbmeta.Spanner, &dbmeta.Binding[dbmeta.Schema]{
		Stmt: dbmeta.Stmt{
			always("SELECT s.catalog_name AS `catalog`"),
			always(", s.schema_name AS `name`"),
			always(", s.schema_owner AS `owner`"),
			always(", CAST(NULL AS STRING) AS `comment`"),
			always(", CAST(NULL AS STRING) AS `access`"),
			always("FROM information_schema.schemata s"),
			always("WHERE " + notSystem("s.schema_name")),
			always("AND " + like("s.schema_name", "@name")),
			always("ORDER BY 2"),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: "always empty: Spanner has no catalog above a schema"},
			{Name: "name", Desc: "empty for the default schema, and the name for any other"},
			{Name: "owner", Desc: "spanner_admin for the default schema and a schema made with CREATE SCHEMA, and spanner_system for the two Spanner keeps. Empty if the server reports none"},
			{Name: "comment", Desc: "always absent: Spanner records no comment on anything"},
			{Name: "access", Desc: "always absent: a grant on a schema is not in INFORMATION_SCHEMA"},
		},
		Params: []dbmeta.Param{
			{Name: "name", Desc: "schema name pattern, empty for every schema", Default: ""},
			{Name: "with_system", Desc: "include INFORMATION_SCHEMA and SPANNER_SYS", Default: false},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Schema, error) {
			var v dbmeta.Schema
			err := rows.Scan(&v.Catalog, &v.Name, dbmeta.NullAsEmpty(&v.Owner), &v.Comment, &v.Access)
			return v, err
		},
	})

	// \d, \dt, \dv. A synonym is a table under another name, and it is its own
	// type, because a caller that treats it as a table is right about reading it
	// and wrong about changing it.
	dbmeta.Tables.Register(dbmeta.Spanner, &dbmeta.Binding[dbmeta.Table]{
		Stmt: dbmeta.Stmt{
			always("SELECT t.table_catalog AS `catalog`"),
			always(", t.table_schema AS `schema`"),
			always(", t.table_name AS `name`"),
			always(", " + tableType + " AS `type`"),
			always(", CAST(NULL AS STRING) AS `comment`"),
			always(", CAST(NULL AS STRING) AS `owner`"),
			always(", CASE t.table_type WHEN 'BASE TABLE' THEN 'permanent' END AS `persistence`"),
			always(", CAST(NULL AS STRING) AS `access_method`"),
			always(", CAST(NULL AS INT64) AS `size`"),
			always(", CAST(NULL AS INT64) AS `rows`"),
			always(", " + tableOptions + " AS `options`"),
			always(", CAST(NULL AS BOOL) AS `row_security`"),
			always(", CAST(NULL AS BOOL) AS `row_security_forced`"),
			always("FROM information_schema.tables t"),
			always(tableJoins),
			always("WHERE " + notSystem("t.table_schema")),
			always("AND " + like("t.table_schema", "@schema")),
			always("AND " + like("t.table_name", "@name")),
			always("AND (@types = '' OR " + dbmeta.InList("@types", tableType) + ")"),
			always("ORDER BY 2, 3"),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: "always empty: Spanner has no catalog above a schema"},
			{Name: "schema", Desc: "empty for the default schema"},
			{Name: "name"},
			{Name: "type", Desc: "table, view, or synonym for a name made with ALTER TABLE ADD SYNONYM. The views of INFORMATION_SCHEMA and SPANNER_SYS are views"},
			{Name: "comment", Desc: "always absent: Spanner records no comment on anything"},
			{Name: "owner", Desc: "always absent: a Spanner table has no owner, and access is by role"},
			{Name: "persistence", Desc: "permanent for a table, and absent for a view and a synonym. Spanner has no temporary table"},
			{Name: "access_method", Desc: "always absent: Spanner has no access method"},
			{Name: "size", Desc: "always absent: SPANNER_SYS.TABLE_SIZES_STATS_1HOUR holds an hourly size and is empty for a young table, and D216 leaves it out until it is measured"},
			{Name: "rows", Desc: "always absent: Spanner keeps no estimate of the rows of a table"},
			{
				Name: "options",
				Desc: "name=value joined by a comma and a space. parent_table, interleave_type and on_delete for a table interleaved in a parent, row_deletion_policy for a table with a row deletion policy, the options of the table such as locality_group, and synonym_for for a synonym. Absent when none applies",
			},
			{Name: "row_security", Desc: "always absent: Spanner has no row level security. Fine grained access control is by role and is read by Privileges"},
			{Name: "row_security_forced", Desc: "always absent, for the same reason"},
		},
		Params: append(schemaNameSystem("table"), dbmeta.TypesParam()),
		Scan: func(rows *sql.Rows) (dbmeta.Table, error) {
			var v dbmeta.Table
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Name, &v.Type, &v.Comment, &v.Owner,
				&v.Persistence, &v.AccessMethod, &v.Size, &v.Rows, &v.Options,
				&v.RowSecurity, &v.RowSecurityForced)
			return v, err
		},
	})

	// \d name. A generated column is stored or virtual, and a TOKENLIST column
	// of a search index is the virtual one. An identity column reports by default,
	// which is the one form Spanner has.
	dbmeta.Columns.Register(dbmeta.Spanner, &dbmeta.Binding[dbmeta.Column]{
		Stmt: dbmeta.Stmt{
			always("SELECT c.table_catalog AS `catalog`"),
			always(", c.table_schema AS `schema`"),
			always(", c.table_name AS `table`"),
			always(", c.column_name AS `name`"),
			always(", c.ordinal_position AS `ordinal`"),
			always(", c.spanner_type AS `data_type`"),
			always(", c.is_nullable = 'YES' AS `nullable`"),
			always(", c.column_default AS `default`"),
			always(", k.column_name IS NOT NULL AS `primary_key`"),
			always(", IF(c.is_identity = 'YES', LOWER(c.identity_generation), NULL) AS `identity`"),
			always(", IF(c.is_generated = 'ALWAYS', IF(c.is_stored = 'YES', 'stored', 'virtual'), NULL) AS `generated`"),
			always(", CAST(NULL AS STRING) AS `comment`"),
			always(", CAST(NULL AS STRING) AS `collation`"),
			always(", CAST(NULL AS STRING) AS `storage`"),
			always(", CAST(NULL AS STRING) AS `compression`"),
			always(", CAST(NULL AS INT64) AS `stats_target`"),
			always("FROM information_schema.columns c"),
			always("LEFT JOIN information_schema.index_columns k"),
			always("ON k.index_name = 'PRIMARY_KEY' AND k.table_catalog = c.table_catalog"),
			always("AND k.table_schema = c.table_schema AND k.table_name = c.table_name"),
			always("AND k.column_name = c.column_name"),
			always("WHERE " + notSystem("c.table_schema")),
			always("AND " + like("c.table_schema", "@schema")),
			always("AND " + like("c.table_name", "@parent")),
			always("AND " + like("c.column_name", "@name")),
			always("ORDER BY 2, 3, 5"),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog"}, {Name: "schema"}, {Name: "table"}, {Name: "name"},
			{Name: "ordinal", Desc: "one based, and it counts a hidden column such as the TOKENLIST of a search index"},
			{Name: "data_type", Desc: "SPANNER_TYPE, such as STRING(100) or ARRAY<FLOAT32>(vector_length=>3). DATA_TYPE is empty in the GoogleSQL dialect"},
			{Name: "nullable"},
			{Name: "default", Desc: "the default expression as Spanner writes it, which is absent for a column with none. A generated column and an identity column have none"},
			{Name: "primary_key", Desc: "whether the column is part of the primary key"},
			{Name: "identity", Desc: "by default for a column made with GENERATED BY DEFAULT AS IDENTITY, and absent otherwise"},
			{Name: "generated", Desc: "stored for a column made with AS (...) STORED, and virtual for the TOKENLIST column of a search index, which is not stored"},
			{Name: "comment", Desc: "always absent: Spanner records no comment on anything"},
			{Name: "collation", Desc: "always absent: a Spanner column has no collation"},
			{Name: "storage", Desc: "always absent: Spanner has no storage choice for a column"},
			{Name: "compression", Desc: "always absent: Spanner has no compression choice for a column"},
			{Name: "stats_target", Desc: "always absent: Spanner has no statistics target"},
		},
		Params: schemaParentName("table", "column"),
		Scan: func(rows *sql.Rows) (dbmeta.Column, error) {
			var v dbmeta.Column
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Table, &v.Name, &v.Ordinal,
				&v.DataType, &v.Nullable, &v.Default, &v.PrimaryKey, &v.Identity,
				&v.Generated, &v.Comment, &v.Collation, &v.Storage, &v.Compression,
				&v.StatsTarget)
			return v, err
		},
	})

	// \di. The primary key is an index named PRIMARY_KEY, as Spanner lists it,
	// and a foreign key that needs an index gets one that Spanner names IDX_ and
	// marks managed.
	dbmeta.Indexes.Register(dbmeta.Spanner, &dbmeta.Binding[dbmeta.Index]{
		Stmt: dbmeta.Stmt{
			always("SELECT i.table_catalog AS `catalog`"),
			always(", i.table_schema AS `schema`"),
			always(", i.table_name AS `table`"),
			always(", i.index_name AS `name`"),
			always(", i.index_type AS `type`"),
			always(", i.is_unique AS `unique`"),
			always(", i.index_type = 'PRIMARY_KEY' AS `primary`"),
			always(", CAST(NULL AS STRING) AS `comment`"),
			always(", CAST(NULL AS STRING) AS `owner`"),
			always(", 'permanent' AS `persistence`"),
			always(", CAST(NULL AS INT64) AS `size`"),
			always(", i.filter AS `predicate`"),
			always(", IF(i.index_state IS NULL, NULL, i.index_state = 'READ_WRITE') AS `valid`"),
			always(", CAST(NULL AS BOOL) AS `clustered`"),
			always(", CAST(NULL AS BOOL) AS `replica_identity`"),
			always(", CAST(NULL AS BOOL) AS `deferrable`"),
			always(", CAST(NULL AS BOOL) AS `initially_deferred`"),
			always(", " + indexOptions + " AS `options`"),
			always(", CAST(NULL AS STRING) AS `definition`"),
			always(", CAST(NULL AS STRING) AS `using`"),
			always(", IF(i.index_type = 'PRIMARY_KEY', 'p', NULL) AS `constraint_type`"),
			always(", CAST(NULL AS STRING) AS `constraint_definition`"),
			always(", CAST(NULL AS BOOL) AS `constraint_period`"),
			always(", CAST(NULL AS BOOL) AS `table_visible`"),
			always("FROM information_schema.indexes i"),
			always(indexJoins),
			always("WHERE " + notSystem("i.table_schema")),
			always("AND " + like("i.table_schema", "@schema")),
			always("AND " + like("i.table_name", "@parent")),
			always("AND " + like("i.index_name", "@name")),
			always("ORDER BY 2, 3, 4"),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog"}, {Name: "schema"}, {Name: "table"}, {Name: "name"},
			{Name: "type", Desc: "INDEX_TYPE as Spanner says it: INDEX, PRIMARY_KEY, SEARCH or VECTOR"},
			{Name: "unique", Desc: "IS_UNIQUE. Spanner reports true for a search index and for the primary key"},
			{Name: "primary", Desc: "true for the index named PRIMARY_KEY, which is the table itself"},
			{Name: "comment", Desc: "always absent: Spanner records no comment on anything"},
			{Name: "owner", Desc: "always absent: a Spanner index has no owner"},
			{Name: "persistence", Desc: "always permanent: Spanner has no temporary index"},
			{Name: "size", Desc: "always absent: SPANNER_SYS.TABLE_SIZES_STATS_1HOUR holds an hourly size, which D216 leaves out until it is measured"},
			{Name: "predicate", Desc: "FILTER, which is the expression a null filtered index or a vector index keeps rows by, such as published IS NOT NULL. Absent for an index of every row"},
			{Name: "valid", Desc: "whether INDEX_STATE is READ_WRITE, and false while the index is backfilling. Absent for the primary key, which has no state"},
			{Name: "clustered", Desc: "always absent: Spanner stores a table in the order of its primary key and has no CLUSTER"},
			{Name: "replica_identity", Desc: "always absent: Spanner has no logical replication"},
			{Name: "deferrable", Desc: "always absent: no Spanner constraint owns an index that it can defer"},
			{Name: "initially_deferred", Desc: "always absent, for the same reason"},
			{Name: "options", Desc: "name=value joined by a comma and a space. interleave_in names the parent table of an interleaved index, managed=true marks an index that Spanner made to back a foreign key, and the rest are the options of the index, such as distance_type. Absent when none applies"},
			{Name: "definition", Desc: "always absent: INFORMATION_SCHEMA keeps the parts of an index and not its statement"},
			{Name: "using", Desc: "always absent, for the same reason"},
			{Name: "constraint_type", Desc: "p for the index of the primary key, and absent for every other, because Spanner has no unique constraint and its foreign key indexes are marked managed in options"},
			{Name: "constraint_definition", Desc: "always absent: Spanner has no exclusion constraint"},
			{Name: "constraint_period", Desc: "always absent: Spanner has no WITHOUT OVERLAPS"},
			{Name: "table_visible", Desc: "always absent: Spanner has no search path"},
		},
		Params: schemaParentName("table", "index"),
		Scan: func(rows *sql.Rows) (dbmeta.Index, error) {
			var v dbmeta.Index
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Table, &v.Name, &v.Type, &v.Unique,
				&v.Primary, &v.Comment, &v.Owner, &v.Persistence, &v.Size, &v.Predicate,
				&v.Valid, &v.Clustered, &v.ReplicaIdentity, &v.Deferrable,
				&v.InitiallyDeferred, &v.Options, &v.Definition, &v.Using,
				&v.ConstraintType, &v.ConstraintDefinition, &v.ConstraintPeriod,
				&v.TableVisible)
			return v, err
		},
	})

	// The default schema is the one an unqualified name resolves in. Spanner has
	// no search path, so there is nothing for a session to change.
	dbmeta.CurrentSchema.Register(dbmeta.Spanner, &dbmeta.Binding[dbmeta.Schema]{
		Stmt: dbmeta.Stmt{
			always("SELECT s.catalog_name AS `catalog`"),
			always(", s.schema_name AS `name`"),
			always(", s.schema_owner AS `owner`"),
			always(", CAST(NULL AS STRING) AS `comment`"),
			always(", CAST(NULL AS STRING) AS `access`"),
			always("FROM information_schema.schemata s"),
			always("WHERE s.schema_name = ''"),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog"},
			{Name: "name", Desc: "always empty: an unqualified name resolves in the default schema, which Spanner names empty"},
			{Name: "owner"},
			{Name: "comment", Desc: "always absent"},
			{Name: "access", Desc: "always absent"},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Schema, error) {
			var v dbmeta.Schema
			err := rows.Scan(&v.Catalog, &v.Name, dbmeta.NullAsEmpty(&v.Owner), &v.Comment, &v.Access)
			return v, err
		},
	})

	// \dv and \sv. VIEW_DEFINITION is the text of the query. SECURITY_TYPE is
	// INVOKER or DEFINER and has no field here.
	dbmeta.Views.Register(dbmeta.Spanner, &dbmeta.Binding[dbmeta.View]{
		Stmt: dbmeta.Stmt{
			always("SELECT v.table_catalog AS `catalog`"),
			always(", v.table_schema AS `schema`"),
			always(", v.table_name AS `name`"),
			always(", v.view_definition AS `definition`"),
			always(", CAST(NULL AS STRING) AS `check_option`"),
			always(", CAST(NULL AS BOOL) AS `updatable`"),
			always(", CAST(NULL AS BOOL) AS `insertable`"),
			always(", CAST(NULL AS STRING) AS `comment`"),
			always("FROM information_schema.views v"),
			always("WHERE " + notSystem("v.table_schema")),
			always("AND " + like("v.table_schema", "@schema")),
			always("AND " + like("v.table_name", "@name")),
			always("ORDER BY 2, 3"),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog"}, {Name: "schema"}, {Name: "name"},
			{Name: "definition", Desc: "the query as Spanner stores it, with the names qualified, because Spanner resolves a view in strict mode"},
			{Name: "check_option", Desc: "always absent: Spanner has no WITH CHECK OPTION"},
			{Name: "updatable", Desc: "always absent: a Spanner view cannot be written to, but INFORMATION_SCHEMA does not say so"},
			{Name: "insertable", Desc: "always absent, for the same reason"},
			{Name: "comment", Desc: "always absent"},
		},
		Params: schemaNameSystem("view"),
		Scan: func(rows *sql.Rows) (dbmeta.View, error) {
			var v dbmeta.View
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Name, &v.Definition, &v.CheckOption,
				&v.Updatable, &v.Insertable, &v.Comment)
			return v, err
		},
	})

	// \ds. A Spanner sequence is bit reversed, so it has a kind, a counter and a
	// range to skip and no bounds, no step and no cycle. Only the start has a
	// field here, and it is absent unless the sequence sets start_with_counter.
	dbmeta.Sequences.Register(dbmeta.Spanner, &dbmeta.Binding[dbmeta.Sequence]{
		Stmt: dbmeta.Stmt{
			always("SELECT s.schema AS `schema`"),
			always(", s.name AS `name`"),
			always(", s.data_type AS `data_type`"),
			always(", (SELECT o.option_value FROM information_schema.sequence_options o" +
				" WHERE o.catalog = s.catalog AND o.schema = s.schema AND o.name = s.name" +
				" AND o.option_name = 'start_with_counter') AS `start`"),
			always(", CAST(NULL AS STRING) AS `minimum`"),
			always(", CAST(NULL AS STRING) AS `maximum`"),
			always(", CAST(NULL AS STRING) AS `increment`"),
			always(", CAST(NULL AS BOOL) AS `cycles`"),
			always(", '' AS `owned_by`"),
			always(", CAST(NULL AS STRING) AS `comment`"),
			always(", CAST(NULL AS INT64) AS `cache_size`"),
			always("FROM information_schema.sequences s"),
			always("WHERE " + notSystem("s.schema")),
			always("AND " + like("s.schema", "@schema")),
			always("AND " + like("s.name", "@name")),
			always("ORDER BY 1, 2"),
		},
		Fields: []dbmeta.Field{
			{Name: "schema"}, {Name: "name"},
			{Name: "data_type", Desc: "always INT64"},
			{Name: "start", Desc: "the start_with_counter option, and absent for a sequence that does not set it. The sequence_kind and the skip range are options with no field here"},
			{Name: "minimum", Desc: "always absent: a bit reversed sequence has no bound that INFORMATION_SCHEMA reports"},
			{Name: "maximum", Desc: "always absent, for the same reason"},
			{Name: "increment", Desc: "always absent: a bit reversed sequence has no step"},
			{Name: "cycles", Desc: "always absent: a bit reversed sequence never repeats a value, and has no CYCLE"},
			{Name: "owned_by", Desc: "always empty: a Spanner sequence belongs to no column"},
			{Name: "comment", Desc: "always absent"},
			{Name: "cache_size", Desc: "always absent: Spanner has no cache size"},
		},
		Params: schemaNameSystem("sequence"),
		Scan: func(rows *sql.Rows) (dbmeta.Sequence, error) {
			var v dbmeta.Sequence
			err := rows.Scan(&v.Schema, &v.Name, &v.DataType, &v.Start, &v.Minimum,
				&v.Maximum, &v.Increment, &v.Cycles, &v.OwnedBy, &v.Comment, &v.CacheSize)
			return v, err
		},
	})
}
