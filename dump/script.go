package dump

import (
	"cmp"
	"fmt"
	"iter"
	"slices"
	"strconv"
	"strings"

	"github.com/xo/dbmeta"
)

// Stage is a part of a script, as in pg_dump.
type Stage int

// The stages. The client writes the rows of the Data stage between the others.
const (
	// Pre holds what the data needs to exist: schemas, types, sequences, tables
	// with their columns and primary keys.
	Pre Stage = iota
	// Data is where the rows go. Script writes no statement of this stage. The
	// builders of the data side do.
	Data
	// Post holds what is cheaper to build once the rows are in: unique keys,
	// checks, foreign keys, indexes, views, triggers and comments.
	Post
)

// String returns the name of the stage.
func (s Stage) String() string {
	switch s {
	case Pre:
		return "pre"
	case Data:
		return "data"
	case Post:
		return "post"
	}
	return "stage " + strconv.Itoa(int(s))
}

// Statement is one statement of a script.
type Statement struct {
	Stage Stage
	// Kind is table, index, view, and so on.
	Kind string
	// Name is a stable name, such as schema.table or schema.table.column, that a
	// client writes next to the statement as a comment.
	Name string
	// SQL is the statement with no terminator. It is empty for an object that
	// cannot be written, and Note says why.
	SQL string
	// Note is a warning, such as a type that was not exact, or the reason that SQL
	// is empty.
	Note string
	// Untranslated says that SQL is in the dialect of the source and was not
	// translated. A view, a routine or a trigger for another dialect is.
	Untranslated bool
}

// Script yields the statements that recreate the schema of a snapshot in the
// target dialect, in the order of the stages. A client runs the Pre statements,
// then loads the rows, then runs the Post statements.
//
// A snapshot is written back to its own dialect with each type as it was read.
func Script(s *Snapshot, target dbmeta.Dialect, p Policy) iter.Seq2[Statement, error] {
	return func(yield func(Statement, error) bool) {
		stmts, err := build(s, target, p)
		if err != nil {
			yield(Statement{}, err)
			return
		}
		for _, st := range stmts {
			if !yield(st, nil) {
				return
			}
		}
	}
}

// generator builds the statements of one script.
type generator struct {
	snap   *Snapshot
	source dbmeta.Dialect
	target dbmeta.Dialect
	style  Style
	out    []Statement
}

func build(s *Snapshot, target dbmeta.Dialect, _ Policy) ([]Statement, error) {
	source := dbmeta.Dialect(s.Dialect)
	if source != target {
		return nil, fmt.Errorf("writing a %s snapshot for %s: %w", source, target, dbmeta.ErrNotSupported)
	}
	style, err := styleOf(target)
	if err != nil {
		return nil, err
	}
	g := &generator{snap: s, source: source, target: target, style: style}
	if err := g.pre(); err != nil {
		return nil, err
	}
	g.post()
	return g.out, nil
}

func (g *generator) add(st Statement) { g.out = append(g.out, st) }

// qualified writes the name of an object, with its schema where the style asks.
func (g *generator) qualified(schema, name string) string {
	if g.style.QualifyAll || (g.style.DefaultSchema != "*" && schema != g.style.DefaultSchema && schema != "") {
		return g.style.Quote(schema) + "." + g.style.Quote(name)
	}
	return g.style.Quote(name)
}

// pre writes the statements that come before the rows.
func (g *generator) pre() error {
	for _, text := range g.style.Preamble {
		g.add(Statement{Stage: Pre, Kind: "setting", SQL: text})
	}
	for _, sc := range g.snap.Schemas {
		if g.style.CreateSchema == nil || g.style.DefaultSchema == "*" || slices.Contains(g.style.ExistingSchemas, sc.Name) {
			continue
		}
		g.add(Statement{Stage: Pre, Kind: "schema", Name: sc.Name, SQL: g.style.CreateSchema(g.style.Quote(sc.Name))})
	}
	for _, sc := range g.snap.Schemas {
		if g.style.Enum != nil {
			for _, e := range sc.Enums {
				name := g.qualified(sc.Name, e.Name)
				g.add(Statement{Stage: Pre, Kind: "type", Name: sc.Name + "." + e.Name,
					SQL: g.style.Enum(name, e.Values, g.style.Literal)})
			}
		}
		if g.style.Sequence != nil {
			for _, q := range sc.Sequences {
				if q.Identity {
					continue
				}
				g.add(Statement{Stage: Pre, Kind: "sequence", Name: sc.Name + "." + q.Name,
					SQL: g.style.Sequence(g.qualified(sc.Name, q.Name), q)})
			}
		}
	}
	for _, sc := range g.snap.Schemas {
		for _, r := range sc.Routines {
			g.routine(sc.Name, r)
		}
	}
	tables, err := g.orderedTables()
	if err != nil {
		return err
	}
	for _, t := range tables {
		sql, err := g.createTable(t.schema, t.table)
		if err != nil {
			return err
		}
		g.add(Statement{Stage: Pre, Kind: "table", Name: t.schema + "." + t.table.Name, SQL: sql})
	}
	return nil
}

// routine writes a routine from the definition that the source keeps. The text
// is the source's own, so a statement for the same dialect needs no change.
func (g *generator) routine(schema string, r Routine) {
	name := schema + "." + r.Name
	def := strings.TrimSpace(r.Definition)
	if !strings.HasPrefix(strings.ToLower(def), "create") {
		g.add(Statement{Stage: Pre, Kind: "routine", Name: name,
			Note: "the source keeps no statement that creates this " + r.Kind + ", so it is not written"})
		return
	}
	g.add(Statement{Stage: Pre, Kind: "routine", Name: name, SQL: strings.TrimRight(def, "; \n\t")})
}

// schemaTable is a table with the schema it is in.
type schemaTable struct {
	schema string
	table  Table
}

// orderedTables lists the tables, in order of name. A product that needs the
// table of a foreign key to exist first gets them in foreign key order.
func (g *generator) orderedTables() ([]schemaTable, error) {
	var all []schemaTable
	for _, sc := range g.snap.Schemas {
		for _, t := range sc.Tables {
			all = append(all, schemaTable{sc.Name, t})
		}
	}
	if !g.style.InlineConstraints || g.style.OrderFree {
		return all, nil
	}
	key := func(schema, table string) string { return schema + "\x00" + table }
	index := map[string]int{}
	for i, t := range all {
		index[key(t.schema, t.table.Name)] = i
	}
	deps := make([][]int, len(all))
	for i, t := range all {
		for _, c := range t.table.Constraints {
			if c.Kind != "foreign key" || c.Ref == nil {
				continue
			}
			schema := c.Ref.Schema
			if schema == "" {
				schema = t.schema
			}
			if j, ok := index[key(schema, c.Ref.Table)]; ok && j != i {
				deps[i] = append(deps[i], j)
			}
		}
	}
	// Depth first, so that a table follows the tables it points at.
	const (
		fresh = iota
		active
		done
	)
	state := make([]int, len(all))
	var out []schemaTable
	var visit func(i int) error
	visit = func(i int) error {
		switch state[i] {
		case done:
			return nil
		case active:
			return fmt.Errorf("the table %s.%s is in a circle: %w", all[i].schema, all[i].table.Name, ErrCycle)
		}
		state[i] = active
		for _, j := range deps[i] {
			if err := visit(j); err != nil {
				return err
			}
		}
		state[i] = done
		out = append(out, all[i])
		return nil
	}
	for i := range all {
		if err := visit(i); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// createTable writes the CREATE TABLE of a table.
func (g *generator) createTable(schema string, t Table) (string, error) {
	q, lit := g.style.Quote, g.style.Literal
	var parts []string
	for _, c := range t.Columns {
		r, err := TypeFor(g.source, g.target, c.Type)
		if err != nil {
			return "", fmt.Errorf("the column %s.%s.%s: %w", schema, t.Name, c.Name, err)
		}
		if g.style.Column != nil {
			parts = append(parts, g.style.Column(c, r.SQL, q, lit))
		} else {
			parts = append(parts, genericColumn(c, r.SQL, q, lit))
		}
	}
	for _, c := range t.Constraints {
		inline := c.Kind == "primary key" || g.style.InlineConstraints
		if !inline {
			continue
		}
		text, ok := g.constraintSQL(schema, c)
		if !ok {
			continue
		}
		parts = append(parts, text)
	}
	sql := "CREATE TABLE " + g.qualified(schema, t.Name) + " (\n  " + strings.Join(parts, ",\n  ") + "\n)"
	if g.style.TableOptions != nil {
		sql += g.style.TableOptions(t, lit)
	}
	return sql, nil
}

// genericColumn writes a column as the name, the type, the default and NOT NULL.
func genericColumn(c Column, typeSQL string, q func(string) string, lit func(string) string) string {
	var b strings.Builder
	b.WriteString(q(c.Name) + " " + typeSQL)
	if c.Default != nil {
		b.WriteString(" DEFAULT " + *c.Default)
	}
	if !c.Nullable {
		b.WriteString(" NOT NULL")
	}
	_ = lit
	return b.String()
}

// constraintSQL writes a constraint as it appears in CREATE TABLE and after ALTER
// TABLE ... ADD. The second result is false for a constraint that the source
// kept no text for, and the caller says so in a note.
func (g *generator) constraintSQL(schema string, c Constraint) (string, bool) {
	q := g.style.Quote
	cols := make([]string, len(c.Columns))
	for i, name := range c.Columns {
		cols[i] = q(name)
	}
	body := c.Definition
	switch c.Kind {
	case "primary key":
		body = "PRIMARY KEY (" + strings.Join(cols, ", ") + ")"
	case "unique":
		if body == "" {
			body = "UNIQUE (" + strings.Join(cols, ", ") + ")"
		}
	case "check":
		// A source can keep the clause alone, with or without its brackets.
		if body = strings.TrimSpace(body); body != "" && !strings.HasPrefix(strings.ToUpper(body), "CHECK") {
			if !strings.HasPrefix(body, "(") {
				body = "(" + body + ")"
			}
			body = "CHECK " + body
		}
	case "foreign key":
		if c.Ref != nil && body == "" {
			ref := make([]string, len(c.Ref.Columns))
			for i, name := range c.Ref.Columns {
				ref[i] = q(name)
			}
			refSchema := c.Ref.Schema
			if refSchema == "" {
				refSchema = schema
			}
			body = "FOREIGN KEY (" + strings.Join(cols, ", ") + ") REFERENCES " +
				g.qualified(refSchema, c.Ref.Table) + " (" + strings.Join(ref, ", ") + ")"
			if c.OnDelete != "" && !strings.EqualFold(c.OnDelete, "NO ACTION") {
				body += " ON DELETE " + c.OnDelete
			}
			if c.OnUpdate != "" && !strings.EqualFold(c.OnUpdate, "NO ACTION") {
				body += " ON UPDATE " + c.OnUpdate
			}
		} else if c.Ref != nil && g.style.ReferenceTable != nil {
			body = g.style.ReferenceTable(body, *c.Ref, q)
		}
	}
	if body == "" {
		return "", false
	}
	if g.style.AnonymousConstraints || c.Name == "" {
		return body, true
	}
	return "CONSTRAINT " + q(c.Name) + " " + body, true
}

// post writes the statements that come after the rows.
func (g *generator) post() {
	// Unique keys and checks first, because a foreign key needs the key that it
	// points at.
	for _, kind := range []string{"unique", "exclusion", "check", "foreign key"} {
		if g.style.InlineConstraints {
			break
		}
		for _, sc := range g.snap.Schemas {
			for _, t := range sc.Tables {
				for _, c := range t.Constraints {
					if c.Kind != kind {
						continue
					}
					text, ok := g.constraintSQL(sc.Name, c)
					if !ok {
						g.add(Statement{Stage: Post, Kind: "constraint", Name: sc.Name + "." + t.Name + "." + c.Name,
							Note: "the source keeps no text for this " + c.Kind + ", so it is not written"})
						continue
					}
					g.add(Statement{Stage: Post, Kind: "constraint", Name: sc.Name + "." + t.Name + "." + c.Name,
						SQL: "ALTER TABLE " + g.qualified(sc.Name, t.Name) + " ADD " + text})
				}
			}
		}
	}
	for _, sc := range g.snap.Schemas {
		for _, t := range sc.Tables {
			names := map[string]bool{}
			for _, c := range t.Constraints {
				names[c.Name] = true
			}
			for _, ix := range t.Indexes {
				if ix.Constraint != "" || names[ix.Name] || (g.style.ImplicitIndex != nil && g.style.ImplicitIndex(t, ix)) {
					continue
				}
				name := sc.Name + "." + t.Name + "." + ix.Name
				if ix.Definition == "" && len(ix.Columns) == 0 {
					g.add(Statement{Stage: Post, Kind: "index", Name: name,
						Note: "the source keeps no columns and no statement for this index, so it is not written"})
					continue
				}
				g.add(Statement{Stage: Post, Kind: "index", Name: name, SQL: g.index(sc.Name, t, ix)})
			}
		}
	}
	for _, sc := range g.snap.Schemas {
		for _, q := range sc.Sequences {
			if q.OwnedBy == "" || q.Identity || g.style.OwnedBy == nil {
				continue
			}
			g.add(Statement{Stage: Post, Kind: "sequence owner", Name: sc.Name + "." + q.Name,
				SQL: g.style.OwnedBy(g.qualified(sc.Name, q.Name), q.OwnedBy)})
		}
	}
	for _, sc := range g.snap.Schemas {
		for _, v := range sc.Views {
			g.add(Statement{Stage: Post, Kind: "view", Name: sc.Name + "." + v.Name, SQL: g.view(sc.Name, v)})
		}
	}
	for _, sc := range g.snap.Schemas {
		for _, t := range sc.Tables {
			for _, tg := range t.Triggers {
				g.trigger(sc.Name, t, tg)
			}
		}
	}
	g.comments()
}

// index writes a CREATE INDEX. The definition that the source keeps is used when
// there is one, because it holds the parts that the columns do not.
func (g *generator) index(schema string, t Table, ix Index) string {
	table := g.qualified(schema, t.Name)
	if ix.Definition != "" {
		return strings.TrimRight(strings.TrimSpace(ix.Definition), "; \n\t")
	}
	if g.style.Index != nil {
		return g.style.Index(table, ix, g.style.Quote)
	}
	return genericIndex(table, ix, g.style.Quote)
}

// genericIndex writes CREATE INDEX with the columns, the direction and the
// predicate.
func genericIndex(table string, ix Index, q func(string) string) string {
	var cols []string
	for _, c := range ix.Columns {
		text := q(c.Name)
		if c.Expression != "" {
			text = "(" + c.Expression + ")"
		}
		if c.Descending {
			text += " DESC"
		}
		cols = append(cols, text)
	}
	sql := "CREATE "
	if ix.Unique {
		sql += "UNIQUE "
	}
	sql += "INDEX " + q(ix.Name) + " ON " + table + " (" + strings.Join(cols, ", ") + ")"
	if ix.Predicate != "" {
		sql += " WHERE " + ix.Predicate
	}
	return sql
}

// view writes a view from its definition.
func (g *generator) view(schema string, v View) string {
	name := g.qualified(schema, v.Name)
	if g.style.View != nil {
		return g.style.View(name, v)
	}
	def := strings.TrimRight(strings.TrimSpace(v.Definition), "; \n\t")
	return "CREATE VIEW " + name + " AS " + def
}

// trigger writes a trigger from what the source keeps.
func (g *generator) trigger(schema string, t Table, tg Trigger) {
	name := schema + "." + t.Name + "." + tg.Name
	if g.style.Trigger != nil {
		sql := g.style.Trigger(g.qualified(schema, t.Name), tg, g.style.Quote)
		if sql == "" {
			g.add(Statement{Stage: Post, Kind: "trigger", Name: name,
				Note: "the definition of this trigger is not in a form that the dump reads, so it is not written"})
			return
		}
		g.add(Statement{Stage: Post, Kind: "trigger", Name: name, SQL: sql})
		return
	}
	def := strings.TrimSpace(tg.Definition)
	if !strings.HasPrefix(strings.ToLower(def), "create") {
		g.add(Statement{Stage: Post, Kind: "trigger", Name: name,
			Note: "the source keeps no statement that creates this trigger, so it is not written"})
		return
	}
	g.add(Statement{Stage: Post, Kind: "trigger", Name: name, SQL: strings.TrimRight(def, "; \n\t")})
}

// comments writes the comment of each table and column, for a product that sets
// them with a statement.
func (g *generator) comments() {
	if g.style.Comment == nil {
		return
	}
	for _, sc := range g.snap.Schemas {
		tables := slices.Clone(sc.Tables)
		slices.SortFunc(tables, func(a, b Table) int { return cmp.Compare(a.Name, b.Name) })
		for _, t := range tables {
			qt := g.qualified(sc.Name, t.Name)
			if t.Comment != "" {
				g.add(Statement{Stage: Post, Kind: "comment", Name: sc.Name + "." + t.Name,
					SQL: g.style.Comment("TABLE", qt, t.Comment, g.style.Literal)})
			}
			for _, c := range t.Columns {
				if c.Comment != "" {
					g.add(Statement{Stage: Post, Kind: "comment", Name: sc.Name + "." + t.Name + "." + c.Name,
						SQL: g.style.Comment("COLUMN", qt+"."+g.style.Quote(c.Name), c.Comment, g.style.Literal)})
				}
			}
		}
	}
}
