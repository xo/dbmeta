package spanner

import (
	"database/sql"

	"github.com/xo/dbmeta"
)

// The detail of a table: index columns, constraints, the columns of a key and
// the NOT NULL constraints.

// notNull is true for a check constraint that Spanner made for a NOT NULL
// column, whose name begins with CK_IS_NOT_NULL_. col is the constraint name.
func notNull(col string) string { return `STARTS_WITH(` + col + `, ` + notNullPrefix + `)` }

func registerDetail() {
	// The columns of an index. A key column has an ordinal and a column that an
	// index STORES has none. The statement numbers the stored ones after the
	// keys, in the order of their names, which is the only order Spanner gives
	// them. Spanner has no window function, so a column counts the key columns
	// and the stored columns that sort at or before it by joining the columns of
	// its own index, and a key column joins nothing.
	dbmeta.IndexColumns.Register(dbmeta.Spanner, &dbmeta.Binding[dbmeta.IndexColumn]{
		Stmt: dbmeta.Stmt{
			always("SELECT c.table_schema AS `schema`"),
			always(", c.table_name AS `table`"),
			always(", c.index_name AS `index`"),
			always(", c.column_name AS `name`"),
			always(", COALESCE(c.ordinal_position, COUNT(s.column_name)) AS `ordinal`"),
			always(", c.expression AS `expression`"),
			always(", IF(c.column_ordering IS NULL, NULL, c.column_ordering = 'DESC') AS `descending`"),
			always(", c.ordinal_position IS NULL AS `include`"),
			always("FROM information_schema.index_columns c"),
			always("LEFT JOIN information_schema.index_columns s"),
			always("ON s.table_catalog = c.table_catalog AND s.table_schema = c.table_schema"),
			always("AND s.table_name = c.table_name AND s.index_name = c.index_name"),
			always("AND c.ordinal_position IS NULL"),
			always("AND (s.ordinal_position IS NOT NULL OR s.column_name <= c.column_name)"),
			always("WHERE " + notSystem("c.table_schema")),
			always("AND " + like("c.table_schema", "@schema")),
			always("AND " + like("c.table_name", "@parent")),
			always("AND " + like("c.index_name", "@index")),
			always("AND " + like("c.column_name", "@name")),
			always("GROUP BY c.table_schema, c.table_name, c.index_name, c.column_name"),
			always(", c.ordinal_position, c.expression, c.column_ordering"),
			always("ORDER BY 1, 2, 3, 5"),
		},
		Fields: []dbmeta.Field{
			{Name: "schema"}, {Name: "table"}, {Name: "index"}, {Name: "name"},
			{Name: "ordinal", Desc: "one based. A stored column has no ordinal in Spanner, so it is numbered after the key columns in the order of its name"},
			{Name: "expression", Desc: "EXPRESSION, which is absent for every index here. Spanner has no index on an expression"},
			{Name: "descending", Desc: "whether the key column is DESC. Absent for a stored column, which has no order"},
			{Name: "include", Desc: "true for a column that the index STORES and does not sort by"},
		},
		Params: append(schemaParentName("table", "column"),
			dbmeta.Param{Name: "index", Desc: "index name pattern, empty for every index", Default: ""}),
		Scan: func(rows *sql.Rows) (dbmeta.IndexColumn, error) {
			var v dbmeta.IndexColumn
			err := rows.Scan(&v.Schema, &v.Table, &v.Index, &v.Name, &v.Ordinal,
				&v.Expression, &v.Descending, &v.Include)
			return v, err
		},
	})

	// \d name, the constraints. Primary key, foreign key and check. The check
	// constraints that Spanner makes for NOT NULL columns are left out, as psql
	// 18 leaves them out, and NotNulls reads them.
	dbmeta.Constraints.Register(dbmeta.Spanner, &dbmeta.Binding[dbmeta.Constraint]{
		Stmt: dbmeta.Stmt{
			always("SELECT k.table_schema AS `schema`"),
			always(", k.table_name AS `table`"),
			always(", k.constraint_name AS `name`"),
			always(", LOWER(k.constraint_type) AS `type`"),
			always(", ck.check_clause AS `definition`"),
			always(", k.is_deferrable = 'YES' AS `deferrable`"),
			always(", k.initially_deferred = 'YES' AS `deferred`"),
			always(", CAST(NULL AS STRING) AS `comment`"),
			always(", k.enforced = 'YES' AS `enforced`"),
			always("FROM information_schema.table_constraints k"),
			always("LEFT JOIN information_schema.check_constraints ck"),
			always("ON ck.constraint_catalog = k.constraint_catalog AND ck.constraint_schema = k.constraint_schema"),
			always("AND ck.constraint_name = k.constraint_name"),
			always("WHERE NOT " + notNull("k.constraint_name")),
			always("AND " + notSystem("k.table_schema")),
			always("AND " + like("k.table_schema", "@schema")),
			always("AND " + like("k.table_name", "@parent")),
			always("AND " + like("k.constraint_name", "@name")),
			always("ORDER BY 1, 2, 3"),
		},
		Fields: []dbmeta.Field{
			{Name: "schema"}, {Name: "table"}, {Name: "name"},
			{Name: "type", Desc: "primary key, foreign key or check. Spanner has no unique constraint, and a unique index is read by Indexes"},
			{Name: "definition", Desc: "CHECK_CLAUSE for a check constraint, as Spanner writes it without the word CHECK. Absent for a key, whose columns are in ConstraintColumns"},
			{Name: "deferrable", Desc: "IS_DEFERRABLE as the server says it, which is false for every constraint"},
			{Name: "deferred", Desc: "INITIALLY_DEFERRED, which is false for every constraint"},
			{Name: "comment", Desc: "always absent"},
			{Name: "enforced", Desc: "ENFORCED. A foreign key made NOT ENFORCED reads false, and Spanner checks every other"},
		},
		Params: schemaParentName("table", "constraint"),
		Scan: func(rows *sql.Rows) (dbmeta.Constraint, error) {
			var v dbmeta.Constraint
			err := rows.Scan(&v.Schema, &v.Table, &v.Name, &v.Type, &v.Definition,
				&v.Deferrable, &v.Deferred, &v.Comment, &v.Enforced)
			return v, err
		},
	})

	// The columns of a primary key and of a foreign key, and the column a
	// foreign key points at. A foreign key points at the primary key of a table
	// or at a unique index, and REFERENTIAL_CONSTRAINTS names it in either case,
	// so the target is read from the key columns for the first and from the
	// index columns for the second. Both are joined by the position that
	// KEY_COLUMN_USAGE gives.
	dbmeta.ConstraintColumns.Register(dbmeta.Spanner, &dbmeta.Binding[dbmeta.ConstraintColumn]{
		Stmt: dbmeta.Stmt{
			always("SELECT k.table_catalog AS `catalog`"),
			always(", k.table_schema AS `schema`"),
			always(", k.table_name AS `table`"),
			always(", k.constraint_name AS `constraint`"),
			always(", k.column_name AS `name`"),
			always(", k.ordinal_position AS `ordinal`"),
			always(", IF(r.constraint_name IS NULL, NULL, COALESCE(p.table_catalog, c.table_catalog)) AS `foreign_catalog`"),
			always(", IF(r.constraint_name IS NULL, NULL, COALESCE(p.table_schema, c.table_schema)) AS `foreign_schema`"),
			always(", IF(r.constraint_name IS NULL, NULL, COALESCE(p.table_name, c.table_name)) AS `foreign_table`"),
			always(", IF(r.constraint_name IS NULL, NULL, COALESCE(p.column_name, c.column_name)) AS `foreign_name`"),
			always("FROM information_schema.key_column_usage k"),
			always("LEFT JOIN information_schema.referential_constraints r"),
			always("ON r.constraint_catalog = k.constraint_catalog AND r.constraint_schema = k.constraint_schema"),
			always("AND r.constraint_name = k.constraint_name"),
			always("LEFT JOIN information_schema.key_column_usage p"),
			always("ON p.constraint_catalog = r.unique_constraint_catalog AND p.constraint_schema = r.unique_constraint_schema"),
			always("AND p.constraint_name = r.unique_constraint_name"),
			always("AND p.ordinal_position = k.position_in_unique_constraint"),
			always("LEFT JOIN information_schema.index_columns c"),
			always("ON c.table_catalog = r.unique_constraint_catalog AND c.table_schema = r.unique_constraint_schema"),
			always("AND c.index_name = r.unique_constraint_name"),
			always("AND c.ordinal_position = k.position_in_unique_constraint"),
			always("WHERE " + notSystem("k.table_schema")),
			always("AND " + like("k.table_schema", "@schema")),
			always("AND " + like("k.table_name", "@parent")),
			always("AND " + like("k.constraint_name", "@name")),
			always("ORDER BY 2, 3, 4, 6"),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog"}, {Name: "schema"}, {Name: "table"},
			{Name: "constraint"}, {Name: "name"},
			{Name: "ordinal", Desc: "ORDINAL_POSITION, one based. A check constraint has no row here, because KEY_COLUMN_USAGE holds the keys only"},
			{Name: "foreign_catalog", Desc: "for a foreign key, the catalog of the table it points at, which is empty"},
			{Name: "foreign_schema", Desc: "for a foreign key, the schema of the table it points at"},
			{Name: "foreign_table", Desc: "for a foreign key, the table it points at, which holds the primary key or the unique index it names"},
			{Name: "foreign_name", Desc: "for a foreign key, the column it points at"},
		},
		Params: schemaParentName("table", "constraint"),
		Scan: func(rows *sql.Rows) (dbmeta.ConstraintColumn, error) {
			var v dbmeta.ConstraintColumn
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Table, &v.Constraint, &v.Name,
				&v.Ordinal, &v.ForeignCatalog, &v.ForeignSchema, &v.ForeignTable,
				&v.ForeignName)
			return v, err
		},
	})

	// \d+ name, Not-null constraints. Spanner keeps each NOT NULL column as a
	// named check constraint, so these are the same constraints that psql 18
	// prints. The hash join is a hint, because Spanner plans this join as a nested
	// loop, and 1600 constraints then take 14 seconds where a hash join takes
	// 0.04.
	dbmeta.NotNulls.Register(dbmeta.Spanner, &dbmeta.Binding[dbmeta.NotNull]{
		Stmt: dbmeta.Stmt{
			always("SELECT u.table_schema AS `schema`"),
			always(", u.table_name AS `table`"),
			always(", u.constraint_name AS `name`"),
			always(", u.column_name AS `column`"),
			always(", FALSE AS `no_inherit`"),
			always(", TRUE AS `local`"),
			always(", FALSE AS `inherited`"),
			always(", ck.spanner_state = 'COMMITTED' AS `validated`"),
			always("FROM information_schema.constraint_column_usage u"),
			always("JOIN@{JOIN_METHOD=HASH_JOIN} information_schema.check_constraints ck"),
			always("ON ck.constraint_catalog = u.constraint_catalog AND ck.constraint_schema = u.constraint_schema"),
			always("AND ck.constraint_name = u.constraint_name"),
			always("WHERE " + notNull("u.constraint_name")),
			always("AND " + notSystem("u.table_schema")),
			always("AND " + like("u.table_schema", "@schema")),
			always("AND " + like("u.table_name", "@parent")),
			always("AND " + like("u.constraint_name", "@name")),
			always("ORDER BY 1, 2, 3"),
		},
		Fields: []dbmeta.Field{
			{Name: "schema"}, {Name: "table"},
			{Name: "name", Desc: "the name Spanner gives, CK_IS_NOT_NULL_ and the table and the column joined by an underscore"},
			{Name: "column"},
			{Name: "no_inherit", Desc: "always false: Spanner has no inheritance"},
			{Name: "local", Desc: "always true, for the same reason"},
			{Name: "inherited", Desc: "always false, for the same reason"},
			{Name: "validated", Desc: "whether SPANNER_STATE is COMMITTED, which is false while Spanner is still adding the constraint"},
		},
		Params: schemaParentName("table", "constraint"),
		Scan: func(rows *sql.Rows) (dbmeta.NotNull, error) {
			var v dbmeta.NotNull
			err := rows.Scan(&v.Schema, &v.Table, &v.Name, &v.Column, &v.NoInherit,
				&v.Local, &v.Inherited, &v.Validated)
			return v, err
		},
	})
}
