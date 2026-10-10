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
			fixed(", ", `(boolean)false`, "role", "create_role"),
			fixed(", ", `(boolean)false`, "role", "create_db"),
			always(`, can_login`),
			fixed(", ", `(boolean)false`, "role", "replication"),
			fixed(", ", `(boolean)false`, "role", "bypass_rls"),
			fixed(", ", `(boolean)true`, "role", "inherit"),
			fixed(", ", `(bigint)-1`, "role", "conn_limit"),
			fixed(", ", `(text)NULL`, "role", "valid_until"),
			always(`, member_of`),
			fixed(", ", `(text)NULL`, "role", "comment"),
			{{Query: `FROM system_auth.roles`}, scylla(`FROM system.roles`)},
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
		Params: clusterFilters("role"),
		Keep:   keep(nil, nil, func(v dbmeta.Role) string { return v.Name }),
		Scan: func(rows *sql.Rows) (dbmeta.Role, error) {
			var (
				v        dbmeta.Role
				memberOf any
			)
			err := rows.Scan(&v.Name, &v.Superuser, pad{}, pad{},
				&v.CanLogin, pad{}, pad{}, pad{}, pad{},
				pad{}, &memberOf, pad{})
			v.Inherit = true
			v.ConnLimit = -1
			v.MemberOf = textList(memberOf)
			return v, err
		},
	})

	// \drds, on ScyllaDB alone. A role's attribute is a value set on the
	// role itself, and ATTACH SERVICE LEVEL is what sets one: it gives the
	// role's sessions the timeout and the share of the server that the
	// service level names. That is the ScyllaDB form of ALTER ROLE ... SET,
	// and D91 records why it is a fair analogue.
	//
	// Cassandra has no such table, so every fragment names the ScyllaDB key
	// and Cassandra reports that it cannot answer.
	//
	// The row is per attribute rather than per role, because CQL cannot
	// group. There are three columns for three fields, and settings needs
	// both the attribute's name and its value, so the name is selected under
	// "database" and Scan folds the two into settings. A ScyllaDB role
	// belongs to the cluster rather than to a keyspace, so database is
	// always empty.
	dbmeta.RoleSettings.Register(dbmeta.Cassandra, &dbmeta.Binding[dbmeta.RoleSetting]{
		Stmt: dbmeta.Stmt{
			{scylla(`SELECT role`)},
			{scylla(`, name AS "database"`)},
			{scylla(`, value AS "settings"`)},
			{scylla(`FROM system.role_attributes`)},
		},
		Fields: []dbmeta.Field{
			{Name: "role"},
			{
				Name: "database",
				Desc: "always empty: a role belongs to the cluster, and an" +
					" attribute applies wherever the role connects",
			},
			{
				Name: "settings",
				Desc: "one attribute as name=value, such as" +
					" service_level=reporting. A role with two has two rows",
			},
		},
		Params: clusterFilters("role"),
		Keep:   keep(nil, nil, func(v dbmeta.RoleSetting) string { return v.Role.V }),
		Scan: func(rows *sql.Rows) (dbmeta.RoleSetting, error) {
			var (
				v           dbmeta.RoleSetting
				name, value string
			)
			err := rows.Scan(&v.Role, &name, &value)
			v.Settings = sql.Null[string]{V: name + "=" + value, Valid: true}
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
			fixed(", ", `(text)NULL`, "role", "grantor"),
			fixed(", ", `(boolean)false`, "role", "admin"),
			fixed(", ", `(boolean)true`, "role", "inherit"),
			fixed(", ", `(boolean)true`, "role", "set"),
			{{Query: `FROM system_auth.role_members`}, scylla(`FROM system.role_members`)},
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
		Params: clusterFilters("role"),
		Keep:   keep(nil, nil, func(v dbmeta.RoleGrant) string { return v.Role }),
		Scan: func(rows *sql.Rows) (dbmeta.RoleGrant, error) {
			var v dbmeta.RoleGrant
			err := rows.Scan(&v.Role, &v.MemberOf, pad{}, pad{}, pad{}, pad{})
			v.Inherit = true
			v.Set = true
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
			fixed(", ", `(text)NULL`, "role", "column_access"),
			fixed(", ", `(text)NULL`, "role", "policies"),
			{{Query: `FROM system_auth.role_permissions`}, scylla(`FROM system.role_permissions`)},
		},
		Fields: []dbmeta.Field{
			{Name: "schema", Desc: "the keyspace named in the resource path, absent for a grant above one"},
			{Name: "name", Desc: "the table named in the resource path, or the resource itself"},
			{Name: "type", Desc: "the first part of the resource path, such as data, roles or functions"},
			{Name: "access", Desc: "the permissions granted, as one text"},
			{
				Name: "column_access",
				Desc: "always absent: Cassandra grants on a table and not on a column",
			},
			{
				Name: "policies",
				Desc: "always absent: Cassandra has no row level security policy",
			},
		},
		Params: filters("resource"),
		Keep:   keep(func(v dbmeta.Privilege) string { return v.Schema.V }, nil, func(v dbmeta.Privilege) string { return v.Name }),
		Scan: func(rows *sql.Rows) (dbmeta.Privilege, error) {
			// The resource is selected three times, once for each field it
			// feeds, so that the query returns as many columns as it
			// declares fields. One copy is taken apart and the other two are
			// only there to be consumed.
			var (
				v                             dbmeta.Privilege
				resource, spare1, spare2, acc any
			)
			err := rows.Scan(&resource, &spare1, &spare2, &acc, pad{}, pad{})
			var schema string
			schema, v.Name, v.Type = splitResource(toText(resource))
			// A grant above a keyspace names none.
			v.Schema = sql.Null[string]{V: schema, Valid: schema != ""}
			if s := textList(acc); s != "" {
				v.Access = sql.Null[string]{V: s, Valid: true}
			}
			return v, err
		},
	})

	// The server configuration, which arrived as a virtual table in 4.0.
	// Every fragment gates on it, so an older release reports that the server
	// is too old rather than a wrong answer.
	//
	// ScyllaDB keeps it in system.config, which also records each setting's
	// type and where its value came from. Its release_version reads 3.0.8,
	// which is below the gate, so the ScyllaDB fragment is what answers
	// there.
	dbmeta.Settings.Register(dbmeta.Cassandra, &dbmeta.Binding[dbmeta.Setting]{
		Stmt: dbmeta.Stmt{
			{{Min: v40, Query: `SELECT name`}, scylla(`SELECT name`)},
			{{Min: v40, Query: `, value`}, scylla(`, value`)},
			{{Min: v40, Query: `, (text)NULL AS "type"`}, scylla(`, type`)},
			{{Min: v40, Query: `, (text)NULL AS "context"`}, scylla(`, name AS "context"`)},
			{{Min: v40, Query: `, (text)NULL AS "access"`}, scylla(`, name AS "access"`)},
			{{Min: v40, Query: `, (text)NULL AS "display"`}, scylla(`, name AS "display"`)},
			{{Min: v40, Query: `FROM system_views.settings`}, scylla(`FROM system.config`)},
		},
		Fields: []dbmeta.Field{
			{Name: "name"},
			{
				Name: "value",
				Desc: "the current value. ScyllaDB writes it as JSON, so a text" +
					" value arrives in double quotes",
			},
			{
				Name: "type", Key: Scylla,
				Desc: "the type ScyllaDB records, such as integer. Cassandra's" +
					" virtual table records none",
			},
			{
				Name: "context",
				Desc: "always absent: neither product records when a setting" +
					" can be changed",
			},
			{
				Name: "access",
				Desc: "always absent: whether a setting can be changed at runtime" +
					" is not in the table",
			},
			{Name: "display", Desc: "always absent: neither product shows a value in one form, which is value"},
		},
		Params: clusterFilters("setting"),
		Keep:   keep(nil, nil, func(v dbmeta.Setting) string { return v.Name }),
		Scan: func(rows *sql.Rows) (dbmeta.Setting, error) {
			// The type is scanned into the field rather than into pad,
			// because on ScyllaDB it is a real column. On Cassandra it is
			// the padded NULL, which github.com/xo/cassandra reports as one, so
			// the field is absent there by itself.
			var v dbmeta.Setting
			err := rows.Scan(&v.Name, &v.Value, &v.Type, pad{}, pad{}, pad{})
			return v, err
		},
	})

	// \df.
	dbmeta.Functions.Register(dbmeta.Cassandra, &dbmeta.Binding[dbmeta.Function]{
		Stmt: dbmeta.Stmt{
			fixed("SELECT ", `(text)''`, "keyspace_name", "catalog"),
			always(`, keyspace_name AS "schema"`),
			always(`, function_name AS "name"`),
			fixed(", ", `(text)NULL`, "keyspace_name", "id"),
			fixed(", ", `(text)'function'`, "keyspace_name", "kind"),
			always(`, return_type AS "result_type"`),
			always(`, argument_types AS "arg_types"`),
			fixed(", ", `(text)''`, "keyspace_name", "volatility"),
			fixed(", ", `(text)''`, "keyspace_name", "parallel"),
			fixed(", ", `(text)NULL`, "keyspace_name", "owner"),
			fixed(", ", `(text)''`, "keyspace_name", "security"),
			fixed(", ", `(text)NULL`, "keyspace_name", "access"),
			always(`, language`),
			always(`, body AS "source"`),
			fixed(", ", `(text)NULL`, "keyspace_name", "comment"),
			fixed(", ", `(text)NULL`, "keyspace_name", "definition"),
			always(`, body AS "prosrc"`),
			always(`FROM system_schema.functions`),
		},
		Fields: routineFields("function"),
		Params: filters("function"),
		Keep:   keep(func(v dbmeta.Function) string { return v.Schema }, nil, func(v dbmeta.Function) string { return v.Name }),
		Scan:   scanRoutine("function"),
	})

	// \da. An aggregate has no body of its own: it names a state function
	// and a final function, which are ordinary functions.
	dbmeta.Aggregates.Register(dbmeta.Cassandra, &dbmeta.Binding[dbmeta.Function]{
		Stmt: dbmeta.Stmt{
			fixed("SELECT ", `(text)''`, "keyspace_name", "catalog"),
			always(`, keyspace_name AS "schema"`),
			always(`, aggregate_name AS "name"`),
			fixed(", ", `(text)NULL`, "keyspace_name", "id"),
			fixed(", ", `(text)'aggregate'`, "keyspace_name", "kind"),
			always(`, return_type AS "result_type"`),
			always(`, argument_types AS "arg_types"`),
			fixed(", ", `(text)''`, "keyspace_name", "volatility"),
			fixed(", ", `(text)''`, "keyspace_name", "parallel"),
			fixed(", ", `(text)NULL`, "keyspace_name", "owner"),
			fixed(", ", `(text)''`, "keyspace_name", "security"),
			fixed(", ", `(text)NULL`, "keyspace_name", "access"),
			always(`, state_func AS "language"`),
			always(`, final_func AS "source"`),
			fixed(", ", `(text)NULL`, "keyspace_name", "comment"),
			fixed(", ", `(text)NULL`, "keyspace_name", "definition"),
			fixed(", ", `(text)NULL`, "keyspace_name", "prosrc"),
			always(`FROM system_schema.aggregates`),
		},
		Fields: aggregateFields(),
		Params: filters("aggregate"),
		Keep:   keep(func(v dbmeta.Function) string { return v.Schema }, nil, func(v dbmeta.Function) string { return v.Name }),
		Scan:   scanRoutine("aggregate"),
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
		{Name: "owner", Desc: "always absent: a " + kind + " has no owner in the catalog"},
		{Name: "security", Desc: "always empty: CQL has no SECURITY DEFINER"},
		{Name: "access", Desc: "always absent: a grant is on the functions resource"},
		{Name: "language"},
		{Name: "source"},
		{Name: "comment", Desc: "always absent: a " + kind + " carries no comment"},
		{
			Name: "definition",
			Desc: "always absent: the catalog keeps the parts of a " + kind +
				" and DESCRIBE builds the statement",
		},
		{
			Name: "prosrc",
			Desc: "the body, which is the same text as source. It is absent for an aggregate, which has no body",
		},
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

// scanRoutine returns the Scan for one of the two routine queries. kind is
// what the query selects as its kind, which Scan sets itself.
func scanRoutine(kind string) func(*sql.Rows) (dbmeta.Function, error) {
	return func(rows *sql.Rows) (dbmeta.Function, error) {
		var (
			v    dbmeta.Function
			args any
		)
		// An aggregate has no body, and ScyllaDB selects a stand in column
		// for the absent value, so the last column is read only for a
		// function.
		var prosrc any = pad{}
		if kind == "function" {
			prosrc = &v.Prosrc
		}
		err := rows.Scan(pad{}, &v.Schema, &v.Name, pad{}, pad{}, &v.ResultType,
			&args, pad{}, pad{}, pad{}, pad{}, pad{},
			&v.Language, &v.Source, pad{}, pad{}, prosrc)
		v.Kind = kind
		v.ArgTypes = sql.Null[string]{V: textList(args), Valid: true}
		return v, err
	}
}
