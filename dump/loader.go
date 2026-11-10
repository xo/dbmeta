package dump

import (
	"encoding/hex"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/xo/dbmeta"
)

// defaultBatch is the rows of one INSERT when the product names no limit.
const defaultBatch = 500

// Loader writes the statements that move the rows of one table into the target.
// \dump calls Insert and writes the text to a file, and \copy calls InsertArgs and
// binds each row, so both pick the same columns and the same types from one
// plan (D274). A Loader builds statements and runs nothing.
type Loader struct {
	source, target dbmeta.Dialect
	src, dst       Style
	schema, table  string
	cols           []loadColumn
	key            []string
	override       bool
	identity       bool
	snap           *Snapshot
	// selfRef is set for a table that has a foreign key to itself.
	selfRef bool
}

// loadColumn is a column that the loader moves.
type loadColumn struct {
	name     string
	from, to Type
	col      Column
}

// NewLoader prepares the loader of one table. The plan decides the type of each
// column, so the loader fails with a [*PlanError] when a column has no allowed
// type, and a client that has made the plan already gets the same answer.
//
// A generated column is left out when the target is the source, because the
// product computes it. For another dialect it is a plain column, and its values
// are moved.
func NewLoader(s *Snapshot, schema, table string, target dbmeta.Dialect, p Policy) (*Loader, error) {
	source := dbmeta.Dialect(s.Dialect)
	src, err := styleOf(source)
	if err != nil {
		return nil, err
	}
	dst, err := styleOf(target)
	if err != nil {
		return nil, err
	}
	plan, err := MakePlan(s, target, p)
	if err != nil {
		return nil, err
	}
	byName := map[string]ColumnPlan{}
	for _, c := range plan.Columns {
		byName[c.Name] = c
	}
	var t *Table
	for i := range s.Schemas {
		if s.Schemas[i].Name != schema {
			continue
		}
		for j := range s.Schemas[i].Tables {
			if s.Schemas[i].Tables[j].Name == table {
				t = &s.Schemas[i].Tables[j]
			}
		}
	}
	if t == nil {
		return nil, fmt.Errorf("the snapshot has no table %s.%s: %w", schema, table, dbmeta.ErrNotFound)
	}
	l := &Loader{source: source, target: target, src: src, dst: dst, schema: schema, table: table, snap: s}
	for _, c := range t.Constraints {
		if c.Kind == "foreign key" && c.Ref != nil && c.Ref.Table == table && (c.Ref.Schema == "" || c.Ref.Schema == schema) {
			l.selfRef = true
		}
	}
	for _, c := range t.Columns {
		if computedColumn(c) && source == target {
			continue
		}
		cp := byName[schema+"."+table+"."+c.Name]
		l.cols = append(l.cols, loadColumn{name: c.Name, from: c.Type, to: cp.Type, col: c})
		if c.Identity != "" {
			l.identity = true
		}
		if source == target && (strings.EqualFold(c.Identity, "a") || strings.EqualFold(c.Identity, "always")) {
			l.override = true
		}
	}
	for _, c := range t.Constraints {
		if c.Kind == "primary key" {
			l.key = c.Columns
		}
	}
	if len(l.cols) == 0 {
		return nil, fmt.Errorf("the table %s.%s has no column to move: %w", schema, table, dbmeta.ErrNotFound)
	}
	return l, nil
}

// Prologue returns the statement that the client runs before the INSERT of this
// table, or the empty string. SQL Server needs SET IDENTITY_INSERT for a table
// with an identity column, and the statement holds for the connection that runs
// it, so the client runs the Prologue, the inserts and the Epilogue on one
// connection.
func (l *Loader) Prologue() string {
	if l.identity && l.dst.IdentityInsert != nil {
		return l.dst.IdentityInsert(l.qualify(l.dst), true)
	}
	return ""
}

// Epilogue returns the statement that ends the Prologue, or the empty string.
func (l *Loader) Epilogue() string {
	if l.identity && l.dst.IdentityInsert != nil {
		return l.dst.IdentityInsert(l.qualify(l.dst), false)
	}
	return ""
}

// SourceSession returns the statements that the client runs on the connection of
// the source before it runs SelectAll, so that a value is read in the zone that the
// loader assumes. It is empty for a product whose values do not depend on the session.
func (l *Loader) SourceSession() []string { return l.src.Session }

// TargetSession returns the statements that the client runs on the connection of
// the target before it runs an INSERT. A script holds the same statements in its
// preamble.
func (l *Loader) TargetSession() []string { return l.dst.Session }

// Columns returns the names of the columns that the loader moves, in order.
func (l *Loader) Columns() []string {
	out := make([]string, len(l.cols))
	for i, c := range l.cols {
		out[i] = c.name
	}
	return out
}

// Ordered says that SelectAll orders the rows by the primary key, so that a dump
// of the same table is the same twice. A table with no key has the order that the
// source gives, and a dump of it is not reproducible.
func (l *Loader) Ordered() bool { return len(l.key) > 0 }

// SelectAll writes the statement that reads the rows from the source, with the
// quoted names of the source and the order of the primary key. The client runs it
// and passes each row to Insert or Args. dbmeta reads no row itself.
func (l *Loader) SelectAll() string {
	cols := make([]string, len(l.cols))
	alias := ""
	for i, c := range l.cols {
		cols[i] = l.src.Quote(c.name)
		if l.src.SelectColumn != nil {
			// The table has an alias when a column needs one to name its own table.
			if text, ok := l.src.SelectColumn(l.snap, l.schema, c.col, "t."+cols[i]); ok {
				cols[i] = text + " AS " + cols[i]
				alias = " t"
			}
		}
	}
	if alias != "" {
		for i, c := range l.cols {
			if !strings.Contains(cols[i], " AS ") {
				cols[i] = "t." + l.src.Quote(c.name)
			}
		}
	}
	sql := "SELECT " + strings.Join(cols, ", ") + " FROM " + l.qualifySource() + alias
	if len(l.key) > 0 {
		key := make([]string, len(l.key))
		for i, name := range l.key {
			key[i] = l.src.Quote(name)
		}
		sql += " ORDER BY " + strings.Join(key, ", ")
	}
	return sql
}

// BatchSize is the most rows that one INSERT takes: the limit of the product, and
// the number of parameters that it takes divided by the columns.
func (l *Loader) BatchSize() int {
	// DuckDB checks each row of a statement against the rows that come before it in
	// the same statement, so a table that refers to itself loads a row at a time.
	if l.selfRef && l.dst.RowPerSelfReference {
		return 1
	}
	n := l.dst.MaxRows
	if n == 0 {
		n = defaultBatch
	}
	if l.dst.MaxParameters > 0 {
		n = min(n, max(1, l.dst.MaxParameters/len(l.cols)))
	}
	return n
}

// Insert writes one INSERT of the rows, with every value as a literal. A row has
// a value for each column that Columns names, in order.
func (l *Loader) Insert(rows [][]any) (string, error) {
	if len(rows) == 0 {
		return "", fmt.Errorf("writing an INSERT of no rows: %w", ErrBadValue)
	}
	groups := make([]string, len(rows))
	vars := &variables{}
	for i, row := range rows {
		if len(row) != len(l.cols) {
			return "", fmt.Errorf("row %d has %d values and the table has %d columns: %w", i+1, len(row), len(l.cols), ErrBadValue)
		}
		vals := make([]string, len(row))
		for j, v := range row {
			text, err := l.literal(l.cols[j], v, vars)
			if err != nil {
				return "", fmt.Errorf("the column %s.%s.%s of row %d: %w", l.schema, l.table, l.cols[j].name, i+1, err)
			}
			vals[j] = text
		}
		groups[i] = "(" + strings.Join(vals, ", ") + ")"
	}
	stmt := l.head() + strings.Join(groups, ", ")
	if len(vars.declare) > 0 {
		return l.dst.Block(vars.declare, vars.fill, stmt), nil
	}
	return stmt, nil
}

// variables are the variables that the values of one INSERT need, for a value
// that no literal holds.
type variables struct {
	declare, fill []string
}

// InsertArgs writes an INSERT of n rows with the placeholders of the target, for
// the values that Args returns.
func (l *Loader) InsertArgs(n int) string {
	groups := make([]string, n)
	next := 1
	for i := range groups {
		marks := make([]string, len(l.cols))
		for j := range marks {
			marks[j], _ = l.target.Placeholder(next)
			if l.dst.CastBinary != nil {
				if name := l.dst.CastBinary(l.cols[j].to); name != "" {
					marks[j] = "CAST(" + marks[j] + " AS " + name + ")"
				}
			}
			next++
		}
		groups[i] = "(" + strings.Join(marks, ", ") + ")"
	}
	return l.head() + strings.Join(groups, ", ")
}

// Args converts the rows to the values that a driver of the target binds, in the
// order of the placeholders of InsertArgs.
func (l *Loader) Args(rows [][]any) ([]any, error) {
	out := make([]any, 0, len(rows)*len(l.cols))
	for i, row := range rows {
		if len(row) != len(l.cols) {
			return nil, fmt.Errorf("row %d has %d values and the table has %d columns: %w", i+1, len(row), len(l.cols), ErrBadValue)
		}
		for j, v := range row {
			c := l.cols[j]
			if l.dst.ExpressionColumn != nil && l.dst.ExpressionColumn(c.to) && v != nil {
				return nil, fmt.Errorf("the column %s.%s.%s holds the text of an expression, which a placeholder cannot take: %w",
					l.schema, l.table, c.name, ErrNoEquivalent)
			}
			conv, err := Convert(l.sourceValue(c, v), c.from, c.to)
			if err != nil {
				return nil, fmt.Errorf("the column %s.%s.%s of row %d: %w", l.schema, l.table, c.name, i+1, err)
			}
			conv = l.boundClock(c, conv)
			// A NULL of a binary column is a typed nil for a product whose driver sends
			// an untyped NULL as text, which the server refuses for a varbinary column.
			// Another driver can read a typed nil as an empty value, so it is not
			// the rule for every product.
			if conv == nil && c.to.Kind == Binary && l.dst.TypedNullBinary {
				conv = []byte(nil)
			}
			out = append(out, conv)
		}
	}
	return out, nil
}

// sessionClock gives the instant that a source with a session clock meant. The
// clock of the value is the UTC clock of the session, so it is read as UTC,
// whatever Location the driver gave it. See [Style.SessionClock].
func (l *Loader) sessionClock(c loadColumn, v any) any {
	t, ok := v.(time.Time)
	if !ok || !l.src.SessionClock || c.from.Kind != Timestamp {
		return v
	}
	y, m, d := t.Date()
	h, mi, s := t.Clock()
	return time.Date(y, m, d, h, mi, s, t.Nanosecond(), time.UTC)
}

// boundClock binds a date or a time for a target with a session clock as text,
// because a driver that takes a time.Time moves it to its own location first, and
// that changes the clock and sometimes the date. A timestamp is the text of the
// UTC clock. See [Style.SessionClock].
func (l *Loader) boundClock(c loadColumn, v any) any {
	t, ok := v.(time.Time)
	if !ok || !l.dst.SessionClock {
		return v
	}
	switch c.to.Kind {
	case Date:
		return t.Format("2006-01-02")
	case TimeOfDay:
		return t.Format("15:04:05.999999999")
	case LocalTimestamp:
		return t.Format("2006-01-02 15:04:05.999999999")
	case Timestamp:
		return t.UTC().Format("2006-01-02 15:04:05.999999999")
	}
	return v
}

// qualify writes the name of the table in a style.
func (l *Loader) qualify(s Style) string {
	g := &generator{style: s, snap: l.snap, cross: l.source != l.target}
	return g.qualified(l.schema, l.table)
}

// qualifySource writes the name of the table in the source. A product whose schema
// is its database reads a table of another database than the one that the
// connection has, so the name has its database. See D294.
func (l *Loader) qualifySource() string {
	if l.src.DefaultSchema == "*" && l.schema != "" && l.src.CreateSchema != nil {
		return l.src.Quote(l.schema) + "." + l.src.Quote(l.table)
	}
	return l.qualify(l.src)
}

// head writes INSERT INTO name (columns) and the clause for an identity column.
func (l *Loader) head() string {
	cols := make([]string, len(l.cols))
	for i, c := range l.cols {
		cols[i] = l.dst.Quote(c.name)
	}
	head := "INSERT INTO " + l.qualify(l.dst) + " (" + strings.Join(cols, ", ") + ")"
	if l.override && l.dst.OverridingIdentity != "" {
		head += " " + l.dst.OverridingIdentity
	}
	return head + " VALUES "
}

// literal writes one value as a literal of the target.
func (l *Loader) literal(c loadColumn, v any, vars *variables) (string, error) {
	if l.dst.ValueLiteral != nil && l.source == l.target {
		if text, ok := l.dst.ValueLiteral(c.from, v); ok {
			return text, nil
		}
	}
	conv, err := Convert(l.sourceValue(c, v), c.from, c.to)
	if err != nil {
		return "", err
	}
	switch x := conv.(type) {
	case nil:
		return "NULL", nil
	case bool:
		if l.dst.Boolean != nil {
			return l.dst.Boolean(x), nil
		}
		if x {
			return "TRUE", nil
		}
		return "FALSE", nil
	case int64:
		return strconv.FormatInt(x, 10), nil
	case float64:
		if math.IsNaN(x) || math.IsInf(x, 0) {
			if l.dst.FloatSpecial != nil {
				if text, ok := l.dst.FloatSpecial(x); ok {
					return text, nil
				}
			}
			return "", fmt.Errorf("%v has no literal in %s: %w", x, l.target, ErrBadValue)
		}
		return strconv.FormatFloat(x, 'g', -1, 64), nil
	case []byte:
		if l.dst.Binary == nil {
			return "", fmt.Errorf("%s has no binary literal: %w", l.target, ErrNoEquivalent)
		}
		if l.dst.LongBinary != nil && l.dst.Block != nil && len(x) > 2000 {
			declare, fill, ref := l.dst.LongBinary(len(vars.declare)+1, hex.EncodeToString(x))
			vars.declare = append(vars.declare, declare)
			vars.fill = append(vars.fill, fill...)
			return ref, nil
		}
		text := l.dst.Binary(hex.EncodeToString(x))
		if l.dst.CastBinary != nil {
			if name := l.dst.CastBinary(c.to); name != "" {
				text = "CAST(" + text + " AS " + name + ")"
			}
		}
		return text, nil
	case time.Time:
		return l.timeLiteral(c.to.Kind, x)
	case string:
		if c.to.Kind == Decimal || c.to.Kind == Integer || c.to.Kind == BigInteger {
			return x, nil
		}
		// The text of an expression that builds the value is the value.
		if l.dst.ExpressionColumn != nil && l.dst.ExpressionColumn(c.to) {
			return x, nil
		}
		if strings.ContainsRune(x, 0) {
			return "", fmt.Errorf("text with a NUL has no literal in %s: %w", l.target, ErrBadValue)
		}
		return l.dst.Literal(x), nil
	}
	return "", fmt.Errorf("%T has no literal: %w", conv, ErrBadValue)
}

// timeLiteral writes a date or a time, with an offset for an instant when the
// product reads one.
func (l *Loader) timeLiteral(kind Kind, t time.Time) (string, error) {
	text, err := l.timeText(kind, t)
	if err != nil || l.dst.TimeLiteral == nil {
		return text, err
	}
	return l.dst.TimeLiteral(kind, text), nil
}

// timeText writes a date or a time as quoted text.
func (l *Loader) timeText(kind Kind, t time.Time) (string, error) {
	// A time of day has no year, and a driver gives it year 0 or 1.
	if kind != TimeOfDay && kind != TimeOfDayOffset && (t.Year() < 1 || t.Year() > 9999) {
		return "", fmt.Errorf("the year %d is outside the years that dump writes: %w", t.Year(), ErrBadValue)
	}
	switch kind {
	case Date:
		return "'" + t.Format("2006-01-02") + "'", nil
	case TimeOfDay:
		return "'" + t.Format("15:04:05.999999999") + "'", nil
	case LocalTimestamp:
		return "'" + t.Format("2006-01-02 15:04:05.999999999") + "'", nil
	case TimeOfDayOffset:
		return "'" + t.Format("15:04:05.999999999Z07:00") + "'", nil
	}
	// A product that keeps the offset of a value, as Oracle does for TIMESTAMP WITH TIME
	// ZONE, gets it back, when the source is the same product. See D297.
	if kind == Timestamp && l.dst.KeepOffset && l.source == l.target {
		return "'" + t.Format("2006-01-02 15:04:05.999999999 -07:00") + "'", nil
	}
	t = t.UTC()
	if l.dst.ZoneInTimestamps {
		return "'" + t.Format("2006-01-02 15:04:05.999999999") + "+00:00'", nil
	}
	return "'" + t.Format("2006-01-02 15:04:05.999999999") + "'", nil
}

// sourceValue gives the value of a column as the loader reads it: the instant
// that a source with a session clock meant, and the bytes of a uuid in the order
// of its text for a source whose driver gives them in the order of the product.
func (l *Loader) sourceValue(c loadColumn, v any) any {
	v = l.sessionClock(c, v)
	if b, ok := v.([]byte); ok && len(b) == 16 && c.from.Kind == UUID && l.src.MixedEndianUUID {
		return []byte{b[3], b[2], b[1], b[0], b[5], b[4], b[7], b[6], b[8], b[9], b[10], b[11], b[12], b[13], b[14], b[15]}
	}
	return v
}
