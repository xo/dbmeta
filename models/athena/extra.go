package athena

import (
	"database/sql"

	"github.com/xo/dbmeta"
)

func registerExtra() {
	// \dP. A Hive table with a partition column. The column is a column of the
	// table and COLUMNS marks it with the extra_info partition key.
	dbmeta.PartitionedTables.Register(dbmeta.Athena, &dbmeta.Binding[dbmeta.PartitionedTable]{
		Stmt: dbmeta.Stmt{
			always(`SELECT c.table_schema AS "schema"`),
			always(`, c.table_name AS "name"`),
			always(`, '' AS "owner"`),
			always(`, 'table' AS "type"`),
			always(`, '' AS "parent"`),
			always(`, 'list' AS "strategy"`),
			always(`, c.column_name AS "expression"`),
			always(`, c.comment AS "comment"`),
			always(`FROM information_schema.columns c`),
			always(`WHERE c.extra_info = 'partition key'`),
			always(`AND ` + notSystem("c.table_schema")),
			always(`AND ` + like("c.table_schema", "@schema")),
			always(`AND ` + like("c.table_name", "@name")),
			always(`ORDER BY 1, 2, c.ordinal_position`),
		},
		Fields: []dbmeta.Field{
			{Name: "schema"}, {Name: "name"},
			{Name: "owner", Desc: "always empty: a table has no owner that INFORMATION_SCHEMA reports"},
			{Name: "type", Desc: "always table: Athena partitions a table and nothing else"},
			{Name: "parent", Desc: "always empty: a partition is a prefix in S3 and not a table"},
			{Name: "strategy", Desc: "always list: a Hive table is partitioned by the value of a column"},
			{Name: "expression", Desc: "the partition column. A table partitioned by two columns has two rows, in the order of the columns. An Iceberg table partitioned by a transform has none, because COLUMNS does not mark its columns"},
			{Name: "comment", Desc: "the comment of the partition column. Absent when none is set"},
		},
		Params: []dbmeta.Param{
			{Name: "schema", Desc: "Glue database name pattern, empty for every database", Default: ""},
			{Name: "name", Desc: "table name pattern, empty for every table", Default: ""},
			{Name: "with_system", Desc: "include information_schema, which Athena lists as a schema", Default: false},
		},
		Scan: func(rows *sql.Rows) (dbmeta.PartitionedTable, error) {
			var v dbmeta.PartitionedTable
			err := rows.Scan(&v.Schema, &v.Name, &v.Owner, &v.Type, &v.Parent,
				&v.Strategy, &v.Expression, &v.Comment)
			return v, err
		},
	})

	// The schema of the session, which is the database that the connection names.
	dbmeta.CurrentSchema.Register(dbmeta.Athena, &dbmeta.Binding[dbmeta.Schema]{
		Stmt: dbmeta.Stmt{
			always(`SELECT current_catalog AS "catalog"`),
			always(`, current_schema AS "name"`),
			always(`, '' AS "owner"`),
			always(`, CAST(NULL AS varchar) AS "comment"`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: "the session catalog"},
			{Name: "name", Desc: "the Glue database that the connection names. Absent when the connection names none"},
			{Name: "owner", Desc: "always empty, as it is in Schemas"},
			{Name: "comment", Desc: "always absent, as it is in Schemas"},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Schema, error) {
			var v dbmeta.Schema
			err := rows.Scan(&v.Catalog, &v.Name, &v.Owner, &v.Comment)
			return v, err
		},
	})

	dbmeta.CurrentUser.Register(dbmeta.Athena, &dbmeta.Binding[dbmeta.User]{
		Stmt: dbmeta.Stmt{
			always(`SELECT current_user AS "name"`),
			always(`, CAST(NULL AS varchar) AS "session"`),
		},
		Fields: []dbmeta.Field{
			{Name: "name", Desc: "current_user, which is the number of the AWS account for an IAM user and not the name of the user, so two users of one account read the same value"},
			{Name: "session", Desc: "always absent: an Athena session has no second user"},
		},
		Scan: func(rows *sql.Rows) (dbmeta.User, error) {
			var v dbmeta.User
			err := rows.Scan(&v.Name, &v.Session)
			return v, err
		},
	})
}
