package elasticsearch

import (
	"context"
	"database/sql"
	"fmt"
	"iter"
	"strings"

	"github.com/xo/dbmeta"
)

// readAll runs one statement and yields what scan returns for each row. The
// scan function says whether the caller keeps the row, because a statement of
// Elasticsearch takes no filter and the caller's patterns are matched here.
//
// The rows close when the sequence ends, when the caller stops, and when the
// context ends, so a walk holds one connection until then. The driver reads
// the next page with the cursor of the statement, and closes the cursor on the
// server when the rows close before the last page.
func readAll[T any](ctx context.Context, db dbmeta.Queryer, query string,
	scan func(rows *sql.Rows) (T, bool, error),
) iter.Seq2[T, error] {
	return func(yield func(T, error) bool) {
		var zero T
		rows, err := db.QueryContext(ctx, query)
		if err != nil {
			yield(zero, fmt.Errorf("running %s: %w", query, err))
			return
		}
		defer rows.Close()
		for rows.Next() {
			v, keep, err := scan(rows)
			if err != nil {
				yield(zero, fmt.Errorf("reading a row of %s: %w", query, err))
				return
			}
			if keep && !yield(v, nil) {
				return
			}
		}
		if err := rows.Err(); err != nil {
			yield(zero, fmt.Errorf("reading %s: %w", query, err))
		}
	}
}

// arg returns a text argument of a walk, which has every parameter filled.
func arg(args map[string]any, name string) string {
	s, _ := args[name].(string)
	return s
}

// withSystem returns whether a walk includes what Elasticsearch builds in.
func withSystem(args map[string]any) bool {
	b, _ := args["with_system"].(bool)
	return b
}

// visible returns true when a walk keeps the index called name. A name that
// starts with a dot is a hidden index, and a walk keeps it only for with_system.
func visible(args map[string]any, name string) bool {
	return withSystem(args) || !strings.HasPrefix(name, ".")
}
