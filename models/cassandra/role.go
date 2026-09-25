package cassandra

import (
	"database/sql"
	"strings"

	"github.com/xo/dbmeta"
)

func registerRoles() {
	// \du. A Cassandra role is a user and a group at once, which is why
	// CREATE USER is defined in terms of it.
	dbmeta.Roles.Register(dbmeta.Cassandra, &dbmeta.Binding[dbmeta.Role]{
		Stmt: dbmeta.Stmt{
			always(`SELECT role AS "name"`),
			always(`, is_superuser AS "superuser"`),
			always(`, (boolean)false AS "create_role"`),
			always(`, (boolean)false AS "create_db"`),
			always(`, can_login`),
			always(`, (boolean)false AS "replication"`),
			always(`, (boolean)false AS "bypass_rls"`),
			always(`, (boolean)true AS "inherit"`),
			always(`, (bigint)-1 AS "conn_limit"`),
			always(`, (text)NULL AS "valid_until"`),
			always(`, member_of`),
			always(`, (text)NULL AS "comment"`),
			always(`FROM system_auth.roles`),
		},
		Fields: []dbmeta.Field{
			{Name: "name"}, {Name: "superuser"},
			{
				Name: "create_role",
				Desc: "always false: Cassandra grants CREATE on the roles" +
					" resource rather than flagging the role, and the privileges" +
					" query returns that",
			},
			{Name: "create_db", Desc: "always false: Cassandra has no database to create"},
			{Name: "can_login"},
			{Name: "replication", Desc: "always false: replication is a keyspace setting here"},
			{Name: "bypass_rls", Desc: "always false: Cassandra has no row level security"},
			{
				Name: "inherit",
				Desc: "always true: a Cassandra role always has the permissions" +
					" of the roles granted to it, and there is no NOINHERIT",
			},
			{Name: "conn_limit", Desc: "always -1: Cassandra sets no per role connection limit"},
			{Name: "valid_until", Desc: "always absent: a password here does not expire"},
			{Name: "member_of", Desc: "the roles granted to this one, as one text"},
			{Name: "comment", Desc: "always absent: a role carries no comment"},
		},
		Params: filters("role"),
		Scan: func(rows *sql.Rows) (dbmeta.Role, error) {
			var (
				v        dbmeta.Role
				memberOf any
			)
			err := rows.Scan(&v.Name, &v.Superuser, &v.CreateRole, &v.CreateDB,
				&v.CanLogin, &v.Replication, &v.BypassRLS, &v.Inherit, &v.ConnLimit,
				pad{}, &memberOf, pad{})
			v.MemberOf = textList(memberOf)
			return v, err
		},
	})

	// One row per role granted to another.
	//
	// system_auth.role_members is keyed the other way round from the field
	// names here: its role column is the role being granted and its member
	// column is who now has it. Scan does not swap them, the statement does,
	// by naming them as this query means them.
	dbmeta.RoleGrants.Register(dbmeta.Cassandra, &dbmeta.Binding[dbmeta.RoleGrant]{
		Stmt: dbmeta.Stmt{
			always(`SELECT member AS "role"`),
			always(`, role AS "member_of"`),
			always(`, (text)NULL AS "grantor"`),
			always(`, (boolean)false AS "admin"`),
			always(`, (boolean)true AS "inherit"`),
			always(`, (boolean)true AS "set"`),
			always(`FROM system_auth.role_members`),
		},
		Fields: []dbmeta.Field{
			{Name: "role", Desc: "the role that was granted something"},
			{Name: "member_of", Desc: "the role it was granted"},
			{Name: "grantor", Desc: "always absent: Cassandra records no grantor"},
			{
				Name: "admin",
				Desc: "always false: CQL has no WITH ADMIN OPTION, and AUTHORIZE" +
					" on the roles resource is the nearest thing",
			},
			{Name: "inherit", Desc: "always true: a granted role is always inherited"},
			{Name: "set", Desc: "always true: there is no SET ROLE to withhold"},
		},
		Params: filters("role"),
		Scan: func(rows *sql.Rows) (dbmeta.RoleGrant, error) {
			var v dbmeta.RoleGrant
			err := rows.Scan(&v.Role, &v.MemberOf, pad{}, &v.Admin,
				&v.Inherit, &v.Set)
			return v, err
		},
	})

	// \dp. A grant in Cassandra names a resource as a path, such as
	// data/myspace/mytable, and Scan takes it apart because CQL cannot.
	dbmeta.Privileges.Register(dbmeta.Cassandra, &dbmeta.Binding[dbmeta.Privilege]{
		Stmt: dbmeta.Stmt{
			always(`SELECT resource AS "schema"`),
			always(`, resource AS "name"`),
			always(`, resource AS "type"`),
			always(`, permissions AS "access"`),
			always(`, (text)'' AS "column_access"`),
			always(`, (text)'' AS "policies"`),
			always(`FROM system_auth.role_permissions`),
		},
		Fields: []dbmeta.Field{
			{Name: "schema", Desc: "the keyspace named in the resource path, empty for a grant above one"},
			{Name: "name", Desc: "the table named in the resource path, or the resource itself"},
			{Name: "type", Desc: "the first part of the resource path, such as data, roles or functions"},
			{Name: "access", Desc: "the permissions granted, as one text"},
			{
				Name: "column_access",
				Desc: "always empty: Cassandra grants on a table and not on a column",
			},
			{
				Name: "policies",
				Desc: "always empty: Cassandra has no row level security policy",
			},
		},
		Params: filters("resource"),
		Scan: func(rows *sql.Rows) (dbmeta.Privilege, error) {
			// The resource is selected three times, once for each field it
			// feeds, so that the query returns as many columns as it
			// declares fields. One copy is taken apart and the other two are
			// only there to be consumed.
			var (
				v                             dbmeta.Privilege
				resource, spare1, spare2, acc any
			)
			err := rows.Scan(&resource, &spare1, &spare2, &acc,
				&v.ColumnAccess, &v.Policies)
			v.Schema, v.Name, v.Type = splitResource(toText(resource))
			if s := textList(acc); s != "" {
				v.Access = sql.Null[string]{V: s, Valid: true}
			}
			return v, err
		},
	})

	// The server configuration, which arrived as a virtual table in 4.0.
	// Every fragment gates on it, so an older release reports that the server
	// is too old rather than a wrong answer.
	dbmeta.Settings.Register(dbmeta.Cassandra, &dbmeta.Binding[dbmeta.Setting]{
		Stmt: dbmeta.Stmt{
			{{Min: v40, Query: `SELECT name`}},
			{{Min: v40, Query: `, value`}},
			{{Min: v40, Query: `, (text)NULL AS "type"`}},
			{{Min: v40, Query: `, (text)NULL AS "context"`}},
			{{Min: v40, Query: `, (text)NULL AS "access"`}},
			{{Min: v40, Query: `FROM system_views.settings`}},
		},
		Fields: []dbmeta.Field{
			{Name: "name"},
			{Name: "value"},
			{Name: "type", Desc: "always absent: the virtual table records no type"},
			{Name: "context", Desc: "always absent: it records no context either"},
			{
				Name: "access",
				Desc: "always absent: whether a setting can be changed at runtime" +
					" is not in the table",
			},
		},
		Params: filters("setting"),
		Scan: func(rows *sql.Rows) (dbmeta.Setting, error) {
			var v dbmeta.Setting
			err := rows.Scan(&v.Name, &v.Value, pad{}, pad{}, pad{})
			return v, err
		},
	})

	// \df.
	dbmeta.Functions.Register(dbmeta.Cassandra, &dbmeta.Binding[dbmeta.Function]{
		Stmt: dbmeta.Stmt{
			always(`SELECT (text)'' AS "catalog"`),
			always(`, keyspace_name AS "schema"`),
			always(`, function_name AS "name"`),
			always(`, (text)NULL AS "id"`),
			always(`, (text)'function' AS "kind"`),
			always(`, return_type AS "result_type"`),
			always(`, argument_types AS "arg_types"`),
			always(`, (text)'' AS "volatility"`),
			always(`, (text)'' AS "parallel"`),
			always(`, (text)'' AS "owner"`),
			always(`, (text)'' AS "security"`),
			always(`, (text)NULL AS "access"`),
			always(`, language`),
			always(`, body AS "source"`),
			always(`, (text)NULL AS "comment"`),
			always(`FROM system_schema.functions`),
		},
		Fields: routineFields("function"),
		Params: filters("function"),
		Scan:   scanRoutine,
	})

	// \da. An aggregate has no body of its own: it names a state function
	// and a final function, which are ordinary functions.
	dbmeta.Aggregates.Register(dbmeta.Cassandra, &dbmeta.Binding[dbmeta.Function]{
		Stmt: dbmeta.Stmt{
			always(`SELECT (text)'' AS "catalog"`),
			always(`, keyspace_name AS "schema"`),
			always(`, aggregate_name AS "name"`),
			always(`, (text)NULL AS "id"`),
			always(`, (text)'aggregate' AS "kind"`),
			always(`, return_type AS "result_type"`),
			always(`, argument_types AS "arg_types"`),
			always(`, (text)'' AS "volatility"`),
			always(`, (text)'' AS "parallel"`),
			always(`, (text)'' AS "owner"`),
			always(`, (text)'' AS "security"`),
			always(`, (text)NULL AS "access"`),
			always(`, state_func AS "language"`),
			always(`, final_func AS "source"`),
			always(`, (text)NULL AS "comment"`),
			always(`FROM system_schema.aggregates`),
		},
		Fields: aggregateFields(),
		Params: filters("aggregate"),
		Scan:   scanRoutine,
	})
}

// splitResource takes a Cassandra permission resource apart.
//
// A resource is a path: data means every keyspace, data/myspace means one,
// and data/myspace/mytable means one table. Above data there are roles,
// functions and mbeans, which name no keyspace at all.
func splitResource(resource string) (schema, name, kind string) {
	parts := strings.Split(resource, "/")
	kind = parts[0]
	switch len(parts) {
	case 1:
		return "", resource, kind
	case 2:
		return parts[1], "", kind
	}
	return parts[1], parts[2], kind
}

// routineFields describes what both routine queries return.
func routineFields(kind string) []dbmeta.Field {
	return []dbmeta.Field{
		{Name: "catalog", Desc: "always empty: Cassandra has nothing above a keyspace"},
		{Name: "schema"}, {Name: "name"},
		{Name: "id", Desc: "always absent: a " + kind + " has no id in the catalog"},
		{Name: "kind"},
		{Name: "result_type"},
		{Name: "arg_types", Desc: "the argument types, in order, as one text"},
		{
			Name: "volatility",
			Desc: "always empty: Cassandra records whether a function is called" +
				" on null input and nothing about volatility",
		},
		{Name: "parallel", Desc: "always empty: Cassandra has no parallel safety mark"},
		{Name: "owner", Desc: "always empty: a " + kind + " has no owner in the catalog"},
		{Name: "security", Desc: "always empty: CQL has no SECURITY DEFINER"},
		{Name: "access", Desc: "always absent: a grant is on the functions resource"},
		{Name: "language"},
		{Name: "source"},
		{Name: "comment", Desc: "always absent: a " + kind + " carries no comment"},
	}
}

// aggregateFields is routineFields with the two columns an aggregate spends
// differently named for what they hold.
func aggregateFields() []dbmeta.Field {
	out := routineFields("aggregate")
	for i := range out {
		switch out[i].Name {
		case "language":
			out[i].Desc = "the state function, which is what an aggregate runs" +
				" per row: an aggregate has no language of its own"
		case "source":
			out[i].Desc = "the final function, which an aggregate runs once at" +
				" the end: there is no body to return"
		}
	}
	return out
}

// scanRoutine reads one row of either routine query.
func scanRoutine(rows *sql.Rows) (dbmeta.Function, error) {
	var (
		v    dbmeta.Function
		args any
	)
	err := rows.Scan(&v.Catalog, &v.Schema, &v.Name, pad{}, &v.Kind, &v.ResultType,
		&args, &v.Volatility, &v.Parallel, &v.Owner, &v.Security, pad{},
		&v.Language, &v.Source, pad{})
	v.ArgTypes = textList(args)
	return v, err
}
