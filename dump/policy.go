package dump

import (
	"fmt"
	"strings"

	"github.com/xo/dbmeta"
)

// Allow says which levels below Exact a column can have. Exact is always
// allowed. A level allows only itself, so allowing Text does not allow Lossy:
// someone can want an array written as text and still refuse a float that loses
// digits.
type Allow struct {
	Lossy bool
	Text  bool
}

// Policy says what a dump allows. The zero value allows only what is Exact, and
// dbmeta never picks a level that the policy did not allow.
//
// It has three scopes, and the most specific one wins: a level for a column, a
// level for a kind, and the default.
type Policy struct {
	// Default is what every column allows, unless a narrower scope says more.
	Default Allow
	// Kinds is what the columns of a kind allow.
	Kinds map[Kind]Allow
	// Columns is what one column allows, by its stable name, such as
	// schema.table.column.
	Columns map[string]Allow
	// Casts read a column as another kind than its type text says.
	Casts []Cast
}

// Cast reads a column as another kind, in the way that pgloader casts a type. It
// matches by the text of the source type, or by the name of the column, or both,
// and an empty field matches every column. The name of a column is a pattern of
// SQL LIKE over schema.table.column.
type Cast struct {
	// Column is a pattern for schema.table.column.
	Column string
	// Type is the source type text, compared without case.
	Type string
	// Kind is the kind to read the column as.
	Kind Kind
}

// allow returns what a column allows.
func (p Policy) allow(name string, k Kind) Allow {
	if a, ok := p.Columns[name]; ok {
		return a
	}
	if a, ok := p.Kinds[k]; ok {
		return a
	}
	return p.Default
}

// cast applies the first cast that matches the column.
func (p Policy) cast(name string, t Type) Type {
	for _, c := range p.Casts {
		if c.Column != "" && !like(c.Column, name) {
			continue
		}
		if c.Type != "" && !strings.EqualFold(strings.TrimSpace(c.Type), strings.TrimSpace(t.Raw)) {
			continue
		}
		t.Kind = c.Kind
		return t
	}
	return t
}

// ColumnPlan is what a dump does with one column.
type ColumnPlan struct {
	// Name is the stable name, such as schema.table.column.
	Name string
	// Source is the type as read, and Type is the type after the casts.
	Source Type
	Type   Type
	// SQL is the type in the target, with the level and the reason for a level
	// that is not Exact.
	SQL   string
	Level Level
	Note  string
	// Check is a statement for the client to run on the source before the dump,
	// for a column that was read as a kind by a convention. It returns the number
	// of rows that the kind cannot hold, and the dump stops if it is not zero.
	Check string
}

// Object is an object that a dump writes as the text of the source, because
// dbmeta translates no SQL (D274).
type Object struct {
	// Kind is view, routine, trigger, check, default or generated.
	Kind string
	Name string
	// Source is the text of the source, when there is one.
	Source string
}

// Plan is what a dump to a target does with a snapshot, known before any
// statement is written, so that a client can warn or stop first.
type Plan struct {
	Source, Target dbmeta.Dialect
	Columns        []ColumnPlan
	// Untranslated are the objects that are written as the text of the source.
	Untranslated []Object
	// DroppedCollations are the columns whose collation is not written, because
	// the target is not the source. A collation changes uniqueness and sort order.
	DroppedCollations []string
}

// Failure is a column that has no allowed level.
type Failure struct {
	Name string
	Type string
	// Err says why. It wraps [ErrNoEquivalent].
	Err error
}

// PlanError lists every column that has no allowed level, all at once, so that
// one run says every flag that the user needs.
type PlanError struct {
	Failures []Failure
}

// Error satisfies the error interface.
func (e *PlanError) Error() string {
	parts := make([]string, len(e.Failures))
	for i, f := range e.Failures {
		parts[i] = f.Name + " (" + f.Type + ")"
	}
	return fmt.Sprintf("%d columns have no allowed type in the target: %s", len(e.Failures), strings.Join(parts, ", "))
}

// Unwrap lets errors.Is find [ErrNoEquivalent].
func (e *PlanError) Unwrap() error { return ErrNoEquivalent }

// MakePlan works out what a dump to the target does with each column and which
// objects it writes as the text of the source. It fails with a [*PlanError] when
// a column has no allowed level.
func MakePlan(s *Snapshot, target dbmeta.Dialect, p Policy) (Plan, error) {
	plan := Plan{Source: dbmeta.Dialect(s.Dialect), Target: target}
	var failures []Failure
	same := plan.Source == target
	for _, sc := range s.Schemas {
		for _, t := range sc.Tables {
			keys := keyColumns(t)
			for _, c := range t.Columns {
				name := sc.Name + "." + t.Name + "." + c.Name
				cp, err := planColumn(plan.Source, target, sc.Name, t.Name, name, c, p, keys[c.Name])
				if err != nil {
					failures = append(failures, Failure{Name: name, Type: c.Type.Raw, Err: err})
					continue
				}
				plan.Columns = append(plan.Columns, cp)
				if !same {
					if c.Collation != "" && c.Collation != "default" {
						plan.DroppedCollations = append(plan.DroppedCollations, name)
					}
					if c.Generated != "" && c.Default != nil {
						plan.Untranslated = append(plan.Untranslated, Object{Kind: "generated", Name: name, Source: *c.Default})
					} else if c.Default != nil {
						if _, ok := portableDefault(plan.Source, target, c); !ok {
							plan.Untranslated = append(plan.Untranslated, Object{Kind: "default", Name: name, Source: *c.Default})
						}
					}
				}
			}
			if same {
				continue
			}
			for _, c := range t.Constraints {
				if c.Kind == "check" || c.Kind == "exclusion" {
					plan.Untranslated = append(plan.Untranslated, Object{Kind: "check", Name: sc.Name + "." + t.Name + "." + c.Name, Source: c.Definition})
				}
			}
			for _, tg := range t.Triggers {
				plan.Untranslated = append(plan.Untranslated, Object{Kind: "trigger", Name: sc.Name + "." + t.Name + "." + tg.Name, Source: tg.Definition})
			}
		}
		if same {
			continue
		}
		for _, v := range sc.Views {
			plan.Untranslated = append(plan.Untranslated, Object{Kind: "view", Name: sc.Name + "." + v.Name, Source: v.Definition})
		}
		for _, r := range sc.Routines {
			plan.Untranslated = append(plan.Untranslated, Object{Kind: "routine", Name: sc.Name + "." + r.Name, Source: r.Definition})
		}
	}
	if len(failures) > 0 {
		return plan, &PlanError{Failures: failures}
	}
	return plan, nil
}

// planColumn picks the best level that the policy allows for one column, in the
// order Exact, Lossy, Text.
func planColumn(source, target dbmeta.Dialect, schema, table, name string, c Column, p Policy, keyed bool) (ColumnPlan, error) {
	cp := ColumnPlan{Name: name, Source: c.Type, Type: p.cast(name, c.Type)}
	if source == target {
		cp.SQL = c.Type.Raw
		if cp.SQL == "" {
			r, err := RenderType(target, cp.Type)
			if err != nil {
				return cp, err
			}
			cp.SQL = r.SQL
		}
		return cp, nil
	}
	allow := p.allow(name, cp.Type.Kind)
	r, err := RenderType(target, cp.Type)
	if style, serr := styleOf(target); serr == nil && keyed && style.KeyType != nil {
		if kr, ok := style.KeyType(cp.Type); ok {
			r, err = kr, nil
		}
	}
	switch {
	case err == nil && r.Level == Exact:
	case err == nil && r.Level == Lossy && allow.Lossy:
	default:
		if allow.Text {
			t, terr := TextFor(target, cp.Type)
			if terr != nil {
				return cp, terr
			}
			r = t
			break
		}
		if err == nil {
			return cp, fmt.Errorf("%s is %s in the target and the policy does not allow it: %w", c.Type.Raw, r.Level, ErrNoEquivalent)
		}
		return cp, err
	}
	cp.SQL, cp.Level, cp.Note = r.SQL, r.Level, r.Note
	cp.Check = booleanCheck(source, schema, table, c)
	return cp, nil
}

// booleanCheck writes the statement that counts the rows that a boolean cannot
// hold, for a MySQL tinyint(1) or bit(1). The catalog cannot tell a boolean from
// a small number, so the client runs this on the source, and the dump stops when
// it counts a row. The statement is empty for any other column.
func booleanCheck(source dbmeta.Dialect, schema, table string, c Column) string {
	if source != dbmeta.MySQL || c.Type.Kind != Boolean {
		return ""
	}
	style, err := styleOf(source)
	if err != nil {
		return ""
	}
	g := &generator{style: style}
	col := style.Quote(c.Name)
	return "SELECT COUNT(*) FROM " + g.qualified(schema, table) + " WHERE " + col + " IS NOT NULL AND " + col + " NOT IN (0, 1)"
}

// keyColumns returns the columns of a table that are in a key or an index.
func keyColumns(t Table) map[string]bool {
	keys := map[string]bool{}
	for _, c := range t.Constraints {
		for _, name := range c.Columns {
			keys[name] = true
		}
	}
	for _, ix := range t.Indexes {
		cols := ix.Columns
		// A source that keeps the statement and no columns, as DuckDB does.
		if len(cols) == 0 {
			cols = definitionColumns(ix.Definition)
		}
		for _, c := range cols {
			if c.Name != "" {
				keys[c.Name] = true
			}
		}
	}
	return keys
}
