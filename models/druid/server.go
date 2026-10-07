package druid

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/xo/dbmeta"
)

func registerServer() {
	// sys.server_properties holds the runtime properties of every service of
	// the cluster, one row for each property of each service. A name repeats
	// across the services, so the context names the service. The table needs
	// the permission STATE, which an ordinary user does not hold.
	dbmeta.Settings.Register(dbmeta.Druid, &dbmeta.Binding[dbmeta.Setting]{
		Stmt: dbmeta.Stmt{
			always(`SELECT s.property AS "name"`),
			always(`, s."value" AS "value"`),
			always(`, NULL AS "type"`),
			always(`, s.service_name AS "context"`),
			always(`, NULL AS "access"`),
			always(`, NULL AS "display"`),
			always(`FROM sys.server_properties s`),
			always(`WHERE ` + like("s.property", "@name")),
			always(`ORDER BY 1, 4, s.server`),
		},
		Fields: []dbmeta.Field{
			{Name: "name", Desc: "the runtime property, such as druid.processing.numThreads. It repeats for each service that sets it"},
			{Name: "value", Desc: "the value as text. A value that is a JSON array of strings, such as the list of extensions, arrives from the driver as a list, and the model writes it back as compact JSON"},
			{Name: "type", Desc: "always absent: server_properties records no type"},
			{Name: "context", Desc: "the service that holds the property, such as druid/router"},
			{Name: "access", Desc: "always absent: a property has no grant"},
			{Name: "display", Desc: "always absent: Druid shows a value in one form, which is value"},
		},
		Params: []dbmeta.Param{{Name: "name", Desc: "property name pattern, empty for every property", Default: ""}},
		Scan: func(rows *sql.Rows) (dbmeta.Setting, error) {
			var (
				v     dbmeta.Setting
				value any
			)
			if err := rows.Scan(&v.Name, &value, &v.Type, &v.Context, &v.Access, &v.Display); err != nil {
				return v, err
			}
			var err error
			v.Value, err = text(value)
			return v, err
		},
	})
}

// text reads a value as text. The driver reads a string that holds a JSON array
// of two or more strings as a list, because Druid names it and a multi-value
// string alike (dbimp D164). A setting such as druid.extensions.loadList is
// that text, so the list is written back as JSON.
func text(v any) (sql.Null[string], error) {
	switch t := v.(type) {
	case nil:
		return sql.Null[string]{}, nil
	case string:
		return sql.Null[string]{V: t, Valid: true}, nil
	case []any:
		b, err := json.Marshal(t)
		if err != nil {
			return sql.Null[string]{}, fmt.Errorf("writing a list value as JSON: %w", err)
		}
		return sql.Null[string]{V: string(b), Valid: true}, nil
	}
	return sql.Null[string]{}, fmt.Errorf("reading a %T value as text", v)
}
