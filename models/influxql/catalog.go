package influxql

import (
	"cmp"
	"context"
	"iter"
	"maps"
	"slices"
	"strings"

	"github.com/xo/dbmeta"
)

func schemaNameSystem(kind string) []dbmeta.Param {
	return []dbmeta.Param{
		{Name: "schema", Desc: "database name pattern, empty for every database", Default: ""},
		{Name: "name", Desc: kind + " name pattern, empty for every " + kind, Default: ""},
		{Name: "with_system", Desc: "include _internal, _monitoring and _tasks, the databases InfluxDB keeps for itself", Default: false},
	}
}

// nameSystem are the parameters of the databases themselves.
var nameSystem = []dbmeta.Param{
	{Name: "name", Desc: "database name pattern, empty for every database", Default: ""},
	{Name: "with_system", Desc: "include _internal, _monitoring and _tasks, the databases InfluxDB keeps for itself", Default: false},
}

func register() {
	registerRelations()
	registerRoles()
}

func registerRelations() {
	// \dn. A database is the only namespace, so it is the schema. One
	// statement.
	dbmeta.Schemas.Register(dbmeta.InfluxQL, &dbmeta.Binding[dbmeta.Schema]{
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: "always empty: InfluxDB has nothing above a database"},
			{Name: "name"},
			{Name: "owner", Desc: "always empty: a database has no owner"},
			{Name: "comment", Desc: "always absent: InfluxDB has no comment"},
		},
		Params: nameSystem,
		Walk: func(ctx context.Context, db dbmeta.Queryer, args map[string]any) iter.Seq2[dbmeta.Schema, error] {
			dbs, err := databases(ctx, db, args, "name")
			out := make([]dbmeta.Schema, 0, len(dbs))
			for _, d := range dbs {
				out = append(out, dbmeta.Schema{Name: d})
			}
			return yieldAll(out, err)
		},
	})

	// \l. The same rows as Schemas, in the shape of a database. One
	// statement.
	dbmeta.Databases.Register(dbmeta.InfluxQL, &dbmeta.Binding[dbmeta.Database]{
		Fields: []dbmeta.Field{
			{Name: "name", Desc: "the database, which is a bucket through its mapping on InfluxDB 2"},
			{Name: "owner", Desc: "always empty: a database has no owner"},
			{Name: "encoding", Desc: "always empty: InfluxDB records no encoding"},
			{Name: "collate", Desc: "always empty: InfluxDB has no collation"},
			{Name: "ctype", Desc: "always empty: InfluxDB has no collation"},
			{Name: "access", Desc: "always absent: Privileges holds the grants on a database"},
			{Name: "tablespace", Desc: "always absent: InfluxDB has no tablespace"},
			{Name: "size", Desc: "always absent: no statement that every release has reports a size"},
			{Name: "comment", Desc: "always absent: InfluxDB has no comment"},
		},
		Params: nameSystem,
		Walk: func(ctx context.Context, db dbmeta.Queryer, args map[string]any) iter.Seq2[dbmeta.Database, error] {
			dbs, err := databases(ctx, db, args, "name")
			out := make([]dbmeta.Database, 0, len(dbs))
			for _, d := range dbs {
				out = append(out, dbmeta.Database{Name: d})
			}
			return yieldAll(out, err)
		},
	})

	// \dt. SHOW MEASUREMENTS ON lists the measurements of one database from
	// the index, so a walk costs one statement, and one more for each
	// database.
	dbmeta.Tables.Register(dbmeta.InfluxQL, &dbmeta.Binding[dbmeta.Table]{
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: "always empty: InfluxDB has nothing above a database"},
			{Name: "schema", Desc: "the database"},
			{Name: "name", Desc: "the measurement"},
			{Name: "type", Desc: "always table: a measurement is the one kind of relation"},
			{Name: "comment", Desc: "always absent: InfluxDB has no comment"},
		},
		Params: append(schemaNameSystem("measurement"), dbmeta.TypesParam()),
		Walk: func(ctx context.Context, db dbmeta.Queryer, args map[string]any) iter.Seq2[dbmeta.Table, error] {
			ms, err := measurements(ctx, db, args, "name")
			var out []dbmeta.Table
			if types := arg(args, "types"); types == "" || slices.Contains(strings.Split(types, ","), "table") {
				for _, m := range ms {
					out = append(out, dbmeta.Table{Schema: m.schema, Name: m.name, Type: "table"})
				}
			}
			return yieldAll(out, err)
		},
	})

	// \d NAME. SHOW FIELD KEYS ON and SHOW TAG KEYS ON list the fields and
	// the tags of every measurement of one database, as a series for each
	// measurement. Both read the index and not the points, which D165
	// measured. So a walk costs one statement, and two more for each
	// database.
	dbmeta.Columns.Register(dbmeta.InfluxQL, &dbmeta.Binding[dbmeta.Column]{
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: "always empty: InfluxDB has nothing above a database"},
			{Name: "schema", Desc: "the database"},
			{Name: "table", Desc: "the measurement"},
			{Name: "name"},
			{Name: "ordinal", Desc: "the position in the answer of SELECT *: time first, then every tag and field by name," +
				" a field before a tag of the same name"},
			{Name: "data_type", Desc: "timestamp for time, tag for a tag, as InfluxQL writes a cast to a tag," +
				" and the type of a field: float, integer, unsigned, string or boolean." +
				" A field that has two types in two shards is two rows"},
			{Name: "nullable", Desc: "false for time alone: a point can leave out any tag or field"},
			{Name: "default", Desc: "always absent: InfluxDB has no default"},
			{Name: "primary_key", Desc: "always false: InfluxDB has no primary key, and a point with the same tags and time replaces the one before it"},
			{Name: "identity", Desc: "always absent: InfluxDB has no identity column"},
			{Name: "generated", Desc: "always absent: InfluxDB has no generated column"},
			{Name: "comment", Desc: "always absent: InfluxDB has no comment"},
			{Name: "collation", Desc: "always absent: InfluxDB has no collation"},
		},
		Params: append([]dbmeta.Param{
			{Name: "parent", Desc: "measurement name pattern, empty for every measurement", Default: ""},
		}, schemaNameSystem("column")...),
		Walk: func(ctx context.Context, db dbmeta.Queryer, args map[string]any) iter.Seq2[dbmeta.Column, error] {
			return func(yield func(dbmeta.Column, error) bool) {
				dbs, err := databases(ctx, db, args, "schema")
				if err != nil {
					yield(dbmeta.Column{}, err)
					return
				}
				for _, d := range dbs {
					cols, err := columns(ctx, db, d, arg(args, "parent"))
					if err != nil {
						yield(dbmeta.Column{}, err)
						return
					}
					for _, c := range cols {
						if !dbmeta.Like(arg(args, "name"), c.Name) {
							continue
						}
						if !yield(c, nil) {
							return
						}
					}
				}
			}
		},
	})
}

// key is one tag or field of a measurement.
type key struct {
	name, typ string
	tag       bool
}

// columns reads the columns of every measurement of the database d whose
// name matches parent, in measurement order and then in ordinal order. It
// is two statements.
func columns(ctx context.Context, db dbmeta.Queryer, d, parent string) ([]dbmeta.Column, error) {
	fields, err := readAll(ctx, db, `SHOW FIELD KEYS ON `+quote(d))
	if err != nil {
		return nil, err
	}
	tags, err := readAll(ctx, db, `SHOW TAG KEYS ON `+quote(d))
	if err != nil {
		return nil, err
	}
	keys := make(map[string][]key)
	for _, s := range fields {
		for i := range s.rows {
			keys[s.name] = append(keys[s.name], key{name: s.value(i, "fieldKey").V, typ: s.value(i, "fieldType").V})
		}
	}
	for _, s := range tags {
		for i := range s.rows {
			keys[s.name] = append(keys[s.name], key{name: s.value(i, "tagKey").V, typ: "tag", tag: true})
		}
	}
	var out []dbmeta.Column
	for _, m := range slices.Sorted(maps.Keys(keys)) {
		if m == "" || !dbmeta.Like(parent, m) {
			continue
		}
		ks := keys[m]
		// SELECT * returns the keys by name after time, and a field before a
		// tag of the same name, which it renames with _1.
		slices.SortStableFunc(ks, func(a, b key) int {
			if c := cmp.Compare(a.name, b.name); c != 0 {
				return c
			}
			switch {
			case a.tag == b.tag:
				return 0
			case b.tag:
				return -1
			}
			return 1
		})
		out = append(out, dbmeta.Column{Schema: d, Table: m, Name: "time", Ordinal: 1, DataType: "timestamp"})
		for i, k := range ks {
			out = append(out, dbmeta.Column{
				Schema: d, Table: m, Name: k.name, Ordinal: i + 2,
				DataType: k.typ, Nullable: true,
			})
		}
	}
	return out, nil
}
