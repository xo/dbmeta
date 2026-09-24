package dbmeta

import (
	"context"
	"database/sql"
	"errors"
	"iter"
	"strings"
	"sync"
)

// Gate is a version test. The server must report Key at Min or newer. A zero
// Gate is met by every server.
//
// Key names which reported version to compare against. The empty key is the
// main one, which every server reports. A named key is a fact about the
// server, and a server that does not report it does not meet the gate however
// new it is. Two things use a name:
//
// A database that reports several versions that move independently. Cassandra
// reports three, so a gate on the CQL version sets Key to "cql".
//
// A product that shares a dialect with another product. MariaDB and MySQL
// share one dialect and one model, and their release numbers have no relation
// to each other, so a gate on MariaDB 10.3 sets Key to "mariadb" and never
// applies to MySQL. Comparing the numbers alone is the fault D44 records.
type Gate struct {
	Min Version
	Key string
}

// Met reports whether versions satisfies g.
func (g Gate) Met(versions VersionSet) bool {
	if g.Key != "" && !versions.Has(g.Key) {
		return false
	}
	return versions.Get(g.Key).AtLeast(g.Min)
}

// Fragment is one alternative for one piece of a statement. It applies when
// the server meets its gate.
type Fragment struct {
	Min Version
	Key string
	SQL string
}

// Gate returns the version test this fragment applies.
func (f Fragment) Gate() Gate { return Gate{Min: f.Min, Key: f.Key} }

// Choice holds the alternatives for one piece of a statement. Exactly one is
// used, and [Choice.Resolve] says which.
//
// A piece is usually one column of the select list, because that is what
// changes between releases.
//
// Every alternative of a Choice must produce the same column, under the same
// name. That rule is what lets one Go type read the result of every supported
// version, and nothing here can check it. Whichever side lacks a source pads,
// old or new, by selecting a literal NULL under that name. Where the type
// differs between releases, each alternative casts to one common type.
type Choice []Fragment

// Resolve returns the SQL of the alternative that applies to versions.
//
// Three rules decide it, in order. An alternative whose gate the server does
// not meet is out. Among those left, a named key beats the empty key, because
// a fragment written for one product is more specific than one written for the
// family. Among those sharing a key, the highest Min wins.
//
// Two alternatives naming different keys, both met, return
// [ErrAmbiguousFragment]. Nothing decides between them and guessing would pick
// by a number that means something different on each side. It is a fault in
// the model.
//
// A Choice where every alternative names a key and the server reports none of
// them returns [ErrNotSupported] rather than [ErrVersionTooOld]. The server is
// not too old for this piece. It is the wrong product for it, and no upgrade
// changes that. A server that does report the key and is below the Min is too
// old, which an upgrade fixes, so that stays [ErrVersionTooOld].
func (c Choice) Resolve(versions VersionSet) (string, error) {
	var (
		best  Fragment
		found bool
		// reachable counts the alternatives this server could meet on a newer
		// release: the ones with no key, and the ones whose key it reports.
		reachable int
	)
	for _, f := range c {
		if f.Key == "" || versions.Has(f.Key) {
			reachable++
		}
		if !f.Gate().Met(versions) {
			continue
		}
		if !found {
			best, found = f, true
			continue
		}
		switch {
		case best.Key == f.Key:
			if best.Min.Compare(f.Min) < 0 {
				best = f
			}
		case best.Key == "":
			best = f
		case f.Key == "":
			// the more specific alternative already won
		default:
			return "", ErrAmbiguousFragment
		}
	}
	switch {
	case found:
		return best.SQL, nil
	case len(c) > 0 && reachable == 0:
		return "", ErrNotSupported
	}
	return "", ErrVersionTooOld
}

// Stmt is the pieces of one statement, in order. Each piece resolves on its
// own, so a statement is assembled rather than chosen.
type Stmt []Choice

// SQL resolves every piece against versions and joins the results with a
// newline. Each piece carries its own punctuation, so a piece that adds a
// column to a select list begins with a comma.
func (st Stmt) SQL(versions VersionSet) (string, error) {
	if len(st) == 0 {
		return "", ErrEmptyQuery
	}
	parts := make([]string, 0, len(st))
	for _, c := range st {
		s, err := c.Resolve(versions)
		if err != nil {
			return "", err
		}
		if s = strings.TrimRight(s, " \t"); s != "" {
			parts = append(parts, s)
		}
	}
	if len(parts) == 0 {
		return "", ErrEmptyQuery
	}
	return strings.Join(parts, "\n"), nil
}

// Always returns a Stmt of one piece that is the same for every version.
func Always(sql string) Stmt {
	return Stmt{Choice{{SQL: sql}}}
}

// Field is a result column that a query declares.
//
// The set of fields is fixed for every version. Min and Key record the oldest
// version where the field has a real source, so a caller can tell a value that
// is null from a field the server has no source for. Below that the column is
// padded with NULL. Read [Field.Present] rather than the gate, which cannot
// tell an absent key from an old server on its own.
type Field struct {
	Name string
	Desc string

	// Min and Key are the gate that gives the field a real source. They mean
	// what they mean on a [Gate].
	Min Version
	Key string
	// Also holds further gates, for a field that arrived in a different
	// release of each product. The field has a source when the gate above is
	// met or any gate here is met. MariaDB recorded a check constraint from
	// 10.2 and MySQL from 8.0.16, and no single number says both.
	Also []Gate
}

// Present reports whether the field has a real source on this server. A field
// that is not present is padded with NULL, so a caller reading NULL calls this
// to find out which of the two it has.
func (f Field) Present(versions VersionSet) bool {
	if (Gate{Min: f.Min, Key: f.Key}).Met(versions) {
		return true
	}
	for _, g := range f.Also {
		if g.Met(versions) {
			return true
		}
	}
	return false
}

// Param is a query parameter that a query declares. The name is what appears
// in the SQL after an at sign, so a parameter named schema is written @schema.
type Param struct {
	Name string
	Desc string
	// Default is used when the caller supplies no value. A parameter with a
	// nil Default is required, and leaving it out is an error.
	//
	// Most metadata parameters narrow a result and mean "every one" when they
	// are empty, so most declare a default. That is not the same as ignoring
	// an unknown name, which is always an error: a caller that misspells a
	// parameter is told, rather than silently getting an unfiltered result.
	Default any
}

// Binding is everything one dialect provides for one query.
type Binding[T any] struct {
	// Stmt is the versioned statement.
	Stmt Stmt
	// Columns are the result columns, in the order the statement returns them.
	Fields []Field
	// Params are the parameters the statement takes.
	Params []Param
	// Scan reads one row. A generator writes it, so no reflection is needed.
	Scan func(*sql.Rows) (T, error)
}

// Fields declares result columns that no version gates, which is most of them.
func Fields(names ...string) []Field {
	out := make([]Field, len(names))
	for i, name := range names {
		out[i] = Field{Name: name}
	}
	return out
}

// Query is one kind of metadata object, such as a table or a column.
//
// There is one exported Query value per object kind, and a model registers
// what its dialect provides for it. A caller names the object by its value, so
// the result type is inferred and an unknown type cannot be asked for.
type Query[T any] struct {
	name string

	mu       sync.RWMutex
	bindings map[Dialect]*Binding[T]
}

// AnyQuery is a query with its result type erased, so that a caller can list
// every query and describe it without naming 40 types. [Query] satisfies it.
//
// Use it to list what dbmeta knows. To read rows, use the [Query] value
// itself, which keeps the result type.
type AnyQuery interface {
	// Name returns the name of the object kind.
	Name() string
	// Support says whether the query can be asked of m.
	Support(m *Meta) Support
	// Fields returns the result columns for m, in order.
	Fields(m *Meta) ([]Field, error)
	// Params returns the parameters the query takes for m.
	Params(m *Meta) ([]Param, error)
	// SQL returns the statement for m and the arguments in order.
	SQL(m *Meta, args map[string]any) (string, []any, error)
}

var (
	queryMu sync.RWMutex
	queries []AnyQuery
)

// NewQuery returns a query for one kind of object. The root package declares
// one value per kind. A model does not call this.
func NewQuery[T any](name string) *Query[T] {
	q := &Query[T]{name: name, bindings: make(map[Dialect]*Binding[T])}
	queryMu.Lock()
	defer queryMu.Unlock()
	for _, other := range queries {
		if other.Name() == name {
			panic("dbmeta: query declared twice: " + name)
		}
	}
	queries = append(queries, q)
	return q
}

// Queries returns every query dbmeta knows, in the order they were declared.
// A caller lists them to show a person what it can ask for, and calls
// [AnyQuery.Support] to find out which of them this database answers.
func Queries() []AnyQuery {
	queryMu.RLock()
	defer queryMu.RUnlock()
	return append([]AnyQuery(nil), queries...)
}

// Name returns the name of the object kind.
func (q *Query[T]) Name() string { return q.name }

// Register records what a dialect provides for this query. A model file in
// internal calls it from its init. Registering twice for one dialect panics,
// because it is a build mistake.
func (q *Query[T]) Register(d Dialect, b *Binding[T]) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if _, ok := q.bindings[d]; ok {
		panic("dbmeta: query " + q.name + " registered twice for dialect " + string(d))
	}
	q.bindings[d] = b
}

// Support says whether this query can be asked of m.
type Support int

// Support values.
const (
	// NotBuilt means no model for the dialect is in this binary. It says
	// nothing about the database.
	NotBuilt Support = iota
	// NotSupported means the model is present and the database has no such
	// object.
	NotSupported
	// Supported means the query can be asked.
	Supported
)

// String satisfies the [fmt.Stringer] interface.
func (s Support) String() string {
	switch s {
	case NotBuilt:
		return "not built"
	case NotSupported:
		return "not supported"
	case Supported:
		return "supported"
	}
	return "supported"
}

// Support reports whether m can answer this query, distinguishing a database
// that has no such object from a model that was left out of this binary.
func (q *Query[T]) Support(m *Meta) Support {
	if m == nil {
		return NotBuilt
	}
	if _, ok := m.dialect.Info(); !ok {
		return NotBuilt
	}
	b := q.binding(m.dialect)
	if b == nil {
		return NotSupported
	}
	// A dialect two products share registers one binding for both, so the
	// binding alone does not say the product answers. A statement built only
	// from fragments naming a key this server does not report is a statement
	// for the other product. See D44.
	if _, err := b.Stmt.SQL(m.versions); errors.Is(err, ErrNotSupported) {
		return NotSupported
	}
	return Supported
}

// Fields returns the result columns for m, in order.
//
// Read [Field.Min] to tell a field the server is too old to have from a value
// that is genuinely null. Both arrive as NULL under the padding rule.
func (q *Query[T]) Fields(m *Meta) ([]Field, error) {
	b, err := q.lookup(m)
	if err != nil {
		return nil, err
	}
	return b.Fields, nil
}

// Params returns the parameters this query takes for m.
func (q *Query[T]) Params(m *Meta) ([]Param, error) {
	b, err := q.lookup(m)
	if err != nil {
		return nil, err
	}
	return b.Params, nil
}

// SQL returns the statement for m with its placeholders written the way the
// dialect wants them, and the argument values in matching order.
//
// The caller can run it itself. An argument named in the statement and missing
// from args is an error, and an argument in args that the statement does not
// name is an error. Neither is ignored.
func (q *Query[T]) SQL(m *Meta, args map[string]any) (string, []any, error) {
	b, err := q.lookup(m)
	if err != nil {
		return "", nil, err
	}
	info, _ := m.dialect.Info()
	s, err := b.Stmt.SQL(m.versions)
	if err != nil {
		return "", nil, err
	}
	return bind(s, info.Placeholder, b.Params, args)
}

// All runs the query against db and yields one value per row.
//
// The iterator holds a database connection until it ends. Stopping early, with
// break or by returning false from the yield, releases it, and so does
// cancelling ctx. Do not open a second iterator inside the body of the first:
// that needs a second connection and deadlocks on a pool of one. Ask for every
// row you want in one call and filter in the loop.
func (q *Query[T]) All(ctx context.Context, m *Meta, db DB, args map[string]any) iter.Seq2[T, error] {
	return func(yield func(T, error) bool) {
		var zero T
		s, vals, err := q.SQL(m, args)
		if err != nil {
			yield(zero, err)
			return
		}
		rows, err := db.QueryContext(ctx, s, vals...)
		if err != nil {
			yield(zero, err)
			return
		}
		defer rows.Close()
		b := q.binding(m.dialect)
		for rows.Next() {
			v, err := b.Scan(rows)
			if !yield(v, err) {
				return
			}
		}
		if err := rows.Err(); err != nil {
			yield(zero, err)
		}
	}
}

// binding returns the registered binding for d, or nil.
func (q *Query[T]) binding(d Dialect) *Binding[T] {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return q.bindings[d]
}

// lookup returns the binding for m, distinguishing a missing model from an
// unsupported object.
func (q *Query[T]) lookup(m *Meta) (*Binding[T], error) {
	switch q.Support(m) {
	case NotBuilt:
		return nil, ErrModelNotBuilt
	case NotSupported:
		return nil, ErrNotSupported
	}
	return q.binding(m.dialect), nil
}

// First returns the first value the sequence yields.
//
// The second result reports whether there was one, so a caller can tell an
// empty result from a zero value. An empty result is not an error: it means
// the database has no such object.
func First[T any](seq iter.Seq2[T, error]) (T, bool, error) {
	for v, err := range seq {
		return v, err == nil, err
	}
	var zero T
	return zero, false, nil
}

// bind rewrites the named parameters of s into the placeholders the dialect
// wants, and returns the values in matching order.
func bind(s string, placeholder func(int) string, params []Param, args map[string]any) (string, []any, error) {
	known := make(map[string]Param, len(params))
	for _, p := range params {
		known[p.Name] = p
	}
	for name := range args {
		if _, ok := known[name]; !ok {
			return "", nil, ErrUnknownParam
		}
	}
	var (
		out  strings.Builder
		vals []any
	)
	for {
		i := strings.IndexByte(s, '@')
		if i < 0 {
			out.WriteString(s)
			break
		}
		out.WriteString(s[:i])
		s = s[i+1:]
		j := 0
		for j < len(s) && (s[j] == '_' || s[j] >= 'a' && s[j] <= 'z' || s[j] >= 'A' && s[j] <= 'Z' || s[j] >= '0' && s[j] <= '9') {
			j++
		}
		name := s[:j]
		s = s[j:]
		if name == "" {
			out.WriteByte('@')
			continue
		}
		p, ok := known[name]
		if !ok {
			return "", nil, ErrUnknownParam
		}
		v, given := args[name]
		if !given {
			if p.Default == nil {
				return "", nil, ErrMissingParam
			}
			v = p.Default
		}
		// A repeated parameter gets a new placeholder and a repeated value,
		// rather than reusing the first one. PostgreSQL would accept either,
		// because $2 may appear twice, but MySQL writes ? and every ? consumes
		// one argument. Reusing the number there gives "expected 5 arguments,
		// got 3". Appending is correct for both.
		vals = append(vals, v)
		out.WriteString(placeholder(len(vals)))
	}
	return out.String(), vals, nil
}
