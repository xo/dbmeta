package influxql

import (
	"context"
	"database/sql"
	"iter"
	"maps"
	"slices"
	"strings"

	"github.com/xo/dbmeta"
)

// user is one row of SHOW USERS.
type user struct {
	name  string
	admin bool
}

// users lists every user whose name matches the pattern in param. It is one
// statement, which only an administrator can run, and which InfluxDB 2 and 3
// do not have.
func users(ctx context.Context, db dbmeta.Queryer, args map[string]any, param string) ([]user, error) {
	sets, err := readAll(ctx, db, `SHOW USERS`)
	if err != nil {
		return nil, err
	}
	var out []user
	for _, s := range sets {
		for i := range s.rows {
			name := s.value(i, "user").V
			if dbmeta.Like(arg(args, param), name) {
				out = append(out, user{name: name, admin: s.value(i, "admin").V == "true"})
			}
		}
	}
	return out, nil
}

func registerRoles() {
	// \du. An InfluxDB user is the role, because InfluxDB has no role
	// that groups users. An administrator can do everything, and every
	// other user has only the grants that Privileges lists. One statement.
	dbmeta.Roles.Register(dbmeta.InfluxQL, &dbmeta.Binding[dbmeta.Role]{
		Fields: []dbmeta.Field{
			{Name: "name", Desc: "the user"},
			{Name: "superuser", Desc: "whether the user is an administrator"},
			{Name: "create_role", Desc: "whether the user is an administrator, who alone can make a user"},
			{Name: "create_db", Desc: "whether the user is an administrator, who alone can make a database"},
			{Name: "can_login", Desc: "always true: every user logs in"},
			{Name: "replication", Desc: "always false: InfluxDB has no replication privilege"},
			{Name: "bypass_rls", Desc: "always false: InfluxDB has no row level security"},
			{Name: "inherit", Desc: "always false: InfluxDB has no role to inherit from"},
			{Name: "conn_limit", Desc: "always -1: a user has no connection limit"},
			{Name: "valid_until", Desc: "always absent: a password does not expire"},
			{Name: "member_of", Desc: "always empty: InfluxDB has no role to be a member of"},
			{Name: "comment", Desc: "always absent: InfluxDB has no comment"},
		},
		Params: []dbmeta.Param{{Name: "name", Desc: "user name pattern, empty for every user", Default: ""}},
		Walk: func(ctx context.Context, db dbmeta.Queryer, args map[string]any) iter.Seq2[dbmeta.Role, error] {
			us, err := users(ctx, db, args, "name")
			out := make([]dbmeta.Role, 0, len(us))
			for _, u := range us {
				out = append(out, dbmeta.Role{
					Name: u.name, Superuser: u.admin, CreateRole: u.admin, CreateDB: u.admin,
					CanLogin: true, ConnLimit: -1,
				})
			}
			return yieldAll(out, err)
		},
	})

	// \dp. A grant is READ, WRITE or ALL on one database, and SHOW GRANTS
	// FOR lists the grants of one user. So the walk costs one statement,
	// and one more for each user. A row here is one database with every
	// grant on it. An administrator holds every privilege without a grant,
	// and SHOW GRANTS lists none for one, so an administrator is not named.
	dbmeta.Privileges.Register(dbmeta.InfluxQL, &dbmeta.Binding[dbmeta.Privilege]{
		Fields: []dbmeta.Field{
			{Name: "schema", Desc: "the database"},
			{Name: "name", Desc: "always empty: a grant is on a whole database"},
			{Name: "type", Desc: "always database"},
			{Name: "access", Desc: "user=privilege for each grant on the database, sorted and joined by a comma and a space," +
				" where the privilege is READ, WRITE or ALL PRIVILEGES"},
			{Name: "column_access", Desc: "always absent: a grant is on a whole database"},
			{Name: "policies", Desc: "always absent: InfluxDB has no row level security"},
		},
		Params: []dbmeta.Param{
			{Name: "schema", Desc: "database name pattern, empty for every database", Default: ""},
			{Name: "name", Desc: "always matched against the empty name, so empty or % lists every grant", Default: ""},
		},
		Walk: func(ctx context.Context, db dbmeta.Queryer, args map[string]any) iter.Seq2[dbmeta.Privilege, error] {
			return yieldAll(privileges(ctx, db, args))
		},
	})

	// \dconfig. SHOW DIAGNOSTICS reports the configuration of InfluxDB 1 as
	// a series for each section, such as config-data, with a column for
	// each setting. Only an administrator can run it, and InfluxDB 2 and 3
	// do not have it. One statement.
	dbmeta.Settings.Register(dbmeta.InfluxQL, &dbmeta.Binding[dbmeta.Setting]{
		Fields: []dbmeta.Field{
			{Name: "name", Desc: "the section and the setting, such as data.cache-max-memory-size"},
			{Name: "value", Desc: "the value as text, and absent where it is null"},
			{Name: "type", Desc: "always absent: SHOW DIAGNOSTICS records no type"},
			{Name: "context", Desc: "always absent: InfluxDB reads every setting from its file when it starts"},
			{Name: "access", Desc: "always absent: a setting has no grant of its own"},
			{Name: "display", Desc: "always absent: SHOW DIAGNOSTICS shows a value in one form, which is value"},
		},
		Params: []dbmeta.Param{
			{Name: "name", Desc: "setting name pattern, empty for every setting", Default: ""},
		},
		Walk: func(ctx context.Context, db dbmeta.Queryer, args map[string]any) iter.Seq2[dbmeta.Setting, error] {
			return yieldAll(settings(ctx, db, args))
		},
	})
}

// privileges reads the grants of every user and returns one row for each
// database that holds one.
func privileges(ctx context.Context, db dbmeta.Queryer, args map[string]any) ([]dbmeta.Privilege, error) {
	us, err := users(ctx, db, map[string]any{}, "name")
	if err != nil {
		return nil, err
	}
	if !dbmeta.Like(arg(args, "name"), "") {
		return nil, nil
	}
	grants := make(map[string][]string)
	for _, u := range us {
		sets, err := readAll(ctx, db, `SHOW GRANTS FOR `+quote(u.name))
		if err != nil {
			return nil, err
		}
		for _, s := range sets {
			for i := range s.rows {
				d := s.value(i, "database").V
				p := s.value(i, "privilege").V
				if p == "NO PRIVILEGES" || !dbmeta.Like(arg(args, "schema"), d) {
					continue
				}
				grants[d] = append(grants[d], u.name+"="+p)
			}
		}
	}
	var out []dbmeta.Privilege
	for _, d := range slices.Sorted(maps.Keys(grants)) {
		access := grants[d]
		slices.Sort(access)
		out = append(out, dbmeta.Privilege{
			Schema: sql.Null[string]{V: d, Valid: true},
			Type:   "database",
			Access: sql.Null[string]{V: strings.Join(access, ", "), Valid: true},
		})
	}
	return out, nil
}

// diagnosticsConfig is the series of SHOW DIAGNOSTICS that repeats
// config-data under a shorter name. InfluxDB 1 registers the data section
// twice, so the model reads config-data and skips this one.
const diagnosticsConfig = "config"

// settings reads the configuration from SHOW DIAGNOSTICS.
func settings(ctx context.Context, db dbmeta.Queryer, args map[string]any) ([]dbmeta.Setting, error) {
	sets, err := readAll(ctx, db, `SHOW DIAGNOSTICS`)
	if err != nil {
		return nil, err
	}
	var out []dbmeta.Setting
	for _, s := range sets {
		section, ok := strings.CutPrefix(s.name, diagnosticsConfig+"-")
		if !ok || len(s.rows) == 0 {
			continue
		}
		for j, col := range s.cols {
			name := section + "." + col
			if col == "measurement" || !dbmeta.Like(arg(args, "name"), name) {
				continue
			}
			out = append(out, dbmeta.Setting{Name: name, Value: s.rows[0][j]})
		}
	}
	slices.SortFunc(out, func(a, b dbmeta.Setting) int { return strings.Compare(a.Name, b.Name) })
	return out, nil
}
