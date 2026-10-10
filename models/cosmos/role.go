package cosmos

import (
	"database/sql"
	"strconv"
	"strings"

	"github.com/xo/dbmeta"
)

// Roles, privileges and settings.

// resourceKind says what a resource link names, by the number of its parts. A
// link is dbs/RID/, dbs/RID/colls/RID/, and so on down to a document, and every
// id in it is a resource id and not a name.
func resourceKind(link string) string {
	parts := strings.Split(strings.Trim(link, "/"), "/")
	if len(parts) < 2 {
		return "resource"
	}
	switch parts[len(parts)-2] {
	case "dbs":
		return "database"
	case "colls":
		return "container"
	case "docs":
		return "document"
	case "sprocs":
		return "stored procedure"
	case "triggers":
		return "trigger"
	case "udfs":
		return "function"
	case "users":
		return "user"
	}
	return parts[len(parts)-2]
}

func registerRoles() {
	// \du. A user of a database is a principal that holds permissions, and an
	// application hands its resource token to a client. It has no password, no
	// attribute and no membership. The feed has an id and two links.
	dbmeta.Roles.Register(dbmeta.Cosmos, &dbmeta.Binding[dbmeta.Role]{
		Stmt: catalog("$users"),
		Fields: []dbmeta.Field{
			field("database", "the database that the URL names"),
			field("id", "the name of the user"),
			field("rid", "the resource id of the user"),
			field("link", "the self link of the user"),
		},
		Params: accountFilters("user"),
		Keep:   keep(nil, nil, func(v dbmeta.Role) string { return v.Name }),
		Scan: func(rows *sql.Rows) (dbmeta.Role, error) {
			r, err := scanRow(rows)
			// A user cannot sign in, and it has no limit on its connections.
			return dbmeta.Role{Name: r.text("id"), ConnLimit: -1}, err
		},
	})

	// \dp. A permission is a grant of Read or All on one resource to one user. It
	// is a row of its own, so a resource that two users can read has two rows.
	dbmeta.Privileges.Register(dbmeta.Cosmos, &dbmeta.Binding[dbmeta.Privilege]{
		Stmt: catalog("$permissions"),
		Fields: []dbmeta.Field{
			field("database", "the database that the URL names. It is the schema"),
			field("user", "the user that holds the permission"),
			field("id", "the name of the permission"),
			field("rid", "the resource id of the permission"),
			field("link", "the self link of the permission"),
			field("permission_mode", "Read or All"),
			field("resource_link", "the link of the resource, which is rid based"),
		},
		Params: databaseFilters("permission"),
		Keep:   keep(func(v dbmeta.Privilege) string { return v.Schema.V }, nil, func(v dbmeta.Privilege) string { return v.Name }),
		Scan: func(rows *sql.Rows) (dbmeta.Privilege, error) {
			r, err := scanRow(rows)
			link := r.text("resource_link")
			return dbmeta.Privilege{
				Schema: r.nullText("database"),
				// The link is the only name that the feed gives the resource.
				Name:   link,
				Type:   resourceKind(link),
				Access: sql.Null[string]{V: r.text("user") + "=" + r.text("permission_mode"), Valid: r.text("user") != ""},
			}, err
		},
	})

	// \dconfig. The provisioned throughput of a database and of each container that
	// has its own is the nearest to a setting that the service lists. A database
	// that shares its throughput has one row, and a container with its own has a
	// row too.
	dbmeta.Settings.Register(dbmeta.Cosmos, &dbmeta.Binding[dbmeta.Setting]{
		Stmt: catalog("$offers"),
		Fields: []dbmeta.Field{
			field("scope", "database or container"),
			field("database", "the database that the URL names"),
			field("container", "the container of a container offer, and absent for a database offer"),
			field("id", "the id of the offer"),
			field("rid", "the resource id of the offer"),
			field("resource_link", "the link of the resource that the offer is for"),
			field("version", "the version of the offer, V2 for request units"),
			field("offer_type", "Invalid for request units, and the tier of a legacy offer"),
			field("throughput", "the manual request units a second, and absent for autoscale"),
			field("autoscale_max_throughput", "the largest request units a second of an autoscale offer, and absent for a manual one"),
			field("autoscale_increment_percent", "the percent that an autoscale offer grows by, and absent for a manual one"),
		},
		Params: accountFilters("setting"),
		Keep:   keep(nil, nil, func(v dbmeta.Setting) string { return v.Name }),
		Scan: func(rows *sql.Rows) (dbmeta.Setting, error) {
			r, err := scanRow(rows)
			name := "throughput " + r.text("database")
			if c := r.text("container"); c != "" {
				name += "/" + c
			}
			v := dbmeta.Setting{Name: name, Context: r.nullText("scope")}
			switch manual, auto := r.number("throughput"), r.number("autoscale_max_throughput"); {
			case manual.Valid:
				v.Value = sql.Null[string]{V: strconv.FormatInt(manual.V, 10), Valid: true}
				v.Type = sql.Null[string]{V: "manual", Valid: true}
				v.Display = sql.Null[string]{V: v.Value.V + " RU/s", Valid: true}
			case auto.Valid:
				v.Value = sql.Null[string]{V: strconv.FormatInt(auto.V, 10), Valid: true}
				v.Type = sql.Null[string]{V: "autoscale", Valid: true}
				v.Display = sql.Null[string]{V: "up to " + v.Value.V + " RU/s", Valid: true}
			}
			return v, err
		},
	})
}
