package bigquery

import (
	"database/sql"

	"github.com/xo/dbmeta"
)

// Datasets and the grants on a dataset.

// datasetFilter keeps the SCHEMATA row of the dataset that the connection reads.
// SCHEMATA lists every dataset of the project, and @@dataset_id is NULL on a
// session with a default dataset, so no statement can read the name of that
// dataset. The tables of the dataset name it, because INFORMATION_SCHEMA.TABLES
// names no dataset in front and reads only the default one. A dataset with no
// table or view has no row, for that reason.
const datasetFilter = `s.schema_name IN (SELECT table_schema FROM INFORMATION_SCHEMA.TABLES)`

// schemaStmt reads the dataset of the connection with its description and
// options, which SCHEMATA_OPTIONS holds one to a row. The options are read once
// and joined, as tableJoin does.
func schemaStmt() dbmeta.Stmt {
	return dbmeta.Stmt{
		always("SELECT s.catalog_name AS `catalog`"),
		always(", s.schema_name AS `name`"),
		always(", s.schema_owner AS `owner`"),
		always(", o.description AS `comment`"),
		always(", CAST(NULL AS STRING) AS `access`"),
		always(", o.options AS `options`"),
		always("FROM INFORMATION_SCHEMA.SCHEMATA s"),
		always("LEFT JOIN (SELECT catalog_name, schema_name"),
		always(", MAX(IF(option_name = 'description', " + unquote("option_value") + ", NULL)) AS description"),
		always(", STRING_AGG(IF(option_name = 'description', NULL, option_name || '=' || option_value)"),
		always(", ', ' ORDER BY option_name) AS options"),
		always("FROM INFORMATION_SCHEMA.SCHEMATA_OPTIONS GROUP BY catalog_name, schema_name) o"),
		always("ON o.catalog_name = s.catalog_name AND o.schema_name = s.schema_name"),
		always("WHERE " + datasetFilter),
	}
}

var schemaFields = []dbmeta.Field{
	{Name: "catalog", Desc: "the project"},
	{Name: "name", Desc: "the dataset that the connection reads. It is the only row, because a statement cannot name another dataset, and a dataset with no table has none"},
	{Name: "owner", Desc: "always empty: SCHEMATA leaves the owner empty, because access is by IAM"},
	{Name: "comment", Desc: "the description option of the dataset"},
	{Name: "access", Desc: "always absent: the grants on the dataset are in Privileges"},
	{Name: "options", Desc: "the other options of the dataset, such as location, default_table_expiration_days and max_time_travel_hours, with their literals as BigQuery writes them"},
}

func scanSchema(rows *sql.Rows) (dbmeta.Schema, error) {
	var v dbmeta.Schema
	err := rows.Scan(&v.Catalog, &v.Name, dbmeta.NullAsEmpty(&v.Owner), &v.Comment, &v.Access, &v.Options)
	return v, err
}

// privilegeColumns is the column list of the grants on a dataset. FORMAT reads
// the percent signs of a statement, so the list holds none.
const privilegeColumns = "CAST(NULL AS STRING) AS `schema`" +
	", p.object_name AS `name`" +
	", LOWER(p.object_type) AS `type`" +
	", p.grantee || \"=\" || p.privilege_type AS `access`" +
	", CAST(NULL AS STRING) AS `column_access`" +
	", CAST(NULL AS STRING) AS `policies`"

// privilegeStmt reads OBJECT_PRIVILEGES for the dataset of the connection.
//
// OBJECT_PRIVILEGES is a view of the region and answers only a query whose
// WHERE clause names one object. The name of the view holds the region, and a
// statement cannot bind a name, so the statement builds the second statement
// as text and runs it. The location comes from SCHEMATA and the object from
// datasetFilter. A dataset that has no table has no name to read, and the
// statement reads no row for it.
func privilegeStmt() dbmeta.Stmt {
	inner := "SELECT " + privilegeColumns +
		" FROM `region-%s`.INFORMATION_SCHEMA.OBJECT_PRIVILEGES p WHERE p.object_name = \"%s\" ORDER BY 4"
	none := "SELECT " + privilegeColumns +
		" FROM (SELECT \"\" AS object_name, \"\" AS object_type, \"\" AS grantee, \"\" AS privilege_type) p WHERE FALSE"
	return dbmeta.Stmt{
		always("EXECUTE IMMEDIATE IFNULL((SELECT FORMAT('" + inner + "', LOWER(s.location), s.schema_name)"),
		always("FROM INFORMATION_SCHEMA.SCHEMATA s WHERE " + datasetFilter + "), '" + none + "')"),
	}
}

func registerSchemas() {
	// \dn. The one dataset that the connection reads, with its description in
	// the comment and its other options in Options.
	dbmeta.Schemas.Register(dbmeta.BigQuery, &dbmeta.Binding[dbmeta.Schema]{
		Stmt: append(schemaStmt(),
			always("AND "+like("s.schema_name", "@name")),
			always("ORDER BY 2")),
		Fields: schemaFields,
		Params: nameOnly("dataset"),
		Scan:   scanSchema,
	})

	// The dataset that an unqualified name resolves in, which is the default
	// dataset of the connection.
	dbmeta.CurrentSchema.Register(dbmeta.BigQuery, &dbmeta.Binding[dbmeta.Schema]{
		Stmt:   schemaStmt(),
		Fields: schemaFields,
		Scan:   scanSchema,
	})

	// \dp for the dataset. A table has no grant of its own unless somebody set
	// one, and OBJECT_PRIVILEGES answers one object at a time, so the grants of
	// the tables are not read.
	dbmeta.Privileges.Register(dbmeta.BigQuery, &dbmeta.Binding[dbmeta.Privilege]{
		Stmt: privilegeStmt(),
		Fields: []dbmeta.Field{
			{Name: "schema", Desc: "always absent: the row is the dataset itself"},
			{Name: "name", Desc: "the dataset that the connection reads"},
			{Name: "type", Desc: "schema, which OBJECT_PRIVILEGES calls a dataset"},
			{Name: "access", Desc: "one grant per row, as grantee=role, where the grantee is an IAM principal such as serviceAccount:name@project.iam.gserviceaccount.com. There is no grantor"},
			{Name: "column_access", Desc: "always absent: BigQuery has no grant on a column here"},
			{Name: "policies", Desc: "always absent: no view lists a row access policy"},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Privilege, error) {
			var v dbmeta.Privilege
			err := rows.Scan(&v.Schema, &v.Name, &v.Type, &v.Access, &v.ColumnAccess, &v.Policies)
			return v, err
		},
	})
}
