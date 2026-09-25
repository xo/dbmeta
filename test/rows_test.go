package test

import (
	"database/sql"
	"testing"
)

// Helpers that run a statement and close its rows properly. A defer belongs in
// a function of its own, because every caller here runs one statement per
// query in a loop over 48 of them, and a defer in that loop would hold 48
// result sets open until the test ended.

// columnsOf runs the statement and returns the columns it returned.
func columnsOf(t *testing.T, db *sql.DB, query string, vals []any) ([]string, error) {
	t.Helper()
	rows, err := db.QueryContext(t.Context(), query, vals...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return cols, rows.Close()
}

// eachRawRow runs the statement and calls fn once per row, with every column
// read as raw bytes so that a NULL is visible as one.
func eachRawRow(t *testing.T, db *sql.DB, query string, vals []any, width int, fn func([]sql.RawBytes)) error {
	t.Helper()
	rows, err := db.QueryContext(t.Context(), query, vals...)
	if err != nil {
		return err
	}
	defer rows.Close()
	raw := make([]sql.RawBytes, width)
	dest := make([]any, width)
	for i := range dest {
		dest[i] = &raw[i]
	}
	for rows.Next() {
		if err := rows.Scan(dest...); err != nil {
			return err
		}
		fn(raw)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	return rows.Close()
}

// nullableStrings reads every column of every row as a nullable string, which
// is how the cross product comparison reads an answer it does not interpret.
func nullableStrings(t *testing.T, db *sql.DB, query string, vals []any) ([]string, [][]sql.Null[string], error) {
	t.Helper()
	rows, err := db.QueryContext(t.Context(), query, vals...)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		return nil, nil, err
	}
	var out [][]sql.Null[string]
	for rows.Next() {
		cells := make([]sql.Null[string], len(cols))
		dest := make([]any, len(cols))
		for i := range cells {
			dest[i] = &cells[i]
		}
		if err := rows.Scan(dest...); err != nil {
			return nil, nil, err
		}
		out = append(out, cells)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	return cols, out, rows.Close()
}
