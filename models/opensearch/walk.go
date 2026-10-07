package opensearch

import (
	"context"
	"database/sql"
	"fmt"
	"iter"
	"strconv"
	"strings"

	"github.com/xo/dbmeta"
)

// arg returns a text argument of a walk, which has every parameter filled.
func arg(args map[string]any, name string) string {
	s, _ := args[name].(string)
	return s
}

// withSystem returns whether a walk includes what OpenSearch builds in.
func withSystem(args map[string]any) bool {
	b, _ := args["with_system"].(bool)
	return b
}

// visible returns true when a walk keeps the index called name. A name that
// starts with a dot is a hidden index, such as .opendistro_security, and a walk
// keeps it only for with_system.
func visible(args map[string]any, name string) bool {
	return withSystem(args) || !strings.HasPrefix(name, ".")
}

// oneOf reports whether list, which is a list of words joined by commas, is
// empty or names word.
func oneOf(list, word string) bool {
	if list == "" {
		return true
	}
	for w := range strings.SplitSeq(list, ",") {
		if w == word {
			return true
		}
	}
	return false
}

// showTable is one row of SHOW TABLES LIKE.
type showTable struct {
	catalog, name, typ string
}

// showTables yields the rows of SHOW TABLES LIKE % that keep says to keep.
// The statement takes a pattern, and its underscore is a wildcard with no
// escape, so the walk asks for every index and matches the name in Go. The
// answer is one request and holds no document. The rows close when the
// sequence ends, when the caller stops, and when the context ends.
func showTables(ctx context.Context, db dbmeta.Queryer, keep func(showTable) bool) iter.Seq2[showTable, error] {
	const query = `SHOW TABLES LIKE %`
	return func(yield func(showTable, error) bool) {
		rows, err := db.QueryContext(ctx, query)
		if err != nil {
			yield(showTable{}, fmt.Errorf("running %s: %w", query, err))
			return
		}
		defer rows.Close()
		for rows.Next() {
			var (
				t      showTable
				schema any
				unused [6]any
			)
			if err := rows.Scan(&t.catalog, &schema, &t.name, &t.typ,
				&unused[0], &unused[1], &unused[2], &unused[3], &unused[4], &unused[5]); err != nil {
				yield(showTable{}, fmt.Errorf("reading a row of %s: %w", query, err))
				return
			}
			if keep(t) && !yield(t, nil) {
				return
			}
		}
		if err := rows.Err(); err != nil {
			yield(showTable{}, fmt.Errorf("reading %s: %w", query, err))
		}
	}
}

// describeColumns is how many columns DESCRIBE TABLES LIKE returns, which are
// the 24 of JDBC's getColumns.
const describeColumns = 24

// columns yields the columns of the indices that SHOW TABLES lists, which
// match the arguments. It reads the names of the indices to the end first, so
// that it holds one connection at a time, and then runs DESCRIBE TABLES LIKE
// for each index with the exact name. A pattern in DESCRIBE merges every index
// that matches into one table named for the pattern, so it cannot say which
// index owns a column.
//
// The cost is one statement for the list and one for each index that matches.
// The server reads the mappings in the cluster state, and never a document.
func columns(ctx context.Context, db dbmeta.Queryer, args map[string]any) iter.Seq2[dbmeta.Column, error] {
	return func(yield func(dbmeta.Column, error) bool) {
		var names []showTable
		for t, err := range showTables(ctx, db, func(t showTable) bool {
			return visible(args, t.name) && dbmeta.Like(arg(args, "parent"), t.name)
		}) {
			if err != nil {
				yield(dbmeta.Column{}, err)
				return
			}
			names = append(names, t)
		}
		if !dbmeta.Like(arg(args, "schema"), "") {
			return
		}
		for _, t := range names {
			if !describe(ctx, db, t.name, args, yield) {
				return
			}
		}
	}
}

// describe yields the columns of one index, and returns false when the walk
// must stop: the caller stopped, or a row failed, or the context ended. An
// index that the server refuses to describe has no column, and describe goes on
// to the next one.
func describe(ctx context.Context, db dbmeta.Queryer, index string, args map[string]any,
	yield func(dbmeta.Column, error) bool,
) bool {
	query := `DESCRIBE TABLES LIKE ` + dbmeta.QuoteLiteral(index, dbmeta.Quoting{})
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		if ctx.Err() != nil {
			yield(dbmeta.Column{}, fmt.Errorf("running %s: %w", query, err))
			return false
		}
		return true
	}
	defer rows.Close()
	for rows.Next() {
		c, err := scanColumn(rows)
		if err != nil {
			yield(dbmeta.Column{}, fmt.Errorf("reading a row of %s: %w", query, err))
			return false
		}
		if c.Table != index || !dbmeta.Like(arg(args, "name"), c.Name) {
			continue
		}
		if !yield(c, nil) {
			return false
		}
	}
	if err := rows.Err(); err != nil {
		yield(dbmeta.Column{}, fmt.Errorf("reading %s: %w", query, err))
		return false
	}
	return true
}

// scanColumn reads one row of DESCRIBE TABLES LIKE. The positions are those of
// JDBC's getColumns: 1 TABLE_CAT, 2 TABLE_SCHEM, 3 TABLE_NAME, 4 COLUMN_NAME,
// 6 TYPE_NAME, 11 NULLABLE, 12 REMARKS, 13 COLUMN_DEF and 17 ORDINAL_POSITION.
// The rest are NULL or repeat what these say. The types of the numbers differ
// between releases, so they are read as they come and converted.
func scanColumn(rows *sql.Rows) (dbmeta.Column, error) {
	var (
		c        dbmeta.Column
		ordinal  any
		remarks  sql.Null[string]
		def      sql.Null[string]
		nullable any
	)
	dest := make([]any, describeColumns)
	for i := range dest {
		dest[i] = new(any)
	}
	dest[0], dest[2], dest[3], dest[5] = &c.Catalog, &c.Table, &c.Name, &c.DataType
	dest[10], dest[11], dest[12], dest[16] = &nullable, &remarks, &def, &ordinal
	if err := rows.Scan(dest...); err != nil {
		return dbmeta.Column{}, err
	}
	n, err := number(ordinal)
	if err != nil {
		return dbmeta.Column{}, fmt.Errorf("reading ORDINAL_POSITION: %w", err)
	}
	c.Ordinal = n
	// NULLABLE 2 is unknown. A document can leave any field out, so a field
	// is nullable, as Elasticsearch reports it.
	c.Nullable = true
	c.Default = def
	c.Comment = remarks
	c.Identity = sql.Null[string]{Valid: true}
	c.Generated = sql.Null[string]{Valid: true}
	return c, nil
}

// number reads a number that a release sends as a number or as text.
func number(v any) (int, error) {
	switch t := v.(type) {
	case int64:
		return int(t), nil
	case int:
		return t, nil
	case string:
		return strconv.Atoi(t)
	case nil:
		return 0, nil
	}
	return 0, fmt.Errorf("unexpected type %T", v)
}
