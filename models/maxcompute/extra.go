package maxcompute

import (
	"database/sql"
	"strings"

	"github.com/xo/dbmeta"
)

// Partitions, comments, roles, privileges, functions, the current schema and the
// current user.

// grants aggregates the privileges of one view into one text for each object, a
// line for each privilege, as grantee=privilege. GRANTOR is empty in every row
// that was seen, so a grantor follows a slash only when there is one. The key
// columns are the ones that name the object in the view, and the result names them
// the same. The view is read once and a statement joins it, so the cost does not
// grow with the number of objects.
func grants(view string, key ...string) string {
	list := strings.Join(key, ", ")
	return `(SELECT ` + list + `, concat_ws(chr(10), sort_array(collect_list(` +
		`grantee || '=' || privilege_type || COALESCE('/' || grantor, '')))) AS acl` +
		` FROM information_schema.` + view + ` GROUP BY ` + list + `)`
}

func registerExtra() {
	// The principal that the connection signed in as. get_user_id() reads the
	// session and needs no view, so it answers before INFORMATION_SCHEMA is
	// installed.
	dbmeta.CurrentUser.Register(dbmeta.MaxCompute, &dbmeta.Binding[dbmeta.User]{
		Stmt: dbmeta.Stmt{
			always("SELECT get_user_id() AS `name`"),
			last(", CAST(NULL AS STRING) AS `session`"),
		},
		Fields: []dbmeta.Field{
			{Name: "name", Desc: "get_user_id(), which is the id of the user in the project, such as p4_ and a number, and the name that TABLES reports as the owner of a table. USERS holds the user name, which is RAM$ and the account and the user"},
			{Name: "session", Desc: "always absent: a session has no second user"},
		},
		Scan: func(rows *sql.Rows) (dbmeta.User, error) {
			var v dbmeta.User
			err := rows.Scan(&v.Name, &v.Session)
			return v, err
		},
	})

	// The schema of the session, which is the project.
	dbmeta.CurrentSchema.Register(dbmeta.MaxCompute, &dbmeta.Binding[dbmeta.Schema]{
		Stmt: dbmeta.Stmt{
			always("SELECT '" + catalog + "' AS `catalog`"),
			always(", p.name AS `name`"),
			always(", '' AS `owner`"),
			always(", CAST(NULL AS STRING) AS `comment`"),
			always("FROM " + project + ";"),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: "always odps"},
			{Name: "name", Desc: "the project, from current_project(), as it is in Schemas"},
			{Name: "owner", Desc: "always empty, as it is in Schemas"},
			{Name: "comment", Desc: "always absent, as it is in Schemas"},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Schema, error) {
			var v dbmeta.Schema
			err := rows.Scan(&v.Catalog, &v.Name, &v.Owner, &v.Comment)
			return v, err
		},
	})

	// \dP. A table that is partitioned by the value of a column. COLUMNS marks
	// each partition column, and the order is the order of the columns.
	dbmeta.PartitionedTables.Register(dbmeta.MaxCompute, &dbmeta.Binding[dbmeta.PartitionedTable]{
		Stmt: dbmeta.Stmt{
			always("SELECT c.table_schema AS `schema`"),
			always(", c.table_name AS `name`"),
			always(", t.owner_name AS `owner`"),
			always(", " + tableType + " AS `type`"),
			always(", '' AS `parent`"),
			always(", 'list' AS `strategy`"),
			always(", c.column_name AS `expression`"),
			always(", c.column_comment AS `comment`"),
			always(", CAST(NULL AS STRING) AS `table`"),
			always(", CAST(NULL AS STRING) AS `access_method`"),
			always(", CAST(NULL AS BIGINT) AS `direct_size`"),
			always(", CAST(NULL AS BIGINT) AS `total_size`"),
			always(", CAST(NULL AS BOOLEAN) AS `parent_visible`"),
			always(", CAST(NULL AS BOOLEAN) AS `table_visible`"),
			always("FROM information_schema.columns c JOIN information_schema.tables t"),
			always("ON t.table_catalog = c.table_catalog AND t.table_schema = c.table_schema"),
			always("AND t.table_name = c.table_name"),
			always("WHERE c.is_partition_key"),
			always("AND " + like("c.table_schema", "@schema")),
			always("AND " + like("c.table_name", "@name")),
			last("ORDER BY c.table_schema, c.table_name, c.ordinal_position"),
		},
		Fields: []dbmeta.Field{
			{Name: "schema"}, {Name: "name"},
			{Name: "owner", Desc: "OWNER_NAME"},
			{Name: "type", Desc: "the type of the table, as Tables spells it"},
			{Name: "parent", Desc: "always empty: a partition is a directory of the table and not a table"},
			{Name: "strategy", Desc: "always list: a table is partitioned by the value of a column"},
			{Name: "expression", Desc: "the partition column. A table partitioned by two columns has two rows, in the order of the columns"},
			{Name: "comment", Desc: "the comment of the partition column. Absent when none is set"},
			{Name: "table", Desc: "always absent: MaxCompute has no partitioned index"},
			{Name: "access_method", Desc: "always absent"},
			{Name: "direct_size", Desc: "always absent: the size of a partition is in Partitions and a row here is one column"},
			{Name: "total_size", Desc: "always absent, for the same reason"},
			{Name: "parent_visible", Desc: "always absent: MaxCompute has no search path"},
			{Name: "table_visible", Desc: "always absent, for the same reason"},
		},
		Params: schemaName("partitioned table"),
		Scan: func(rows *sql.Rows) (dbmeta.PartitionedTable, error) {
			var v dbmeta.PartitionedTable
			err := rows.Scan(&v.Schema, &v.Name, dbmeta.NullAsEmpty(&v.Owner), &v.Type, &v.Parent, &v.Strategy,
				&v.Expression, &v.Comment, &v.Table, &v.AccessMethod, &v.DirectSize,
				&v.TotalSize, &v.ParentVisible, &v.TableVisible)
			return v, err
		},
	})

	// The partitions of a table. A partition is named as its columns are, joined by a slash, such
	// as sold_on=2026-01-01/region=north.
	dbmeta.Partitions.Register(dbmeta.MaxCompute, &dbmeta.Binding[dbmeta.Partition]{
		Stmt: dbmeta.Stmt{
			always("SELECT p.table_schema AS `schema`"),
			always(", p.table_name AS `table`"),
			always(", p.table_schema AS `partition_schema`"),
			always(", p.partition_name AS `partition`"),
			always(", 'partition' AS `type`"),
			always(", p.partition_name AS `bound`"),
			always(", CAST(NULL AS STRING) AS `constraint`"),
			always(", FALSE AS `partitioned`"),
			always(", FALSE AS `detach_pending`"),
			always(", CAST(NULL AS BOOLEAN) AS `table_visible`"),
			always(", CAST(NULL AS BOOLEAN) AS `partition_visible`"),
			always("FROM information_schema.partitions p"),
			always("WHERE " + like("p.table_schema", "@schema")),
			always("AND " + like("p.table_name", "@parent")),
			always("AND " + like("p.table_schema", "@partition_schema")),
			always("AND " + like("p.partition_name", "@name")),
			last("ORDER BY p.table_schema, p.table_name, p.partition_name"),
		},
		Fields: []dbmeta.Field{
			{Name: "schema", Desc: "the project of the table"},
			{Name: "table"},
			{Name: "partition_schema", Desc: "the project of the partition, which is the project of the table"},
			{Name: "partition", Desc: "PARTITION_NAME, such as sold_on=2026-01-01/region=north"},
			{Name: "type", Desc: "always partition"},
			{Name: "bound", Desc: "PARTITION_NAME again, because a partition is the value of each partition column and has no range"},
			{Name: "constraint", Desc: "always absent"},
			{Name: "partitioned", Desc: "always false: a partition has no partitions of its own"},
			{Name: "detach_pending", Desc: "always false: MaxCompute has no detach"},
			{Name: "table_visible", Desc: "always absent: MaxCompute has no search path"},
			{Name: "partition_visible", Desc: "always absent, for the same reason"},
		},
		Params: []dbmeta.Param{
			{Name: "schema", Desc: "project name pattern, empty for every project", Default: ""},
			{Name: "parent", Desc: "table name pattern, empty for every table", Default: ""},
			{Name: "partition_schema", Desc: "project name pattern of the partition, empty for every project", Default: ""},
			{Name: "name", Desc: "partition name pattern, empty for every partition", Default: ""},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Partition, error) {
			var v dbmeta.Partition
			err := rows.Scan(&v.Schema, &v.Table, &v.PartitionSchema, &v.Partition, &v.Type, &v.Bound,
				&v.Constraint, &v.Partitioned, &v.DetachPending, &v.TableVisible, &v.PartitionVisible)
			return v, err
		},
	})

	// \dd. The comment of a relation. The comment of a column is in Columns.
	dbmeta.Comments.Register(dbmeta.MaxCompute, &dbmeta.Binding[dbmeta.Comment]{
		Stmt: dbmeta.Stmt{
			always("SELECT t.table_schema AS `schema`"),
			always(", t.table_name AS `name`"),
			always(", " + tableType + " AS `type`"),
			always(", t.table_comment AS `comment`"),
			always("FROM information_schema.tables t"),
			always("WHERE t.table_comment IS NOT NULL"),
			always("AND " + like("t.table_schema", "@schema")),
			always("AND " + like("t.table_name", "@name")),
			last("ORDER BY t.table_schema, t.table_name"),
		},
		Fields: []dbmeta.Field{
			{Name: "schema"},
			{Name: "name"},
			{Name: "type", Desc: "the type of the relation, as Tables spells it"},
			{Name: "comment"},
		},
		Params: schemaName("object"),
		Scan: func(rows *sql.Rows) (dbmeta.Comment, error) {
			var v dbmeta.Comment
			err := rows.Scan(&v.Schema, &v.Name, &v.Type, &v.Comment)
			return v, err
		},
	})

	// \du and \dg. A user and a role are both rows here, as they are in psql. A
	// user can sign in and a role cannot. MaxCompute records no attribute of
	// either, so every flag is false, and a user is a superuser when it holds the
	// role super_administrator.
	dbmeta.Roles.Register(dbmeta.MaxCompute, &dbmeta.Binding[dbmeta.Role]{
		Stmt: dbmeta.Stmt{
			always("SELECT r.name AS `name`"),
			always(", (r.name = 'super_administrator' OR s.user_name IS NOT NULL) AS `superuser`"),
			always(", FALSE AS `create_role`"),
			always(", FALSE AS `create_db`"),
			always(", r.can_login AS `can_login`"),
			always(", FALSE AS `replication`"),
			always(", FALSE AS `bypass_rls`"),
			always(", TRUE AS `inherit`"),
			always(", CAST(-1 AS BIGINT) AS `conn_limit`"),
			always(", CAST(NULL AS STRING) AS `valid_until`"),
			always(", COALESCE(m.roles, '') AS `member_of`"),
			always(", r.comment AS `comment`"),
			always("FROM (SELECT user_name AS name, TRUE AS can_login, CAST(NULL AS STRING) AS comment"),
			always("FROM information_schema.users"),
			always("UNION ALL SELECT role_name, FALSE, comment FROM information_schema.roles) r"),
			always("LEFT JOIN (SELECT user_name, concat_ws(', ', sort_array(collect_list(role_name))) AS roles"),
			always("FROM information_schema.user_roles GROUP BY user_name) m ON m.user_name = r.name"),
			always("LEFT JOIN (SELECT DISTINCT user_name FROM information_schema.user_roles"),
			always("WHERE role_name = 'super_administrator') s ON s.user_name = r.name"),
			always("WHERE " + like("r.name", "@name")),
			last("ORDER BY r.can_login DESC, r.name"),
		},
		Fields: []dbmeta.Field{
			{Name: "name", Desc: "USER_NAME of a user, such as RAM$ and the account and the user, or ROLE_NAME of a role"},
			{Name: "superuser", Desc: "true for the role super_administrator and for a user that holds it"},
			{Name: "create_role", Desc: "always false: MaxCompute records no such attribute, although the role admin can create a role"},
			{Name: "create_db", Desc: "always false: a project is made outside SQL"},
			{Name: "can_login", Desc: "true for a user and false for a role"},
			{Name: "replication", Desc: "always false: MaxCompute has no such attribute"},
			{Name: "bypass_rls", Desc: "always false: MaxCompute has no row level security"},
			{Name: "inherit", Desc: "always true: a user has the privileges of its roles"},
			{Name: "conn_limit", Desc: "always -1, which is no limit: MaxCompute sets none for a user"},
			{Name: "valid_until", Desc: "always absent: a user does not expire here"},
			{Name: "member_of", Desc: "the roles of the user, joined by a comma and a space, from USER_ROLES. Empty for a role and for a user with no role"},
			{Name: "comment", Desc: "COMMENT of a role, which is the empty text for a role that has none and reserved by the system for admin and super_administrator. Always absent for a user"},
		},
		Params: nameOnly("role"),
		Scan: func(rows *sql.Rows) (dbmeta.Role, error) {
			var v dbmeta.Role
			err := rows.Scan(&v.Name, &v.Superuser, &v.CreateRole, &v.CreateDB, &v.CanLogin,
				&v.Replication, &v.BypassRLS, &v.Inherit, &v.ConnLimit, &v.ValidUntil,
				&v.MemberOf, &v.Comment)
			return v, err
		},
	})

	// \drg.
	dbmeta.RoleGrants.Register(dbmeta.MaxCompute, &dbmeta.Binding[dbmeta.RoleGrant]{
		Stmt: dbmeta.Stmt{
			always("SELECT ur.user_name AS `role`"),
			always(", ur.role_name AS `member_of`"),
			always(", CAST(NULL AS STRING) AS `grantor`"),
			always(", FALSE AS `admin`"),
			always(", TRUE AS `inherit`"),
			always(", TRUE AS `set`"),
			always("FROM information_schema.user_roles ur"),
			always("WHERE " + like("ur.user_name", "@name")),
			last("ORDER BY ur.user_name, ur.role_name"),
		},
		Fields: []dbmeta.Field{
			{Name: "role", Desc: "the user, from USER_NAME. A role cannot be a member of a role here"},
			{Name: "member_of", Desc: "the role, from ROLE_NAME"},
			{Name: "grantor", Desc: "always absent: USER_ROLES does not say who granted the role"},
			{Name: "admin", Desc: "always false: no view records an admin option"},
			{Name: "inherit", Desc: "always true: a user has the privileges of its roles"},
			{Name: "set", Desc: "always true: a user acts with every role that it holds"},
		},
		Params: nameOnly("grantee"),
		Scan: func(rows *sql.Rows) (dbmeta.RoleGrant, error) {
			var v dbmeta.RoleGrant
			err := rows.Scan(&v.Role, &v.MemberOf, &v.Grantor, &v.Admin, &v.Inherit, &v.Set)
			return v, err
		},
	})

	// \dp. The privileges on a relation, a line for each privilege of each user.
	// TABLE_PRIVILEGES names a user and never a role.
	dbmeta.Privileges.Register(dbmeta.MaxCompute, &dbmeta.Binding[dbmeta.Privilege]{
		Stmt: dbmeta.Stmt{
			always("SELECT t.table_schema AS `schema`"),
			always(", t.table_name AS `name`"),
			always(", " + tableType + " AS `type`"),
			always(", a.acl AS `access`"),
			always(", CAST(NULL AS STRING) AS `column_access`"),
			always(", CAST(NULL AS STRING) AS `policies`"),
			always("FROM information_schema.tables t"),
			always("LEFT JOIN " + grants("table_privileges", "table_catalog", "table_schema", "table_name") + " a"),
			always("ON a.table_catalog = t.table_catalog AND a.table_schema = t.table_schema AND a.table_name = t.table_name"),
			always("WHERE " + like("t.table_schema", "@schema")),
			always("AND " + like("t.table_name", "@name")),
			last("ORDER BY t.table_schema, t.table_name"),
		},
		Fields: []dbmeta.Field{
			{Name: "schema", Desc: "the project"}, {Name: "name"},
			{Name: "type", Desc: "the type of the relation, as Tables spells it"},
			{Name: "access", Desc: "the privileges on the relation, a line for each privilege of each user, as user=privilege, such as select or describe. Absent when no user holds one"},
			{Name: "column_access", Desc: "always absent: ColumnPrivileges reads the grants on a column"},
			{Name: "policies", Desc: "always absent: MaxCompute has no row level security"},
		},
		Params: schemaName("relation"),
		Scan: func(rows *sql.Rows) (dbmeta.Privilege, error) {
			var v dbmeta.Privilege
			err := rows.Scan(&v.Schema, &v.Name, &v.Type, &v.Access, &v.ColumnAccess, &v.Policies)
			return v, err
		},
	})

	// The grants on a column, a row for each user and privilege.
	dbmeta.ColumnPrivileges.Register(dbmeta.MaxCompute, &dbmeta.Binding[dbmeta.ColumnPrivilege]{
		Stmt: dbmeta.Stmt{
			always("SELECT p.table_schema AS `schema`"),
			always(", p.table_name AS `table`"),
			always(", p.column_name AS `column`"),
			always(", CAST(row_number() OVER (PARTITION BY p.table_schema, p.table_name, p.column_name"),
			always("ORDER BY p.grantee, p.privilege_type) AS BIGINT) AS `ordinal`"),
			always(", p.grantee || '=' || p.privilege_type || COALESCE('/' || p.grantor, '') AS `access`"),
			always(", p.grantee AS `grantee`"),
			always(", p.grantor AS `grantor`"),
			always(", p.privilege_type AS `privileges`"),
			always("FROM information_schema.column_privileges p"),
			always("WHERE " + like("p.table_schema", "@schema")),
			always("AND " + like("p.table_name", "@parent")),
			always("AND " + like("p.column_name", "@name")),
			last("ORDER BY p.table_schema, p.table_name, p.column_name, `ordinal`"),
		},
		Fields: []dbmeta.Field{
			{Name: "schema", Desc: "the project"}, {Name: "table"}, {Name: "column"},
			{Name: "ordinal", Desc: "the place of the entry among the entries of the column, in the order of the user and the privilege"},
			{Name: "access", Desc: "user=privilege, with a slash and the grantor when there is one"},
			{Name: "grantee", Desc: "GRANTEE, a user"},
			{Name: "grantor", Desc: "GRANTOR. Absent in every row of the other privilege views that was seen"},
			{Name: "privileges", Desc: "PRIVILEGE_TYPE, such as select"},
		},
		Params: schemaParentName("table", "column"),
		Scan: func(rows *sql.Rows) (dbmeta.ColumnPrivilege, error) {
			var v dbmeta.ColumnPrivilege
			err := rows.Scan(&v.Schema, &v.Table, &v.Column, &v.Ordinal, &v.Access,
				&v.Grantee, &v.Grantor, &v.Privileges)
			return v, err
		},
	})

	// \df. A function that the project holds, which is a user defined function. The
	// built in functions are in no view.
	dbmeta.Functions.Register(dbmeta.MaxCompute, &dbmeta.Binding[dbmeta.Function]{
		Stmt: dbmeta.Stmt{
			always("SELECT f.udf_catalog AS `catalog`"),
			always(", f.udf_schema AS `schema`"),
			always(", f.udf_name AS `name`"),
			always(", CAST(NULL AS STRING) AS `id`"),
			always(", 'function' AS `kind`"),
			always(", CAST(NULL AS STRING) AS `result_type`"),
			always(", CAST(NULL AS STRING) AS `arg_types`"),
			always(", '' AS `volatility`"),
			always(", '' AS `parallel`"),
			always(", f.owner_name AS `owner`"),
			always(", '' AS `security`"),
			always(", a.acl AS `access`"),
			always(", '' AS `language`"),
			always(", CAST(NULL AS STRING) AS `source`"),
			always(", CAST(NULL AS STRING) AS `comment`"),
			always(", CAST(NULL AS STRING) AS `definition`"),
			always(", FALSE AS `leakproof`"),
			always(", CAST(NULL AS STRING) AS `prosrc`"),
			always("FROM information_schema.udfs f"),
			always("LEFT JOIN " + grants("udf_privileges", "udf_catalog", "udf_schema", "udf_name") + " a"),
			always("ON a.udf_catalog = f.udf_catalog AND a.udf_schema = f.udf_schema AND a.udf_name = f.udf_name"),
			always("WHERE " + like("f.udf_schema", "@schema")),
			always("AND " + like("f.udf_name", "@name")),
			last("ORDER BY f.udf_schema, f.udf_name"),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: "always odps"}, {Name: "schema", Desc: "the project"}, {Name: "name"},
			{Name: "id", Desc: "always absent: MaxCompute does not overload a function by name here"},
			{Name: "kind", Desc: "always function: UDFS lists no procedure and no aggregate"},
			{Name: "result_type", Desc: "always absent: UDFS holds no type"},
			{Name: "arg_types", Desc: "always absent: UDFS holds no parameter"},
			{Name: "volatility", Desc: "always empty: UDFS holds no such property"},
			{Name: "parallel", Desc: "always empty: MaxCompute has no such property"},
			{Name: "owner", Desc: "OWNER_NAME"},
			{Name: "security", Desc: "always empty: MaxCompute has no such property"},
			{Name: "access", Desc: "the privileges on the function, a line for each privilege of each user, as user=privilege, such as execute. Absent when no user holds one"},
			{Name: "language", Desc: "always empty: UDFS holds no language"},
			{Name: "source", Desc: "always absent: UDFS holds no body"},
			{Name: "comment", Desc: "always absent: UDFS holds no comment"},
			{Name: "definition", Desc: "always absent: UDFS holds no definition"},
			{Name: "leakproof", Desc: "always false: MaxCompute has no such property"},
			{Name: "prosrc", Desc: "always absent: MaxCompute has no such column"},
		},
		Params: schemaName("function"),
		Scan: func(rows *sql.Rows) (dbmeta.Function, error) {
			var v dbmeta.Function
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Name, &v.ID, &v.Kind, &v.ResultType,
				&v.ArgTypes, &v.Volatility, &v.Parallel, &v.Owner, &v.Security, &v.Access,
				&v.Language, &v.Source, &v.Comment, &v.Definition, &v.Leakproof, &v.Prosrc)
			return v, err
		},
	})
}
