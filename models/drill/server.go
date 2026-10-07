package drill

import (
	"database/sql"

	"github.com/xo/dbmeta"
)

func registerServer() {
	// sys.options has 227 rows on 1.22.0. status is DEFAULT or CHANGED and
	// has no field. description has none either, because Setting.Display is
	// the value as shown with its unit and a description is another thing.
	dbmeta.Settings.Register(dbmeta.Drill, &dbmeta.Binding[dbmeta.Setting]{
		Stmt: dbmeta.Stmt{
			always("SELECT o.name AS `name`"),
			always(", o.val AS `value`"),
			always(", o.kind AS `type`"),
			always(", o.optionScope AS `context`"),
			always(", o.accessibleScopes AS `access`"),
			always(", NULL AS `display`"),
			always("FROM sys.options o"),
			always("WHERE " + like("o.name", "@name")),
			always("ORDER BY 1"),
		},
		Fields: []dbmeta.Field{
			{Name: "name", Desc: "the option, such as planner.width.max_per_node"},
			{Name: "value", Desc: "the value as text"},
			{Name: "type", Desc: "the type of the option: BIT, BIGINT, FLOAT, VARCHAR or the like"},
			{Name: "context", Desc: "the scope the value comes from: BOOT, SYSTEM or SESSION"},
			{Name: "access", Desc: "the scopes at which the option can be set: ALL, SYSTEM, SESSION_AND_QUERY or QUERY"},
			{Name: "display", Desc: "always absent: Drill shows a value in one form, which is value"},
		},
		Params: []dbmeta.Param{{Name: "name", Desc: "option name pattern, empty for every option", Default: ""}},
		Scan: func(rows *sql.Rows) (dbmeta.Setting, error) {
			var v dbmeta.Setting
			err := rows.Scan(&v.Name, &v.Value, &v.Type, &v.Context, &v.Access, &v.Display)
			return v, err
		},
	})

	// USER is the user of the request, and SESSION_USER is the user that the
	// session authenticated as. They differ only with impersonation.
	dbmeta.CurrentUser.Register(dbmeta.Drill, &dbmeta.Binding[dbmeta.User]{
		Stmt: dbmeta.Stmt{
			always("SELECT USER AS `name`"),
			always(", SESSION_USER AS `session`"),
		},
		Fields: []dbmeta.Field{
			{Name: "name"},
			{Name: "session", Desc: "the user that authenticated, which is the same as name unless the request impersonates another user"},
		},
		Scan: func(rows *sql.Rows) (dbmeta.User, error) {
			var v dbmeta.User
			err := rows.Scan(&v.Name, &v.Session)
			return v, err
		},
	})
}
