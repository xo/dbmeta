package impala

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/xo/dbmeta"
)

// builtins is the database that holds the functions built into Impala, which
// is the one database Impala keeps for itself.
const builtins = "_impala_builtins"

// readAll reads every row of query as text, closing the rows before it
// returns, so that a walk holds one connection at a time.
func readAll(ctx context.Context, db dbmeta.Queryer, query string) ([][]string, error) {
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("running %s: %w", query, err)
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("reading the columns of %s: %w", query, err)
	}
	var out [][]string
	for rows.Next() {
		vals := make([]sql.Null[string], len(cols))
		dest := make([]any, len(cols))
		for i := range vals {
			dest[i] = &vals[i]
		}
		if err := rows.Scan(dest...); err != nil {
			return nil, fmt.Errorf("reading a row of %s: %w", query, err)
		}
		row := make([]string, len(cols))
		for i, v := range vals {
			row[i] = v.V
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("reading %s: %w", query, err)
	}
	return out, nil
}

// arg returns a text argument of a walk, which has every parameter filled.
func arg(args map[string]any, name string) string {
	s, _ := args[name].(string)
	return s
}

// system returns whether a walk includes what Impala keeps for itself.
func system(args map[string]any) bool {
	b, _ := args["with_system"].(bool)
	return b
}

// database is one row of SHOW DATABASES.
type database struct {
	name    string
	comment sql.Null[string]
}

// databases lists the databases that match the schema pattern in param.
func databases(ctx context.Context, db dbmeta.Queryer, args map[string]any, param string) ([]database, error) {
	rows, err := readAll(ctx, db, `SHOW DATABASES`)
	if err != nil {
		return nil, err
	}
	var out []database
	for _, r := range rows {
		if r[0] == builtins && !system(args) {
			continue
		}
		if !dbmeta.Like(arg(args, param), r[0]) {
			continue
		}
		d := database{name: r[0]}
		if len(r) > 1 && r[1] != "" {
			d.comment = sql.Null[string]{V: r[1], Valid: true}
		}
		out = append(out, d)
	}
	return out, nil
}

// relation is one table or view of a database.
type relation struct {
	schema, name string
	view         bool
}

// relations lists the tables and views of every matching database whose
// name matches the pattern in param.
func relations(ctx context.Context, db dbmeta.Queryer, args map[string]any, param string) ([]relation, error) {
	dbs, err := databases(ctx, db, args, "schema")
	if err != nil {
		return nil, err
	}
	var out []relation
	for _, d := range dbs {
		views, err := readAll(ctx, db, `SHOW VIEWS IN `+quote(d.name))
		if err != nil {
			return nil, err
		}
		isView := make(map[string]bool, len(views))
		for _, v := range views {
			isView[v[0]] = true
		}
		tables, err := readAll(ctx, db, `SHOW TABLES IN `+quote(d.name))
		if err != nil {
			return nil, err
		}
		for _, t := range tables {
			if dbmeta.Like(arg(args, param), t[0]) {
				out = append(out, relation{schema: d.name, name: t[0], view: isView[t[0]]})
			}
		}
	}
	return out, nil
}

// kind is the word for a relation's kind.
func (r relation) kind() string {
	if r.view {
		return "view"
	}
	return "table"
}
