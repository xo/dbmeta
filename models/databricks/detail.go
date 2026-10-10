package databricks

import (
	"database/sql"

	"github.com/xo/dbmeta"
)

// The detail of a table: constraints and the columns of a key.

// keyColumns is a relation of the columns of each key, joined into one text in
// the order of the key. The key is named by its catalog, schema and name.
const keyColumns = "(SELECT constraint_catalog, constraint_schema, constraint_name" +
	", concat_ws(', ', transform(array_sort(collect_list(struct(ordinal_position AS o, column_name AS c))), x -> x.c)) AS cols" +
	" FROM information_schema.key_column_usage GROUP BY constraint_catalog, constraint_schema, constraint_name)"

func registerDetail() {
	// \d name, the constraints. Databricks has a primary key and a foreign key,
	// and it checks neither. It has no unique constraint. A CHECK constraint of a
	// Delta table is kept in the table properties and is not listed.
	dbmeta.Constraints.Register(dbmeta.Databricks, &dbmeta.Binding[dbmeta.Constraint]{
		Stmt: dbmeta.Stmt{
			always("SELECT k.table_schema AS `schema`"),
			always(", k.table_name AS `table`"),
			always(", k.constraint_name AS `name`"),
			always(", LOWER(k.constraint_type) AS `type`"),
			always(", k.constraint_type || ' (' || kc.cols || ')'"),
			always("|| IF(r.constraint_name IS NULL, '', ' REFERENCES ' || ut.table_schema || '.' || ut.table_name"),
			always("|| ' (' || rc.cols || ')') AS `definition`"),
			always(", k.is_deferrable = 'YES' AS `deferrable`"),
			always(", k.initially_deferred = 'YES' AS `deferred`"),
			always(", CAST(NULL AS STRING) AS `comment`"),
			always(", k.enforced = 'YES' AS `enforced`"),
			always("FROM information_schema.table_constraints k"),
			always("LEFT JOIN " + keyColumns + " kc ON kc.constraint_catalog = k.constraint_catalog"),
			always("AND kc.constraint_schema = k.constraint_schema AND kc.constraint_name = k.constraint_name"),
			always("LEFT JOIN information_schema.referential_constraints r ON r.constraint_catalog = k.constraint_catalog"),
			always("AND r.constraint_schema = k.constraint_schema AND r.constraint_name = k.constraint_name"),
			always("LEFT JOIN information_schema.table_constraints ut ON ut.constraint_catalog = r.unique_constraint_catalog"),
			always("AND ut.constraint_schema = r.unique_constraint_schema AND ut.constraint_name = r.unique_constraint_name"),
			always("LEFT JOIN " + keyColumns + " rc ON rc.constraint_catalog = r.unique_constraint_catalog"),
			always("AND rc.constraint_schema = r.unique_constraint_schema AND rc.constraint_name = r.unique_constraint_name"),
			always("WHERE " + notSystem("k.table_schema")),
			always("AND " + like("k.table_schema", "@schema")),
			always("AND " + like("k.table_name", "@parent")),
			always("AND " + like("k.constraint_name", "@name")),
			always("ORDER BY 1, 2, 3"),
		},
		Fields: []dbmeta.Field{
			{Name: "schema"}, {Name: "table"},
			{Name: "name", Desc: "the name that the constraint was made with, or a name that Databricks gives to a key it copies, such as author_clone_pk for a clone of author"},
			{Name: "type", Desc: "primary key or foreign key. Databricks has no unique constraint, and a CHECK constraint of Delta is not listed"},
			{Name: "definition", Desc: "the key as a statement writes it, such as PRIMARY KEY (country, area) or FOREIGN KEY (country, area) REFERENCES dbmeta.region (country, area). Databricks keeps no text for it, so the model builds it from the columns"},
			{Name: "deferrable", Desc: "IS_DEFERRABLE, which is true for every constraint, although a key is never checked and so never deferred"},
			{Name: "deferred", Desc: "INITIALLY_DEFERRED, which is true for every constraint, for the same reason"},
			{Name: "comment", Desc: "always absent: Databricks keeps no comment for a constraint"},
			{Name: "enforced", Desc: "ENFORCED, which is false for every constraint, because Databricks records a key and never checks it"},
		},
		Params: schemaParentName("table", "constraint"),
		Scan: func(rows *sql.Rows) (dbmeta.Constraint, error) {
			var v dbmeta.Constraint
			err := rows.Scan(&v.Schema, &v.Table, &v.Name, &v.Type, &v.Definition,
				&v.Deferrable, &v.Deferred, &v.Comment, &v.Enforced)
			return v, err
		},
	})

	// The columns of a key, and the column a foreign key points at. KEY_COLUMN_USAGE
	// gives the position of each column in the key that is pointed at, and the
	// column of that key at the position is the one.
	dbmeta.ConstraintColumns.Register(dbmeta.Databricks, &dbmeta.Binding[dbmeta.ConstraintColumn]{
		Stmt: dbmeta.Stmt{
			always("SELECT k.table_catalog AS `catalog`"),
			always(", k.table_schema AS `schema`"),
			always(", k.table_name AS `table`"),
			always(", k.constraint_name AS `constraint`"),
			always(", k.column_name AS `name`"),
			always(", k.ordinal_position AS `ordinal`"),
			always(", ut.table_catalog AS `foreign_catalog`"),
			always(", ut.table_schema AS `foreign_schema`"),
			always(", ut.table_name AS `foreign_table`"),
			always(", fk.column_name AS `foreign_name`"),
			always("FROM information_schema.key_column_usage k"),
			always("LEFT JOIN information_schema.referential_constraints r ON r.constraint_catalog = k.constraint_catalog"),
			always("AND r.constraint_schema = k.constraint_schema AND r.constraint_name = k.constraint_name"),
			always("LEFT JOIN information_schema.table_constraints ut ON ut.constraint_catalog = r.unique_constraint_catalog"),
			always("AND ut.constraint_schema = r.unique_constraint_schema AND ut.constraint_name = r.unique_constraint_name"),
			always("LEFT JOIN information_schema.key_column_usage fk ON fk.constraint_catalog = r.unique_constraint_catalog"),
			always("AND fk.constraint_schema = r.unique_constraint_schema AND fk.constraint_name = r.unique_constraint_name"),
			always("AND fk.ordinal_position = k.position_in_unique_constraint"),
			always("WHERE " + notSystem("k.table_schema")),
			always("AND " + like("k.table_schema", "@schema")),
			always("AND " + like("k.table_name", "@parent")),
			always("AND " + like("k.constraint_name", "@name")),
			always("ORDER BY 2, 3, 4, 6"),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog"}, {Name: "schema"}, {Name: "table"},
			{Name: "constraint"}, {Name: "name"},
			{Name: "ordinal", Desc: "ORDINAL_POSITION, one based. The order that the key was written in"},
			{Name: "foreign_catalog", Desc: "for a foreign key, the catalog of the table it points at"},
			{Name: "foreign_schema", Desc: "for a foreign key, the schema of the table it points at"},
			{Name: "foreign_table", Desc: "for a foreign key, the table it points at"},
			{Name: "foreign_name", Desc: "for a foreign key, the column it points at, which is the column of the key of that table at POSITION_IN_UNIQUE_CONSTRAINT"},
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
