package bigquery

import (
	"database/sql"

	"github.com/xo/dbmeta"
)

// The detail of a table: index columns, constraints and the columns of a key.

func registerDetail() {
	// The columns of an index. A search index or a vector index lists its columns
	// with no order, so the ordinal numbers them by name, which is the only order
	// that holds from one run to the next. A column inside a record is the
	// column and a path, and the path is the expression.
	dbmeta.IndexColumns.Register(dbmeta.BigQuery, &dbmeta.Binding[dbmeta.IndexColumn]{
		Stmt: dbmeta.Stmt{
			always("SELECT c.schema AS `schema`"),
			always(", c.table_name AS `table`"),
			always(", c.index_name AS `index`"),
			always(", c.column_name AS `name`"),
			always(", ROW_NUMBER() OVER (PARTITION BY c.schema, c.table_name, c.index_name ORDER BY c.column_name) AS `ordinal`"),
			always(", IF(c.field_path = c.column_name, NULL, c.field_path) AS `expression`"),
			always(", CAST(NULL AS BOOL) AS `descending`"),
			always(", FALSE AS `include`"),
			always("FROM (SELECT index_schema AS schema, table_name, index_name"),
			always(", index_column_name AS column_name, index_field_path AS field_path"),
			always("FROM INFORMATION_SCHEMA.SEARCH_INDEX_COLUMNS"),
			always("UNION ALL SELECT index_schema, table_name, index_name, index_column_name, index_field_path"),
			always("FROM INFORMATION_SCHEMA.VECTOR_INDEX_COLUMNS) c"),
			always("WHERE " + like("c.schema", "@schema")),
			always("AND " + like("c.table_name", "@parent")),
			always("AND " + like("c.index_name", "@index")),
			always("AND " + like("c.column_name", "@name")),
			always("ORDER BY 1, 2, 3, 5"),
		},
		Fields: []dbmeta.Field{
			{Name: "schema"}, {Name: "table"}, {Name: "index"}, {Name: "name"},
			{Name: "ordinal", Desc: "one based, and in the order of the column names, because BigQuery keeps no order for the columns of an index"},
			{Name: "expression", Desc: "INDEX_FIELD_PATH for a column that is a path into a record, and absent for a column indexed whole"},
			{Name: "descending", Desc: "always absent: a search index and a vector index have no sort order"},
			{Name: "include", Desc: "always false: BigQuery has no stored column in an index"},
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

	// \d name, the constraints. BigQuery has a primary key and a foreign key, and
	// it checks neither. It has no unique constraint and no check constraint. A
	// constraint name begins with the table, as book.book_author_fk, and the
	// primary key is named table.pk$.
	dbmeta.Constraints.Register(dbmeta.BigQuery, &dbmeta.Binding[dbmeta.Constraint]{
		Stmt: dbmeta.Stmt{
			always("SELECT k.table_schema AS `schema`"),
			always(", k.table_name AS `table`"),
			always(", k.constraint_name AS `name`"),
			always(", LOWER(k.constraint_type) AS `type`"),
			always(", CAST(NULL AS STRING) AS `definition`"),
			always(", k.is_deferrable = 'YES' AS `deferrable`"),
			always(", k.initially_deferred = 'YES' AS `deferred`"),
			always(", CAST(NULL AS STRING) AS `comment`"),
			always(", k.enforced = 'YES' AS `enforced`"),
			always("FROM INFORMATION_SCHEMA.TABLE_CONSTRAINTS k"),
			always("WHERE " + like("k.table_schema", "@schema")),
			always("AND " + like("k.table_name", "@parent")),
			always("AND " + like("k.constraint_name", "@name")),
			always("ORDER BY 1, 2, 3"),
		},
		Fields: []dbmeta.Field{
			{Name: "schema"}, {Name: "table"},
			{Name: "name", Desc: "the name BigQuery reports, which begins with the table and a dot, and is table.pk$ for a primary key"},
			{Name: "type", Desc: "primary key or foreign key. BigQuery has no unique constraint and no check constraint"},
			{Name: "definition", Desc: "always absent: the columns of a key are in ConstraintColumns, and BigQuery keeps no text for it"},
			{Name: "deferrable", Desc: "IS_DEFERRABLE, which is false for every constraint"},
			{Name: "deferred", Desc: "INITIALLY_DEFERRED, which is false for every constraint"},
			{Name: "comment", Desc: "always absent"},
			{Name: "enforced", Desc: "ENFORCED, which is false for every constraint, because BigQuery records a key and never checks it"},
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
	// foreign key points at. CONSTRAINT_COLUMN_USAGE names the table that a foreign
	// key points at, on the rows of the key. KEY_COLUMN_USAGE gives the position of
	// each column in the primary key of that table, so the column it points at is
	// the one of the primary key at that position. A table in another dataset has
	// no primary key that this dataset can read, so its column is absent.
	dbmeta.ConstraintColumns.Register(dbmeta.BigQuery, &dbmeta.Binding[dbmeta.ConstraintColumn]{
		Stmt: dbmeta.Stmt{
			always("SELECT k.table_catalog AS `catalog`"),
			always(", k.table_schema AS `schema`"),
			always(", k.table_name AS `table`"),
			always(", k.constraint_name AS `constraint`"),
			always(", k.column_name AS `name`"),
			always(", k.ordinal_position AS `ordinal`"),
			always(", f.table_catalog AS `foreign_catalog`"),
			always(", f.table_schema AS `foreign_schema`"),
			always(", f.table_name AS `foreign_table`"),
			always(", IF(f.table_name IS NULL, NULL, p.column_name) AS `foreign_name`"),
			always("FROM INFORMATION_SCHEMA.KEY_COLUMN_USAGE k"),
			always("JOIN INFORMATION_SCHEMA.TABLE_CONSTRAINTS s"),
			always("ON s.constraint_catalog = k.constraint_catalog AND s.constraint_schema = k.constraint_schema"),
			always("AND s.constraint_name = k.constraint_name"),
			always("LEFT JOIN (SELECT constraint_catalog, constraint_schema, constraint_name"),
			always(", ANY_VALUE(table_catalog) AS table_catalog, ANY_VALUE(table_schema) AS table_schema"),
			always(", ANY_VALUE(table_name) AS table_name"),
			always("FROM INFORMATION_SCHEMA.CONSTRAINT_COLUMN_USAGE"),
			always("GROUP BY constraint_catalog, constraint_schema, constraint_name) f"),
			always("ON s.constraint_type = 'FOREIGN KEY' AND f.constraint_catalog = k.constraint_catalog"),
			always("AND f.constraint_schema = k.constraint_schema AND f.constraint_name = k.constraint_name"),
			always("LEFT JOIN (SELECT u.table_catalog, u.table_schema, u.table_name, u.ordinal_position, u.column_name"),
			always("FROM INFORMATION_SCHEMA.KEY_COLUMN_USAGE u JOIN INFORMATION_SCHEMA.TABLE_CONSTRAINTS t"),
			always("ON t.constraint_catalog = u.constraint_catalog AND t.constraint_schema = u.constraint_schema"),
			always("AND t.constraint_name = u.constraint_name WHERE t.constraint_type = 'PRIMARY KEY') p"),
			always("ON p.table_catalog = f.table_catalog AND p.table_schema = f.table_schema"),
			always("AND p.table_name = f.table_name AND p.ordinal_position = k.position_in_unique_constraint"),
			always("WHERE " + like("k.table_schema", "@schema")),
			always("AND " + like("k.table_name", "@parent")),
			always("AND " + like("k.constraint_name", "@name")),
			always("ORDER BY 2, 3, 4, 6"),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog"}, {Name: "schema"}, {Name: "table"},
			{Name: "constraint"}, {Name: "name"},
			{Name: "ordinal", Desc: "ORDINAL_POSITION, one based. The order that the key was written in"},
			{Name: "foreign_catalog", Desc: "for a foreign key, the project of the table it points at"},
			{Name: "foreign_schema", Desc: "for a foreign key, the dataset of the table it points at"},
			{Name: "foreign_table", Desc: "for a foreign key, the table it points at"},
			{Name: "foreign_name", Desc: "for a foreign key, the column it points at, which is the column of the primary key of that table at POSITION_IN_UNIQUE_CONSTRAINT. Absent when the table is in another dataset"},
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
}
