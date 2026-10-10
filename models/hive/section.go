package hive

import (
	"database/sql"

	"github.com/xo/dbmeta"
)

func registerSections() {
	// A partition of a table. PARTITIONS has one row for each partition, and
	// PART_NAME is the values of the partition keys written as key=value and
	// joined by a slash, such as year=2026/month=10. That text is the whole
	// bound, because a Hive partition has no range and no list of values apart
	// from it. A partition is a directory and not a table, so its type is
	// partition, and Hive has no partition of a partition. A table with more
	// than two partition keys still has one level, which the name carries.
	dbmeta.Partitions.Register(dbmeta.Hive, &dbmeta.Binding[dbmeta.Partition]{
		Stmt: dbmeta.Stmt{
			always(`SELECT d.NAME AS "schema"`),
			always(`, t.TBL_NAME AS "table"`),
			always(`, d.NAME AS "partition_schema"`),
			always(`, p.PART_NAME AS "partition"`),
			always(`, 'partition' AS "type"`),
			always(`, p.PART_NAME AS "bound"`),
			always(`, FALSE AS "partitioned"`),
			always(`FROM sys.PARTITIONS p`),
			always(`JOIN sys.TBLS t ON t.TBL_ID = p.TBL_ID`),
			always(`JOIN sys.DBS d ON d.DB_ID = t.DB_ID`),
			always(`WHERE ` + notSystem),
			always(`AND ` + like(`d.NAME`, `@schema`)),
			always(`AND ` + like(`t.TBL_NAME`, `@parent`)),
			always(`AND ` + like(`p.PART_NAME`, `@name`)),
			always(`ORDER BY d.NAME, t.TBL_NAME, p.PART_NAME`),
		},
		Fields: []dbmeta.Field{
			{Name: "schema"}, {Name: "table"},
			{Name: "partition_schema", Desc: "the database of the table, because a partition lives where its table does"},
			{Name: "partition", Desc: "PART_NAME, the key values as key=value joined by a slash"},
			{Name: "type", Desc: "always partition: a Hive partition is a directory and not a table"},
			{Name: "bound", Desc: "the same text as partition. Hive has no range or list apart from the values in the name"},
			{Name: "partitioned", Desc: "always false: Hive has no partition of a partition"},
		},
		Params: parentAndName("partition"),
		Scan: func(rows *sql.Rows) (dbmeta.Partition, error) {
			var v dbmeta.Partition
			err := rows.Scan(&v.Schema, &v.Table, &v.PartitionSchema, &v.Partition,
				&v.Type, &v.Bound, &v.Partitioned)
			return v, err
		},
	})

	// A NOT NULL is a constraint with a name in KEY_CONSTRAINTS, type 3. Hive
	// writes nn_ and a number when the statement named none. Validated is the
	// VALIDATE bit of ENABLE_VALIDATE_RELY, which is set when Hive checked the
	// rows that were already in the table. Hive has no table inheritance.
	dbmeta.NotNulls.Register(dbmeta.Hive, &dbmeta.Binding[dbmeta.NotNull]{
		Stmt: dbmeta.Stmt{
			always(`SELECT d.NAME AS "schema"`),
			always(`, t.TBL_NAME AS "table"`),
			always(`, k.CONSTRAINT_NAME AS "name"`),
			always(`, c.COLUMN_NAME AS "column"`),
			always(`, FALSE AS "no_inherit"`),
			always(`, TRUE AS "local"`),
			always(`, FALSE AS "inherited"`),
			always(`, (k.ENABLE_VALIDATE_RELY & 2) = 2 AS "validated"`),
			always(`FROM ` + keyConstraints),
			always(`JOIN sys.TBLS t ON t.TBL_ID = k.OWNER_TBL_ID`),
			always(`JOIN sys.DBS d ON d.DB_ID = t.DB_ID`),
			always(`JOIN sys.COLUMNS_V2 c ON c.CD_ID = k.OWNER_CD_ID AND c.INTEGER_IDX = k.OWNER_IDX`),
			always(`WHERE k.CONSTRAINT_TYPE = 3`),
			always(`AND ` + notSystem),
			always(`AND ` + like(`d.NAME`, `@schema`)),
			always(`AND ` + like(`t.TBL_NAME`, `@parent`)),
			always(`AND ` + like(`k.CONSTRAINT_NAME`, `@name`)),
			always(`ORDER BY d.NAME, t.TBL_NAME, c.COLUMN_NAME`),
		},
		Fields: []dbmeta.Field{
			{Name: "schema"}, {Name: "table"}, {Name: "name"}, {Name: "column"},
			{Name: "no_inherit", Desc: "always false: Hive has no table inheritance"},
			{Name: "local", Desc: "always true, for the same reason"},
			{Name: "inherited", Desc: "always false, for the same reason"},
			{Name: "validated", Desc: "the VALIDATE bit of ENABLE_VALIDATE_RELY"},
		},
		Params: parentAndName("constraint"),
		Scan: func(rows *sql.Rows) (dbmeta.NotNull, error) {
			var v dbmeta.NotNull
			err := rows.Scan(&v.Schema, &v.Table, &v.Name, &v.Column, &v.NoInherit,
				&v.Local, &v.Inherited, &v.Validated)
			return v, err
		},
	})
}
