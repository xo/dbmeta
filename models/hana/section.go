package hana

import (
	"database/sql"

	"github.com/xo/dbmeta"
)

// partitionBound is the bound of one level of a partition. A range level
// gives its minimum and its maximum, and the partition OTHERS gives neither. A
// hash level gives the place of the partition among the partitions of that
// level, which is the nearest HANA has to a modulus and a remainder.
func partitionBound(level, count, kind string) string {
	n := `p.LEVEL_` + level + `_PARTITION`
	return `CASE WHEN ` + n + ` IS NULL OR ` + n + ` = 0 THEN ''` +
		` WHEN LENGTH(COALESCE(p.LEVEL_` + level + `_RANGE_MIN_VALUE, '')) > 0` +
		` OR LENGTH(COALESCE(p.LEVEL_` + level + `_RANGE_MAX_VALUE, '')) > 0` +
		` THEN ', ' || COALESCE(p.LEVEL_` + level + `_RANGE_MIN_VALUE, '') || ' <= VALUES < '` +
		` || COALESCE(p.LEVEL_` + level + `_RANGE_MAX_VALUE, '')` +
		` WHEN d.` + kind + ` = 'RANGE' THEN ', OTHERS'` +
		` WHEN d.` + kind + ` = 'HASH' THEN ', HASH ' || CAST(` + n + ` AS NVARCHAR(10)) || ' OF '` +
		` || CAST(d.` + count + ` AS NVARCHAR(10)) ELSE '' END`
}

func registerSections() {
	// The partitions of a table. TABLE_PARTITIONS has one row for each leaf
	// partition, and a partition has a number and no name, so the number is
	// the name. A table can partition on up to three levels, and the bound
	// joins the bound of each level with a comma. PARTITIONED_TABLES gives the
	// kind of each level. A HANA partition has no partitions of its own below
	// the leaf, and the levels are one row, so Partitioned is false.
	dbmeta.Partitions.Register(dbmeta.HANA, &dbmeta.Binding[dbmeta.Partition]{
		Stmt: dbmeta.Stmt{
			always(`SELECT p.SCHEMA_NAME AS "schema"`),
			always(`, p.TABLE_NAME AS "table"`),
			always(`, p.SCHEMA_NAME AS "partition_schema"`),
			always(`, CAST(p.PART_ID AS NVARCHAR(10)) AS "partition"`),
			always(`, 'partition' AS "type"`),
			always(`, NULLIF(SUBSTRING(` + partitionBound("1", "LEVEL_1_COUNT", "LEVEL_1_TYPE") +
				` || ` + partitionBound("2", "LEVEL_2_COUNT", "LEVEL_2_TYPE") +
				` || ` + partitionBound("3", "LEVEL_3_COUNT", "LEVEL_3_TYPE") + `, 3), '') AS "bound"`),
			always(`, FALSE AS "partitioned"`),
			always(`FROM SYS.TABLE_PARTITIONS p`),
			always(`JOIN SYS.PARTITIONED_TABLES d ON d.SCHEMA_NAME = p.SCHEMA_NAME AND d.TABLE_NAME = p.TABLE_NAME`),
			always(`WHERE ` + notSystem(`p.SCHEMA_NAME`)),
			always(`AND ` + like(`p.SCHEMA_NAME`, `@schema`)),
			always(`AND ` + like(`p.TABLE_NAME`, `@parent`)),
			always(`AND ` + like(`CAST(p.PART_ID AS NVARCHAR(10))`, `@name`)),
			always(`ORDER BY p.SCHEMA_NAME, p.TABLE_NAME, p.PART_ID`),
		},
		Fields: []dbmeta.Field{
			{Name: "schema"}, {Name: "table"},
			{Name: "partition_schema", Desc: "the schema of the table, because a partition lives where its table does"},
			{Name: "partition", Desc: "PART_ID as text. A HANA partition has a number and no name"},
			{Name: "type", Desc: "always partition"},
			{Name: "bound", Desc: "for a range level, the minimum <= VALUES < the maximum, or OTHERS for the partition that takes the rest. For a hash level, HASH n OF m. A table with more levels joins them with a comma. Absent for a level of another kind"},
			{Name: "partitioned", Desc: "always false: the row is a leaf partition, and the levels are in the bound"},
		},
		Params: parentAndName("partition"),
		Scan: func(rows *sql.Rows) (dbmeta.Partition, error) {
			var v dbmeta.Partition
			err := rows.Scan(&v.Schema, &v.Table, &v.PartitionSchema, &v.Partition,
				&v.Type, &v.Bound, &v.Partitioned)
			return v, err
		},
	})
}
