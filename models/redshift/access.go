package redshift

import (
	"database/sql"

	"github.com/xo/dbmeta"
)

// newline is the separator of a list of grants. Redshift has no E'\n' string.
const newline = `chr(10)`

// registerAccess registers the kinds that D204 added. They read the grants,
// the roles of a user, the languages, the settings and the columns of a key.
//
// Redshift keeps a grant in two places. The ACL of pg_class, pg_namespace and
// the others shows a grant to a user or a group to every user, and it never
// shows a grant to a role. The SVV views show a grant to a role, and they show
// each user only the rows that concern it. The kinds below read the ACL where
// one exists and the SVV views where the ACL has no answer.
func registerAccess() {
	dbmeta.Privileges.Register(dbmeta.Redshift, &dbmeta.Binding[dbmeta.Privilege]{
		Stmt: dbmeta.Stmt{
			always(`SELECT n.nspname AS "schema"`),
			always(`, c.relname AS "name"`),
			always(`, ` + relationType + ` AS "type"`),
			always(`, array_to_string(c.relacl, ` + newline + `) AS "access"`),
			always(`, NULL AS "column_access"`),
			always(`, NULL AS "policies"`),
			always(`FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace`),
			always(`WHERE c.relkind IN ('r', 'v')`),
			always(`AND ` + notSystem("n.nspname")),
			always(`AND ` + like("n.nspname", "@schema")),
			always(`AND ` + like("c.relname", "@name")),
			always(`ORDER BY 1, 2`),
		},
		Fields: []dbmeta.Field{
			{Name: "schema"}, {Name: "name"}, {Name: "type", Desc: "table or view"},
			{Name: "access", Desc: "the ACL of the relation, one entry for each line. An entry for a group starts with the word group. A grant to a role is not in the ACL, so it is not here"},
			{Name: "column_access", Desc: "always absent: pg_attribute has no ACL, and ColumnPrivileges answers it"},
			{Name: "policies", Desc: "always absent: the policies of row level security are in SVV_RLS_POLICY, which is not read"},
		},
		Params: schemaNameSystem("relation"),
		Scan: func(rows *sql.Rows) (dbmeta.Privilege, error) {
			var v dbmeta.Privilege
			err := rows.Scan(&v.Schema, &v.Name, &v.Type, &v.Access, &v.ColumnAccess, &v.Policies)
			return v, err
		},
	})

	// SVV_COLUMN_PRIVILEGES shows a user only the grants that concern it, so
	// a user that holds no column grant reads no row.
	dbmeta.ColumnPrivileges.Register(dbmeta.Redshift, &dbmeta.Binding[dbmeta.ColumnPrivilege]{
		Stmt: dbmeta.Stmt{
			always(`SELECT n.nspname AS "schema"`),
			always(`, c.relname AS "table"`),
			always(`, a.attname AS "column"`),
			always(`, a.attnum AS "ordinal"`),
			always(`, CASE p.identity_type WHEN 'user' THEN p.identity_name` +
				` WHEN 'public' THEN '' ELSE p.identity_type || ' ' || p.identity_name END` +
				` || '=' || p.privilege_type AS "access"`),
			always(`, CASE WHEN p.identity_type = 'public' THEN NULL ELSE p.identity_name END AS "grantee"`),
			always(`, NULL AS "grantor"`),
			always(`, p.privilege_type AS "privileges"`),
			always(`FROM svv_column_privileges p`),
			always(`JOIN pg_namespace n ON n.nspname = p.namespace_name`),
			always(`JOIN pg_class c ON c.relnamespace = n.oid AND c.relname = p.relation_name`),
			always(`JOIN pg_attribute a ON a.attrelid = c.oid AND a.attname = p.column_name`),
			always(`WHERE ` + notSystem("n.nspname")),
			always(`AND ` + like("n.nspname", "@schema")),
			always(`AND ` + like("c.relname", "@parent")),
			always(`AND ` + like("a.attname", "@name")),
			always(`ORDER BY 1, 2, 4, 5`),
		},
		Fields: []dbmeta.Field{
			{Name: "schema"}, {Name: "table"}, {Name: "column"}, {Name: "ordinal"},
			{Name: "access", Desc: "the grantee, an equals sign and the privilege. A group or a role starts with its kind"},
			{Name: "grantee", Desc: "the user, group or role. Absent for PUBLIC"},
			{Name: "grantor", Desc: "always absent: SVV_COLUMN_PRIVILEGES records no grantor"},
			{Name: "privileges", Desc: "the one privilege of the row, such as SELECT or UPDATE"},
		},
		Params: childParams("column"),
		Scan: func(rows *sql.Rows) (dbmeta.ColumnPrivilege, error) {
			var v dbmeta.ColumnPrivilege
			err := rows.Scan(&v.Schema, &v.Table, &v.Column, &v.Ordinal, &v.Access,
				&v.Grantee, &v.Grantor, &v.Privileges)
			return v, err
		},
	})

	// A grant of a role to a user is in SVV_USER_GRANTS and a grant of a
	// role to a role is in SVV_ROLE_GRANTS. Both show a user only the grants
	// that concern it. A member of a group is in pg_group, which shows every
	// user the same rows.
	dbmeta.RoleGrants.Register(dbmeta.Redshift, &dbmeta.Binding[dbmeta.RoleGrant]{
		Stmt: dbmeta.Stmt{
			always(`SELECT m."role", m.member_of, NULL AS "grantor", m."admin", TRUE AS "inherit", TRUE AS "set"`),
			always(`FROM (`),
			always(`SELECT u.user_name AS "role", u.role_name AS member_of, u.admin_option AS "admin"` +
				` FROM svv_user_grants u`),
			always(`UNION ALL SELECT r.role_name, r.granted_role_name, FALSE FROM svv_role_grants r`),
			always(`UNION ALL SELECT u.usename, g.groname, FALSE FROM pg_group g` +
				` JOIN pg_user u ON u.usesysid = ANY (g.grolist)`),
			always(`) m`),
			always(`WHERE ` + like(`m."role"`, "@name")),
			always(`ORDER BY 1, 2`),
		},
		Fields: []dbmeta.Field{
			{Name: "role", Desc: "the user or the role that is the member"},
			{Name: "member_of", Desc: "the role or the group that it belongs to"},
			{Name: "grantor", Desc: "always absent: no Redshift view records the grantor"},
			{Name: "admin", Desc: "whether the member can grant the role. Always false for a group"},
			{Name: "inherit", Desc: "always true: a member always inherits the privileges"},
			{Name: "set", Desc: "always true: Redshift has no setting that withholds SET ROLE"},
		},
		Params: []dbmeta.Param{{Name: "name", Desc: "member name pattern, empty for every member", Default: ""}},
		Scan: func(rows *sql.Rows) (dbmeta.RoleGrant, error) {
			var v dbmeta.RoleGrant
			err := rows.Scan(&v.Role, &v.MemberOf, &v.Grantor, &v.Admin, &v.Inherit, &v.Set)
			return v, err
		},
	})

	dbmeta.DefaultACLs.Register(dbmeta.Redshift, &dbmeta.Binding[dbmeta.DefaultACL]{
		Stmt: dbmeta.Stmt{
			always(`SELECT pg_get_userbyid(d.defacluser) AS "owner"`),
			always(`, n.nspname AS "schema"`),
			always(`, CASE d.defaclobjtype WHEN 'r' THEN 'table' WHEN 'f' THEN 'function'` +
				` ELSE CAST(d.defaclobjtype AS text) END AS "type"`),
			always(`, array_to_string(d.defaclacl, ` + newline + `) AS "access"`),
			always(`FROM pg_default_acl d`),
			always(`LEFT JOIN pg_namespace n ON n.oid = d.defaclnamespace`),
			always(`WHERE ` + like("n.nspname", "@schema")),
			always(`ORDER BY 1, 2, 3`),
		},
		Fields: dbmeta.Fields("owner", "schema", "type", "access"),
		Params: []dbmeta.Param{{Name: "schema", Desc: "schema name pattern, empty for every schema", Default: ""}},
		Scan: func(rows *sql.Rows) (dbmeta.DefaultACL, error) {
			var v dbmeta.DefaultACL
			err := rows.Scan(&v.Owner, &v.Schema, &v.Type, &v.Access)
			return v, err
		},
	})

	dbmeta.Languages.Register(dbmeta.Redshift, &dbmeta.Binding[dbmeta.Language]{
		Stmt: dbmeta.Stmt{
			always(`SELECT l.lanname AS "name"`),
			always(`, NULL AS "owner"`),
			always(`, l.lanpltrusted AS "trusted"`),
			always(`, NOT l.lanispl AS "internal"`),
			always(`, (SELECT p.proname FROM pg_proc p WHERE p.oid = l.lanplcallfoid) AS "handler"`),
			always(`, (SELECT p.proname FROM pg_proc p WHERE p.oid = l.lanvalidator) AS "validator"`),
			always(`, NULL AS "inline"`),
			always(`, array_to_string(l.lanacl, ` + newline + `) AS "access"`),
			always(`, obj_description(l.oid, 'pg_language') AS "comment"`),
			always(`FROM pg_language l`),
			always(`WHERE l.lanplcallfoid <> 0`),
			always(`AND ` + like("l.lanname", "@name")),
			always(`ORDER BY 1`),
		},
		Fields: []dbmeta.Field{
			{Name: "name"},
			{Name: "owner", Desc: "always empty: pg_language records no owner"},
			{Name: "trusted"}, {Name: "internal"},
			{Name: "handler", Desc: "the name of the call handler, empty for a language without one"},
			{Name: "validator", Desc: "the name of the validator, empty for a language without one"},
			{Name: "inline", Desc: "always empty: pg_language records no inline handler"},
			{Name: "access"}, {Name: "comment"},
		},
		Params: []dbmeta.Param{{Name: "name", Desc: "language name pattern, empty for every language", Default: ""}},
		Scan: func(rows *sql.Rows) (dbmeta.Language, error) {
			var v dbmeta.Language
			err := rows.Scan(&v.Name, dbmeta.NullAsEmpty(&v.Owner), &v.Trusted, &v.Internal,
				dbmeta.NullAsEmpty(&v.Handler), dbmeta.NullAsEmpty(&v.Validator),
				dbmeta.NullAsEmpty(&v.Inline), &v.Access, &v.Comment)
			return v, err
		},
	})

	dbmeta.Settings.Register(dbmeta.Redshift, &dbmeta.Binding[dbmeta.Setting]{
		Stmt: dbmeta.Stmt{
			always(`SELECT s.name AS "name"`),
			always(`, s.setting AS "value"`),
			always(`, s.vartype AS "type"`),
			always(`, s.context AS "context"`),
			always(`, NULL AS "access"`),
			always(`, current_setting(s.name) AS "display"`),
			always(`FROM pg_settings s`),
			always(`WHERE ` + like("s.name", "@name")),
			always(`ORDER BY 1`),
		},
		Fields: []dbmeta.Field{
			{Name: "name"}, {Name: "value"}, {Name: "type"}, {Name: "context"},
			{Name: "access", Desc: "always absent: Redshift has no grant on a parameter"},
			{Name: "display", Desc: "the value as current_setting shows it"},
		},
		Params: []dbmeta.Param{{Name: "name", Desc: "parameter name pattern, empty for every parameter", Default: ""}},
		Scan: func(rows *sql.Rows) (dbmeta.Setting, error) {
			var v dbmeta.Setting
			err := rows.Scan(&v.Name, &v.Value, &v.Type, &v.Context, &v.Access, &v.Display)
			return v, err
		},
	})

	// Redshift 8.0 has no unnest, so a series of 32 positions, which is the
	// most columns a key holds, stands in for it and a position past the end
	// of the key finds no column.
	dbmeta.ConstraintColumns.Register(dbmeta.Redshift, &dbmeta.Binding[dbmeta.ConstraintColumn]{
		Stmt: dbmeta.Stmt{
			always(`SELECT current_database() AS "catalog"`),
			always(`, n.nspname AS "schema"`),
			always(`, t.relname AS "table"`),
			always(`, k.conname AS "constraint"`),
			always(`, a.attname AS "name"`),
			always(`, k.i AS "ordinal"`),
			always(`, CASE WHEN k.confrelid <> 0 THEN current_database() ELSE NULL END AS "foreign_catalog"`),
			always(`, fn.nspname AS "foreign_schema"`),
			always(`, ft.relname AS "foreign_table"`),
			always(`, fa.attname AS "foreign_name"`),
			always(`FROM (SELECT conname, conrelid, conkey, confrelid, confkey, generate_series(1, 32) AS i`),
			always(`FROM pg_constraint) k`),
			always(`JOIN pg_class t ON t.oid = k.conrelid`),
			always(`JOIN pg_namespace n ON n.oid = t.relnamespace`),
			always(`JOIN pg_attribute a ON a.attrelid = k.conrelid AND a.attnum = k.conkey[k.i]`),
			always(`LEFT JOIN pg_class ft ON ft.oid = k.confrelid`),
			always(`LEFT JOIN pg_namespace fn ON fn.oid = ft.relnamespace`),
			always(`LEFT JOIN pg_attribute fa ON fa.attrelid = k.confrelid AND fa.attnum = k.confkey[k.i]`),
			always(`WHERE k.conrelid <> 0`),
			always(`AND ` + notSystem("n.nspname")),
			always(`AND ` + like("n.nspname", "@schema")),
			always(`AND ` + like("t.relname", "@parent")),
			always(`AND ` + like("k.conname", "@name")),
			always(`ORDER BY 2, 3, 4, 6`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog"}, {Name: "schema"}, {Name: "table"},
			{Name: "constraint", Desc: "the constraint this column belongs to"},
			{Name: "name", Desc: "the column the constraint is on"},
			{Name: "ordinal", Desc: "one based position within the constraint"},
			{Name: "foreign_catalog", Desc: "set for a foreign key, absent otherwise"},
			{Name: "foreign_schema", Desc: "set for a foreign key, absent otherwise"},
			{Name: "foreign_table", Desc: "set for a foreign key, absent otherwise"},
			{Name: "foreign_name", Desc: "the column this one points at, in the same position of the key"},
		},
		Params: childParams("constraint"),
		Scan: func(rows *sql.Rows) (dbmeta.ConstraintColumn, error) {
			var v dbmeta.ConstraintColumn
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Table, &v.Constraint, &v.Name,
				&v.Ordinal, &v.ForeignCatalog, &v.ForeignSchema, &v.ForeignTable, &v.ForeignName)
			return v, err
		},
	})
}
