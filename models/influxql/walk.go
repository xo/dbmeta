package influxql

import (
	"context"
	"database/sql"
	"fmt"
	"iter"
	"slices"

	"github.com/xo/dbmeta"
)

// system are the databases that InfluxDB keeps for itself: _internal, where
// InfluxDB 1 and 3 keep their own statistics, and _monitoring and _tasks,
// the buckets of InfluxDB 2. A user cannot make a bucket whose name starts
// with an underscore on InfluxDB 2.
var system = []string{"_internal", "_monitoring", "_tasks"}

// series is one series of an answer: its name, its columns and its rows.
//
// The driver gives each series of a statement a result set of its own. Its
// first column is measurement, which holds the name of the series, and the
// columns of the series follow (dbimp D81).
type series struct {
	name string
	cols []string
	rows [][]sql.Null[string]
}

// value returns the value of the column named col in row i, or an absent
// value where the series has no such column.
func (s series) value(i int, col string) sql.Null[string] {
	j := slices.Index(s.cols, col)
	if j < 0 {
		return sql.Null[string]{}
	}
	return s.rows[i][j]
}

// readAll reads every series of every result set of query as text, closing
// the rows before it returns, so that a walk holds one connection at a time.
// A series with no rows, and a statement with no series, give a series with
// no rows.
func readAll(ctx context.Context, db dbmeta.Queryer, query string) ([]series, error) {
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("running %s: %w", query, err)
	}
	defer rows.Close()
	var out []series
	for {
		s, err := readSet(rows)
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", query, err)
		}
		out = append(out, s)
		if !rows.NextResultSet() {
			break
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("reading %s: %w", query, err)
	}
	return out, nil
}

// readSet reads the current result set.
func readSet(rows *sql.Rows) (series, error) {
	cols, err := rows.Columns()
	if err != nil {
		return series{}, fmt.Errorf("reading the columns: %w", err)
	}
	s := series{cols: cols}
	for rows.Next() {
		vals := make([]sql.Null[string], len(cols))
		dest := make([]any, len(cols))
		for i := range vals {
			dest[i] = &vals[i]
		}
		if err := rows.Scan(dest...); err != nil {
			return series{}, fmt.Errorf("reading a row: %w", err)
		}
		s.rows = append(s.rows, vals)
	}
	if err := rows.Err(); err != nil {
		return series{}, err
	}
	if len(s.rows) > 0 {
		s.name = s.value(0, "measurement").V
	}
	return s, nil
}

// arg returns a text argument of a walk, which has every parameter filled.
func arg(args map[string]any, name string) string {
	s, _ := args[name].(string)
	return s
}

// withSystem returns whether a walk includes what InfluxDB keeps for itself.
func withSystem(args map[string]any) bool {
	b, _ := args["with_system"].(bool)
	return b
}

// databases lists the databases that match the pattern in param. It is one
// statement, SHOW DATABASES, which lists only the databases the user can
// read.
func databases(ctx context.Context, db dbmeta.Queryer, args map[string]any, param string) ([]string, error) {
	sets, err := readAll(ctx, db, `SHOW DATABASES`)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, s := range sets {
		for i := range s.rows {
			name := s.value(i, "name").V
			if !withSystem(args) && slices.Contains(system, name) {
				continue
			}
			if dbmeta.Like(arg(args, param), name) {
				out = append(out, name)
			}
		}
	}
	return out, nil
}

// measurement is one measurement of a database.
type measurement struct {
	schema, name string
}

// measurements lists the measurements of every matching database whose
// name matches the pattern in param. It is one statement, and one more for
// each database.
func measurements(ctx context.Context, db dbmeta.Queryer, args map[string]any, param string) ([]measurement, error) {
	dbs, err := databases(ctx, db, args, "schema")
	if err != nil {
		return nil, err
	}
	var out []measurement
	for _, d := range dbs {
		sets, err := readAll(ctx, db, `SHOW MEASUREMENTS ON `+quote(d))
		if err != nil {
			return nil, err
		}
		for _, s := range sets {
			for i := range s.rows {
				name := s.value(i, "name").V
				if dbmeta.Like(arg(args, param), name) {
					out = append(out, measurement{schema: d, name: name})
				}
			}
		}
	}
	return out, nil
}

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
