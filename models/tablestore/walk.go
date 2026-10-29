package tablestore

import (
	"context"
	"database/sql"
	"fmt"
	"iter"
	"slices"
	"strconv"

	"github.com/xo/dbmeta"
)

// result is every row of one statement, read as text, with the names of its
// columns.
type result struct {
	cols []string
	rows [][]sql.Null[string]
}

// positions returns where each named column is in a row. A column that the
// service does not return is an error, because the statement changed.
func (r result) positions(names ...string) ([]int, error) {
	out := make([]int, len(names))
	for i, name := range names {
		at := slices.Index(r.cols, name)
		if at < 0 {
			return nil, fmt.Errorf("the statement returns no column %s, only %v", name, r.cols)
		}
		out[i] = at
	}
	return out, nil
}

// readAll reads every row of query as text, closing the rows before it
// returns, so that a walk holds one connection at a time. A NULL stays a NULL.
func readAll(ctx context.Context, db dbmeta.Queryer, query string) (result, error) {
	var out result
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return out, fmt.Errorf("running %s: %w", query, err)
	}
	defer rows.Close()
	if out.cols, err = rows.Columns(); err != nil {
		return out, fmt.Errorf("reading the columns of %s: %w", query, err)
	}
	for rows.Next() {
		vals := make([]sql.Null[string], len(out.cols))
		dest := make([]any, len(out.cols))
		for i := range vals {
			dest[i] = &vals[i]
		}
		if err := rows.Scan(dest...); err != nil {
			return out, fmt.Errorf("reading a row of %s: %w", query, err)
		}
		out.rows = append(out.rows, vals)
	}
	if err := rows.Err(); err != nil {
		return out, fmt.Errorf("reading %s: %w", query, err)
	}
	return out, nil
}

// quote returns a table name between backticks, with a backtick in the name
// doubled, as the front end reads a name.
func quote(name string) string {
	return dbmeta.QuoteIdentifier(name, "`", "`")
}

// listTables reads the names of the mapped tables with SHOW FULL TABLES.
func listTables(ctx context.Context, db dbmeta.Queryer) ([]string, error) {
	res, err := readAll(ctx, db, `SHOW FULL TABLES`)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(res.rows))
	for _, r := range res.rows {
		if len(r) > 0 && r[0].Valid {
			out = append(out, r[0].V)
		}
	}
	return out, nil
}

// vanished reports whether the table is no longer listed. A walk calls it when
// a statement about the table failed, and skips the table only if it is gone.
// If the list fails too, the table is not known to be gone, so it is not
// skipped.
func vanished(ctx context.Context, db dbmeta.Queryer, name string) bool {
	names, err := listTables(ctx, db)
	return err == nil && !slices.Contains(names, name)
}

// walkTables answers a kind that the service reads for one table at a time. It
// runs SHOW FULL TABLES, and then stmt for each table whose name matches the
// parent pattern, and read turns the rows of that statement into values. Each
// statement runs to its end before the next.
//
// A table can be dropped between the list and its statement, and the
// statement then answers an error. The walk lists the tables again. If the
// table is gone, the walk skips it. Any other error ends the walk and
// reaches the caller, so a service that is down does not look like an empty
// instance.
func walkTables[T any](stmt string, read func(table string, res result) ([]T, error)) func(context.Context, dbmeta.Queryer, map[string]any) iter.Seq2[T, error] {
	return func(ctx context.Context, db dbmeta.Queryer, args map[string]any) iter.Seq2[T, error] {
		return func(yield func(T, error) bool) {
			var zero T
			// A table has no schema, so only a pattern that matches the empty
			// text selects anything.
			if !dbmeta.Like(arg(args, "schema"), "") {
				return
			}
			names, err := listTables(ctx, db)
			if err != nil {
				yield(zero, err)
				return
			}
			for _, name := range names {
				if !dbmeta.Like(arg(args, "parent"), name) {
					continue
				}
				res, err := readAll(ctx, db, stmt+" "+quote(name))
				if err != nil {
					if vanished(ctx, db, name) {
						continue
					}
					yield(zero, err)
					return
				}
				vals, err := read(name, res)
				if err != nil {
					yield(zero, err)
					return
				}
				for _, v := range vals {
					if !yield(v, nil) {
						return
					}
				}
			}
		}
	}
}

// walkParams are the parameters of a kind that belongs to a table.
func walkParams(kind string) []dbmeta.Param {
	return []dbmeta.Param{
		{Name: "schema", Desc: "schema name pattern. A table has no schema, so only an empty pattern, or one that matches the empty text, selects a row", Default: ""},
		{Name: "parent", Desc: "table name pattern, empty for every table", Default: ""},
		{Name: "name", Desc: kind + " name pattern, empty for every " + kind, Default: ""},
	}
}

// text returns the value of a column, and absent for a NULL or an empty text.
// The service writes an empty text where MySQL writes a NULL, and what it
// writes is the fact, so an empty text is a value that is not there.
func text(v sql.Null[string]) sql.Null[string] {
	if !v.Valid || v.V == "" {
		return sql.Null[string]{}
	}
	return v
}

func registerWalks() {
	// \d NAME. DESCRIBE names one table, so a walk costs the statement that lists
	// the tables and one DESCRIBE for each table that the parent pattern keeps.
	dbmeta.Columns.Register(dbmeta.Tablestore, &dbmeta.Binding[dbmeta.Column]{
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: "always empty: the instance is the only namespace"},
			{Name: "schema", Desc: "always empty: a table has no schema"},
			{Name: "table"},
			{Name: "name", Desc: "the column, as DESCRIBE names it"},
			{Name: "ordinal", Desc: "the position in DESCRIBE, from 1, which is the order of the CREATE TABLE that mapped the table"},
			{Name: "data_type", Desc: "the type as DESCRIBE writes it, such as bigint(20), mediumtext or varchar(1024)"},
			{Name: "nullable", Desc: "true where DESCRIBE says YES, which every column does but a key column"},
			{Name: "default", Desc: "always absent: DESCRIBE writes an empty text, and SHOW CREATE TABLE writes DEFAULT NULL for every column"},
			{Name: "primary_key", Desc: "true where DESCRIBE says PRI in the Key column"},
			{Name: "identity", Desc: "always absent: no column of the fixture had a value in Extra, so the model reads nothing from it"},
			{Name: "generated", Desc: "always absent: Tablestore SQL has no generated column"},
			{Name: "comment", Desc: "always absent: Tablestore SQL has no comment"},
			{Name: "collation", Desc: "always absent: only SHOW CREATE TABLE names one, for the whole table, and it is a second statement"},
		},
		Params: walkParams("column"),
		Walk: func(ctx context.Context, db dbmeta.Queryer, args map[string]any) iter.Seq2[dbmeta.Column, error] {
			name := arg(args, "name")
			return walkTables(`DESCRIBE`, func(table string, res result) ([]dbmeta.Column, error) {
				return columns(table, name, res)
			})(ctx, db, args)
		},
	})

	// SHOW INDEX names one table, and it lists the key, each secondary index and
	// each search index of it. A walk costs the statement that lists the tables
	// and one SHOW INDEX for each table that the parent pattern keeps.
	dbmeta.Indexes.Register(dbmeta.Tablestore, &dbmeta.Binding[dbmeta.Index]{
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: "always empty: the instance is the only namespace"},
			{Name: "schema", Desc: "always empty: a table has no schema"},
			{Name: "table"},
			{Name: "name", Desc: "PRIMARY for the key of the table, and the name of the index for a secondary index or a search index"},
			{Name: "type", Desc: "the Index_type of SHOW INDEX: GlobalIndex for a secondary index, SearchIndex for a search index and empty for the key"},
			{Name: "unique", Desc: "true where Non_unique is 0, which the key alone has"},
			{Name: "primary", Desc: "true for the key of the table"},
			{Name: "comment", Desc: "always absent: Tablestore SQL has no comment"},
		},
		Params: walkParams("index"),
		Walk: func(ctx context.Context, db dbmeta.Queryer, args map[string]any) iter.Seq2[dbmeta.Index, error] {
			name := arg(args, "name")
			return walkTables(`SHOW INDEX FROM`, func(table string, res result) ([]dbmeta.Index, error) {
				return indexes(table, name, res)
			})(ctx, db, args)
		},
	})

	// The columns of an index come from the same SHOW INDEX, so a walk costs the
	// same as the walk of the indexes.
	dbmeta.IndexColumns.Register(dbmeta.Tablestore, &dbmeta.Binding[dbmeta.IndexColumn]{
		Fields: []dbmeta.Field{
			{Name: "schema", Desc: "always empty: a table has no schema"},
			{Name: "table"},
			{Name: "index", Desc: "the index, as the Indexes kind names it, PRIMARY for the key"},
			{Name: "name", Desc: "the column, as Column_name of SHOW INDEX names it"},
			{Name: "ordinal", Desc: "Seq_in_index, from 1. A secondary index lists the columns that it was made with and then the key of the table, which Tablestore adds"},
			{Name: "expression", Desc: "always absent: SHOW INDEX writes the text NULL for it in every row, and Tablestore has no index on an expression"},
			{Name: "descending", Desc: "false where SHOW INDEX says A in Collation, which it does for every row, and absent for any other value"},
		},
		Params: walkParams("index"),
		Walk: func(ctx context.Context, db dbmeta.Queryer, args map[string]any) iter.Seq2[dbmeta.IndexColumn, error] {
			name := arg(args, "name")
			return walkTables(`SHOW INDEX FROM`, func(table string, res result) ([]dbmeta.IndexColumn, error) {
				return indexColumns(table, name, res)
			})(ctx, db, args)
		},
	})
}

// columns turns the rows of DESCRIBE into columns whose name matches the
// pattern.
func columns(table, pattern string, res result) ([]dbmeta.Column, error) {
	at, err := res.positions("Field", "Type", "Null", "Key", "Default")
	if err != nil {
		return nil, fmt.Errorf("reading DESCRIBE of %s: %w", table, err)
	}
	var out []dbmeta.Column
	for i, r := range res.rows {
		if !dbmeta.Like(pattern, r[at[0]].V) {
			continue
		}
		out = append(out, dbmeta.Column{
			Table:      table,
			Name:       r[at[0]].V,
			Ordinal:    i + 1,
			DataType:   r[at[1]].V,
			Nullable:   r[at[2]].V == "YES",
			Default:    text(r[at[4]]),
			PrimaryKey: r[at[3]].V == "PRI",
		})
	}
	return out, nil
}

// indexes turns the rows of SHOW INDEX into one index for each name, in the
// order that the names first appear, for the names that match the pattern.
func indexes(table, pattern string, res result) ([]dbmeta.Index, error) {
	at, err := res.positions("Non_unique", "Key_name", "Index_type")
	if err != nil {
		return nil, fmt.Errorf("reading SHOW INDEX of %s: %w", table, err)
	}
	var (
		out  []dbmeta.Index
		seen = make(map[string]bool)
	)
	for _, r := range res.rows {
		name := r[at[1]].V
		if seen[name] || !dbmeta.Like(pattern, name) {
			continue
		}
		seen[name] = true
		out = append(out, dbmeta.Index{
			Table:   table,
			Name:    name,
			Type:    r[at[2]].V,
			Unique:  r[at[0]].V == "0",
			Primary: name == "PRIMARY",
		})
	}
	return out, nil
}

// indexColumns turns the rows of SHOW INDEX into one value for each row whose
// index name matches the pattern.
func indexColumns(table, pattern string, res result) ([]dbmeta.IndexColumn, error) {
	at, err := res.positions("Key_name", "Seq_in_index", "Column_name", "Collation", "Expression")
	if err != nil {
		return nil, fmt.Errorf("reading SHOW INDEX of %s: %w", table, err)
	}
	var out []dbmeta.IndexColumn
	for _, r := range res.rows {
		if !dbmeta.Like(pattern, r[at[0]].V) {
			continue
		}
		seq, err := strconv.ParseInt(r[at[1]].V, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("reading Seq_in_index of %s in %s: %w", r[at[0]].V, table, err)
		}
		v := dbmeta.IndexColumn{
			Table:   table,
			Index:   r[at[0]].V,
			Name:    text(r[at[2]]),
			Ordinal: seq,
		}
		switch r[at[3]].V {
		case "A":
			v.Descending = sql.Null[bool]{V: false, Valid: true}
		case "D":
			v.Descending = sql.Null[bool]{V: true, Valid: true}
		}
		// The service writes the text NULL when there is no expression.
		if e := text(r[at[4]]); e.V != "NULL" {
			v.Expression = e
		}
		out = append(out, v)
	}
	return out, nil
}
