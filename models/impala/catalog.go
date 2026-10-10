package impala

import (
	"context"
	"database/sql"
	"iter"
	"strconv"
	"strings"

	"github.com/xo/dbmeta"
)

func always(query string) dbmeta.Choice { return dbmeta.Choice{{Query: query}} }

// yieldAll yields each value, or the error when there is one.
func yieldAll[T any](vs []T, err error) iter.Seq2[T, error] {
	return func(yield func(T, error) bool) {
		if err != nil {
			var zero T
			yield(zero, err)
			return
		}
		for _, v := range vs {
			if !yield(v, nil) {
				return
			}
		}
	}
}

func schemaNameSystem(kind string) []dbmeta.Param {
	return []dbmeta.Param{
		{Name: "schema", Desc: "database name pattern, empty for every database", Default: ""},
		{Name: "name", Desc: kind + " name pattern, empty for every " + kind, Default: ""},
		{Name: "with_system", Desc: "include _impala_builtins, the database of the built in functions", Default: false},
	}
}

func childParams(kind string) []dbmeta.Param {
	return append([]dbmeta.Param{
		{Name: "parent", Desc: "table name pattern, empty for every table", Default: ""},
	}, schemaNameSystem(kind)...)
}

// nameSystem are the parameters of the databases themselves.
var nameSystem = []dbmeta.Param{
	{Name: "name", Desc: "database name pattern, empty for every database", Default: ""},
	{Name: "with_system", Desc: "include _impala_builtins, the database of the built in functions", Default: false},
}

func register() {
	// \dn. A database is the only namespace, so it is the schema.
	dbmeta.Schemas.Register(dbmeta.Impala, &dbmeta.Binding[dbmeta.Schema]{
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: "always empty: Impala has nothing above a database"},
			{Name: "name"},
			{Name: "owner", Desc: "always empty: SHOW DATABASES records no owner"},
			{Name: "comment"},
		},
		Params: nameSystem,
		Walk: func(ctx context.Context, db dbmeta.Queryer, args map[string]any) iter.Seq2[dbmeta.Schema, error] {
			dbs, err := databases(ctx, db, args, "name")
			var out []dbmeta.Schema
			for _, d := range dbs {
				out = append(out, dbmeta.Schema{Name: d.name, Comment: d.comment})
			}
			return yieldAll(out, err)
		},
	})

	// \l. The same rows as Schemas, in the shape of a database.
	dbmeta.Databases.Register(dbmeta.Impala, &dbmeta.Binding[dbmeta.Database]{
		Fields: []dbmeta.Field{
			{Name: "name"},
			{Name: "owner", Desc: "always empty: SHOW DATABASES records no owner"},
			{Name: "encoding", Desc: "always empty: Impala records no encoding"},
			{Name: "collate", Desc: "always empty: Impala has no collation"},
			{Name: "ctype", Desc: "always empty: Impala has no collation"},
			{Name: "access", Desc: "always absent: authorization is off in this image"},
			{Name: "tablespace", Desc: "always absent: Impala has no tablespace"},
			{Name: "size", Desc: "always absent: SHOW DATABASES records no size"},
			{Name: "comment"},
		},
		Params: nameSystem,
		Walk: func(ctx context.Context, db dbmeta.Queryer, args map[string]any) iter.Seq2[dbmeta.Database, error] {
			dbs, err := databases(ctx, db, args, "name")
			var out []dbmeta.Database
			for _, d := range dbs {
				out = append(out, dbmeta.Database{Name: d.name, Comment: d.comment})
			}
			return yieldAll(out, err)
		},
	})

	// \dt and \dv. SHOW TABLES IN lists the tables and views of a
	// database, and DESCRIBE FORMATTED says what each is and holds its
	// comment. Ken chose on 2026-09-30 to read it, which is one more
	// statement for each table (D146).
	dbmeta.Tables.Register(dbmeta.Impala, &dbmeta.Binding[dbmeta.Table]{
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: "always empty: Impala has nothing above a database"},
			{Name: "schema"}, {Name: "name"},
			{Name: "type", Desc: "table, external table, view or materialized view, from the Table Type of DESCRIBE FORMATTED"},
			{Name: "comment", Desc: "the comment table parameter of DESCRIBE FORMATTED"},
			{Name: "owner", Desc: "the Owner row of DESCRIBE FORMATTED, which the walk already reads. Absent where the row is empty"},
			{Name: "persistence", Desc: "permanent for a table and absent for a view. Impala has no temporary or unlogged table"},
			{Name: "access_method", Desc: "the InputFormat row of DESCRIBE FORMATTED, such as org.apache.hadoop.hive.ql.io.parquet.MapredParquetInputFormat. Absent for a view and where the row is empty"},
			{Name: "size", Desc: "the totalSize table parameter, the bytes of the files, which COMPUTE STATS and a refresh fill. Absent for a view and for a table with no value"},
			{Name: "rows", Desc: "the numRows table parameter, which COMPUTE STATS fills. Impala writes -1 where it does not know, and that is absent here"},
			{Name: "options", Desc: "always absent: the table parameters mix statistics, DDL times and settings in one list, and no key marks a setting"},
		},
		Params: append(schemaNameSystem("table"), dbmeta.TypesParam()),
		Walk: func(ctx context.Context, db dbmeta.Queryer, args map[string]any) iter.Seq2[dbmeta.Table, error] {
			return func(yield func(dbmeta.Table, error) bool) {
				rels, err := relations(ctx, db, args, "name")
				if err != nil {
					yield(dbmeta.Table{}, err)
					return
				}
				types := arg(args, "types")
				for _, r := range rels {
					v, err := describe(ctx, db, r)
					if err != nil {
						yield(dbmeta.Table{}, err)
						return
					}
					if types != "" && !dbmeta.ListHas(types, v.Type) {
						continue
					}
					if !yield(v, nil) {
						return
					}
				}
			}
		},
	})

	// \d NAME. DESCRIBE lists a table's columns in their order, with the
	// type and the comment, for a table and a view alike.
	dbmeta.Columns.Register(dbmeta.Impala, &dbmeta.Binding[dbmeta.Column]{
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: "always empty: Impala has nothing above a database"},
			{Name: "schema"}, {Name: "table"}, {Name: "name"},
			{Name: "ordinal", Desc: "the position in DESCRIBE, from 1"},
			{Name: "data_type", Desc: "the type as Impala writes it, such as int or string"},
			{Name: "nullable", Desc: "always true: DESCRIBE does not say that a column is NOT NULL"},
			{Name: "default", Desc: "always absent: DESCRIBE records no default"},
			{Name: "primary_key", Desc: "always false: DESCRIBE does not say which columns a key holds"},
			{Name: "identity", Desc: "always absent: Impala has no identity column"},
			{Name: "generated", Desc: "always absent: Impala has no generated column"},
			{Name: "comment"},
			{Name: "collation", Desc: "always absent: Impala has no collation"},
		},
		Params: childParams("column"),
		Walk: func(ctx context.Context, db dbmeta.Queryer, args map[string]any) iter.Seq2[dbmeta.Column, error] {
			return func(yield func(dbmeta.Column, error) bool) {
				rels, err := relations(ctx, db, args, "parent")
				if err != nil {
					yield(dbmeta.Column{}, err)
					return
				}
				for _, r := range rels {
					rows, err := readAll(ctx, db, `DESCRIBE `+quote(r.schema)+`.`+quote(r.name))
					if err != nil {
						yield(dbmeta.Column{}, err)
						return
					}
					for i, c := range rows {
						if !dbmeta.Like(arg(args, "name"), c[0]) {
							continue
						}
						v := dbmeta.Column{
							Schema: r.schema, Table: r.name, Name: c[0], Ordinal: i + 1,
							DataType: c[1], Nullable: true,
						}
						if len(c) > 2 && c[2] != "" {
							v.Comment = sql.Null[string]{V: c[2], Valid: true}
						}
						if !yield(v, nil) {
							return
						}
					}
				}
			}
		},
	})

	// The views, with the statement each selects from SHOW CREATE VIEW.
	dbmeta.Views.Register(dbmeta.Impala, &dbmeta.Binding[dbmeta.View]{
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: "always empty: Impala has nothing above a database"},
			{Name: "schema"}, {Name: "name"},
			{Name: "definition", Desc: "the CREATE VIEW statement, from SHOW CREATE VIEW"},
			{Name: "check_option", Desc: "always absent: Impala has no WITH CHECK OPTION"},
			{Name: "updatable", Desc: "always absent: an Impala view cannot be written"},
			{Name: "insertable", Desc: "always absent: an Impala view cannot be written"},
			{Name: "comment", Desc: "always absent: the comment of a view is only in DESCRIBE FORMATTED"},
		},
		Params: schemaNameSystem("view"),
		Walk: func(ctx context.Context, db dbmeta.Queryer, args map[string]any) iter.Seq2[dbmeta.View, error] {
			return func(yield func(dbmeta.View, error) bool) {
				rels, err := relations(ctx, db, args, "name")
				if err != nil {
					yield(dbmeta.View{}, err)
					return
				}
				for _, r := range rels {
					if !r.view {
						continue
					}
					rows, err := readAll(ctx, db, `SHOW CREATE VIEW `+quote(r.schema)+`.`+quote(r.name))
					if err != nil {
						yield(dbmeta.View{}, err)
						return
					}
					v := dbmeta.View{Schema: r.schema, Name: r.name}
					if len(rows) > 0 {
						v.Definition = sql.Null[string]{V: rows[0][0], Valid: true}
					}
					if !yield(v, nil) {
						return
					}
				}
			}
		},
	})

	// SHOW COLUMN STATS, which COMPUTE STATS fills. -1 is Impala's word for a
	// value it does not know.
	dbmeta.ColumnStats.Register(dbmeta.Impala, &dbmeta.Binding[dbmeta.ColumnStat]{
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: "always empty: Impala has nothing above a database"},
			{Name: "schema"}, {Name: "table"}, {Name: "name"},
			{Name: "avg_width", Desc: "the average size, and absent where Impala does not know it"},
			{Name: "null_frac", Desc: "always absent: SHOW COLUMN STATS gives the NULL count and not the row count"},
			{Name: "distinct", Desc: "the distinct count, and absent where Impala does not know it"},
			{Name: "min", Desc: "always absent: SHOW COLUMN STATS records no bounds"},
			{Name: "max", Desc: "always absent: SHOW COLUMN STATS records no bounds"},
			{Name: "mean", Desc: "always absent: Impala records no mean"},
			{Name: "top_n", Desc: "always absent: Impala records no most common values"},
			{Name: "top_n_freqs", Desc: "always absent, for the same reason as top_n"},
		},
		Params: childParams("column"),
		Walk: func(ctx context.Context, db dbmeta.Queryer, args map[string]any) iter.Seq2[dbmeta.ColumnStat, error] {
			return func(yield func(dbmeta.ColumnStat, error) bool) {
				rels, err := relations(ctx, db, args, "parent")
				if err != nil {
					yield(dbmeta.ColumnStat{}, err)
					return
				}
				for _, r := range rels {
					if r.view {
						continue
					}
					rows, err := readAll(ctx, db, `SHOW COLUMN STATS `+quote(r.schema)+`.`+quote(r.name))
					if err != nil {
						yield(dbmeta.ColumnStat{}, err)
						return
					}
					for _, c := range rows {
						if len(c) < 6 || !dbmeta.Like(arg(args, "name"), c[0]) {
							continue
						}
						v := dbmeta.ColumnStat{Schema: r.schema, Table: r.name, Name: c[0]}
						if n, err := strconv.ParseFloat(c[2], 64); err == nil && n >= 0 {
							v.Distinct = sql.Null[float64]{V: n, Valid: true}
						}
						if n, err := strconv.ParseFloat(c[5], 64); err == nil && n >= 0 {
							v.AvgWidth = sql.Null[int64]{V: int64(n), Valid: true}
						}
						if !yield(v, nil) {
							return
						}
					}
				}
			}
		},
	})

	// \df and \da. SHOW FUNCTIONS IN lists the functions of one database,
	// with the return type and the signature, and the built in ones are in
	// _impala_builtins.
	for _, c := range []struct {
		q    *dbmeta.Query[dbmeta.Function]
		show string
		kind string
	}{
		{dbmeta.Functions, "SHOW FUNCTIONS IN ", "func"},
		{dbmeta.Aggregates, "SHOW AGGREGATE FUNCTIONS IN ", "agg"},
	} {
		c.q.Register(dbmeta.Impala, &dbmeta.Binding[dbmeta.Function]{
			Fields: []dbmeta.Field{
				{Name: "catalog", Desc: "always empty: Impala has nothing above a database"},
				{Name: "schema", Desc: "the database, and _impala_builtins for a function built into Impala"},
				{Name: "name"},
				{Name: "id", Desc: "the signature, because a name is overloaded"},
				{Name: "kind", Desc: "func or agg"},
				{Name: "result_type"},
				{Name: "arg_types", Desc: "the arguments in the signature, such as STRING"},
				{Name: "volatility", Desc: "always empty: Impala does not record it"},
				{Name: "parallel", Desc: "always empty: Impala does not record it"},
				{Name: "owner", Desc: "always absent: Impala records no owner"},
				{Name: "security", Desc: "always empty: Impala does not record it"},
				{Name: "access", Desc: "always absent: authorization is off in this image"},
				{Name: "language", Desc: "the binary type: builtin, java, native or ir"},
				{Name: "source", Desc: "always absent: SHOW FUNCTIONS records no body"},
				{Name: "comment", Desc: "always absent: a function takes no comment"},
				{Name: "definition", Desc: "always absent: SHOW CREATE FUNCTION is a statement for each database"},
			},
			Params: schemaNameSystem("function"),
			Walk: func(ctx context.Context, db dbmeta.Queryer, args map[string]any) iter.Seq2[dbmeta.Function, error] {
				return func(yield func(dbmeta.Function, error) bool) {
					dbs, err := databases(ctx, db, args, "schema")
					if err != nil {
						yield(dbmeta.Function{}, err)
						return
					}
					for _, d := range dbs {
						rows, err := readAll(ctx, db, c.show+quote(d.name))
						if err != nil {
							yield(dbmeta.Function{}, err)
							return
						}
						for _, f := range rows {
							if len(f) < 3 {
								continue
							}
							name, rest, _ := strings.Cut(f[1], "(")
							if !dbmeta.Like(arg(args, "name"), name) {
								continue
							}
							v := dbmeta.Function{
								Schema: d.name, Name: name, Kind: c.kind,
								ID:         sql.Null[string]{V: f[1], Valid: true},
								ResultType: sql.Null[string]{V: f[0], Valid: true},
								ArgTypes:   sql.Null[string]{V: strings.TrimSuffix(rest, ")"), Valid: true},
								Language:   strings.ToLower(f[2]),
							}
							if !yield(v, nil) {
								return
							}
						}
					}
				}
			},
		})
	}

	// \dconfig. SET ALL lists the query options, with their level.
	dbmeta.Settings.Register(dbmeta.Impala, &dbmeta.Binding[dbmeta.Setting]{
		Fields: []dbmeta.Field{
			{Name: "name", Desc: "the query option in lower case"},
			{Name: "value", Desc: "the value, and absent where it is empty"},
			{Name: "type", Desc: "always absent: SET ALL records no type"},
			{Name: "context", Desc: "the level: regular, advanced, development, deprecated or removed"},
			{Name: "access", Desc: "always absent: an option has no grant of its own"},
			{Name: "display", Desc: "always absent: SET ALL shows a value in one form, which is value"},
		},
		Params: []dbmeta.Param{
			{Name: "name", Desc: "option name pattern, empty for every option", Default: ""},
		},
		Walk: func(ctx context.Context, db dbmeta.Queryer, args map[string]any) iter.Seq2[dbmeta.Setting, error] {
			rows, err := readAll(ctx, db, `SET ALL`)
			var out []dbmeta.Setting
			for _, r := range rows {
				name := strings.ToLower(r[0])
				if len(r) < 3 || !dbmeta.Like(arg(args, "name"), name) {
					continue
				}
				v := dbmeta.Setting{Name: name, Context: sql.Null[string]{V: strings.ToLower(r[2]), Valid: true}}
				if r[1] != "" {
					v.Value = sql.Null[string]{V: r[1], Valid: true}
				}
				out = append(out, v)
			}
			return yieldAll(out, err)
		},
	})

	// The current database, which is one statement.
	dbmeta.CurrentSchema.Register(dbmeta.Impala, &dbmeta.Binding[dbmeta.Schema]{
		Stmt: dbmeta.Stmt{
			always(`SELECT '' AS "catalog"`),
			always(`, current_database() AS "name"`),
			always(`, '' AS "owner"`),
			always(`, CAST(NULL AS STRING) AS "comment"`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: "always empty: Impala has nothing above a database"},
			{Name: "name"},
			{Name: "owner", Desc: "always empty: Impala records no owner"},
			{Name: "comment", Desc: "always absent: the comment is only in SHOW DATABASES"},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Schema, error) {
			var v dbmeta.Schema
			err := rows.Scan(&v.Catalog, &v.Name, &v.Owner, &v.Comment)
			return v, err
		},
	})

	// The user the session acts as, and the one that logged in.
	dbmeta.CurrentUser.Register(dbmeta.Impala, &dbmeta.Binding[dbmeta.User]{
		Stmt: dbmeta.Stmt{
			always(`SELECT effective_user() AS "name"`),
			always(`, logged_in_user() AS "session"`),
		},
		Fields: []dbmeta.Field{
			{Name: "name", Desc: "the effective user, which is anonymous with no authentication"},
			{Name: "session", Desc: "the user that logged in"},
		},
		Scan: func(rows *sql.Rows) (dbmeta.User, error) {
			var v dbmeta.User
			err := rows.Scan(&v.Name, &v.Session)
			return v, err
		},
	})
}

// tableTypes are the words for the Table Type DESCRIBE FORMATTED gives, as
// the hive model names the same values of the same metastore.
var tableTypes = map[string]string{
	"MANAGED_TABLE":     "table",
	"EXTERNAL_TABLE":    "external table",
	"VIRTUAL_VIEW":      "view",
	"MATERIALIZED_VIEW": "materialized view",
}

// describe reads the type, the comment, the owner, the input format and the
// statistics of one relation from DESCRIBE FORMATTED, whose rows are a label
// and a value. A table parameter is a row with no label, the key and the
// value, and a column is never such a row, because a column's row starts with
// its name. The statistics are in the table parameters, so Rows and Size cost
// no statement beyond the one this walk already runs for the type.
func describe(ctx context.Context, db dbmeta.Queryer, r relation) (dbmeta.Table, error) {
	v := dbmeta.Table{Schema: r.schema, Name: r.name, Type: r.kind()}
	rows, err := readAll(ctx, db, `DESCRIBE FORMATTED `+quote(r.schema)+`.`+quote(r.name))
	if err != nil {
		return v, err
	}
	var format string
	for _, row := range rows {
		if len(row) < 3 {
			continue
		}
		label, value := strings.TrimSpace(row[0]), strings.TrimSpace(row[1])
		switch {
		case label == "Table Type:":
			if t, ok := tableTypes[value]; ok {
				v.Type = t
			} else {
				v.Type = strings.ToLower(value)
			}
		case label == "Owner:" && value != "" && value != "null":
			v.Owner = sql.Null[string]{V: value, Valid: true}
		case label == "InputFormat:" && value != "" && value != "null":
			format = value
		case label == "" && value == "comment":
			v.Comment = sql.Null[string]{V: strings.TrimSpace(row[2]), Valid: true}
		case label == "" && value == "numRows":
			v.Rows = statistic(row[2])
		case label == "" && value == "totalSize":
			v.Size = statistic(row[2])
		}
	}
	if !strings.HasSuffix(v.Type, "view") {
		v.Persistence = sql.Null[string]{V: "permanent", Valid: true}
		if format != "" {
			v.AccessMethod = sql.Null[string]{V: format, Valid: true}
		}
	}
	return v, nil
}

// statistic reads a table parameter that holds a count. Impala writes -1 where
// it does not know the value, and that is absent.
func statistic(s string) sql.Null[int64] {
	n, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	if err != nil || n < 0 {
		return sql.Null[int64]{}
	}
	return sql.Null[int64]{V: n, Valid: true}
}
