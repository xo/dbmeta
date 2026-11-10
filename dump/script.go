package dump

import (
	"cmp"
	"fmt"
	"iter"
	"maps"
	"regexp"
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
	// Data holds the settings that go round the rows, such as a product turning
	// its foreign key checks off. The rows are the client's, and the builders of
	// the data side write them.
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
	// translated. A view, a routine, a trigger or a check for another dialect is.
	Untranslated bool
	// Check says that the statement returns a row for each fault, and that the
	// dump fails when it returns one.
	Check bool
}

// Script yields the statements that recreate the schema of a snapshot in the
// target dialect, in the order of the stages. A client runs the Pre statements,
// then the Data settings, then loads the rows, then runs the Post statements.
//
// A snapshot written back to its own dialect keeps each type as it was read. For
// any other dialect the types come from the plan, and the objects that hold SQL
// are written as the text of the source and marked Untranslated. Script fails
// with a [*PlanError] when a column has no type that the policy allows.
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
	cross  bool
	plan   map[string]ColumnPlan
	out    []Statement
	// used is the schema that the last UseSchema statement of each stage named.
	used map[Stage]string
	// rangeWritten holds the routines that came before a range type that names them.
	rangeWritten map[string]bool
	// granted holds the grants that were written, by grantee and object.
	granted map[string]bool
}

func build(s *Snapshot, target dbmeta.Dialect, p Policy) ([]Statement, error) {
	source := dbmeta.Dialect(s.Dialect)
	style, err := styleOf(target)
	if err != nil {
		return nil, err
	}
	plan, err := MakePlan(s, target, p)
	if err != nil {
		return nil, err
	}
	g := &generator{snap: withPartitions(s, source != target || style.PartitionOf == nil), source: source, target: target, style: style, cross: source != target,
		plan: make(map[string]ColumnPlan, len(plan.Columns))}
	for _, c := range plan.Columns {
		g.plan[c.Name] = c
	}
	if err := g.pre(); err != nil {
		return nil, err
	}
	g.data()
	g.post()
	return g.out, nil
}

func (g *generator) add(st Statement) { g.out = append(g.out, st) }

// qualified writes the name of an object, with its schema where the style asks.
func (g *generator) qualified(schema, name string) string {
	if g.style.QualifyAll || g.severalDatabases() || g.otherDatabase(schema) || (g.style.DefaultSchema != "*" && schema != g.style.DefaultSchema && schema != "") {
		return g.style.Quote(schema) + "." + g.style.Quote(name)
	}
	return g.style.Quote(name)
}

// severalDatabases reports that the snapshot holds more than one schema of a
// product whose schema is its database and that makes a database with a statement.
// A script for another product, and one for SQLite, which has one schema, never qualify. The statements name each database then,
// because the connection of a script has one default database at most. See D293.
func (g *generator) severalDatabases() bool {
	return g.snap != nil && !g.cross && g.style.DefaultSchema == "*" && g.style.CreateSchema != nil && len(g.snap.Schemas) > 1
}

// otherDatabase reports that a name is in a database other than the one that the
// snapshot holds alone. A foreign key can point at it, and the script has no
// default database that means it. See D294.
func (g *generator) otherDatabase(schema string) bool {
	if g.snap == nil || g.cross || g.style.DefaultSchema != "*" || g.style.CreateSchema == nil || schema == "" {
		return false
	}
	return len(g.snap.Schemas) == 1 && g.snap.Schemas[0].Name != schema
}

// pre writes the statements that come before the rows.
func (g *generator) pre() error {
	for _, text := range g.style.Preamble {
		g.add(Statement{Stage: Pre, Kind: "setting", SQL: text})
	}
	for _, sc := range g.snap.Schemas {
		if g.style.CreateSchema == nil || (g.style.DefaultSchema == "*" && !g.severalDatabases()) || slices.Contains(g.style.ExistingSchemas, sc.Name) {
			continue
		}
		g.add(Statement{Stage: Pre, Kind: "schema", Name: sc.Name, SQL: g.style.CreateSchema(g.style.Quote(sc.Name))})
		if g.style.SchemaSetup != nil {
			for _, text := range g.style.SchemaSetup(g.style.Quote(sc.Name)) {
				g.add(Statement{Stage: Pre, Kind: "setting", Name: sc.Name, SQL: text})
			}
		}
	}
	g.roles()
	// Extensions come before the objects that use their types.
	if !g.cross && g.style.Extension != nil {
		for _, e := range g.snap.Extensions {
			schema := e.Schema
			if schema == "" {
				schema = g.style.DefaultSchema
			}
			g.add(Statement{Stage: Pre, Kind: "extension", Name: e.Name, SQL: g.style.Extension(g.style.Quote(e.Name), g.style.Quote(schema))})
		}
	}
	// Enums, domains, composite types and sequences are the objects of the source,
	// and another dialect builds its own from the columns.
	if !g.cross {
		for _, sc := range g.snap.Schemas {
			if g.style.Enum != nil {
				for _, e := range sc.Enums {
					g.add(Statement{Stage: Pre, Kind: "type", Name: sc.Name + "." + e.Name,
						SQL: g.style.Enum(g.qualified(sc.Name, e.Name), e.Values, g.style.Literal)})
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
	}
	if !g.cross {
		g.rangeFunctions()
		g.customTypes()
		g.partitionObjects()
		g.xmlSchemas()
	}
	g.earlyRoutines()
	tables, err := g.orderedTables()
	if err != nil {
		return err
	}
	for _, t := range tables {
		sql, err := g.createTable(t.schema, t.table)
		if err != nil {
			return err
		}
		g.add(Statement{Stage: Pre, Kind: "table", Name: t.schema + "." + t.table.Name, SQL: sql,
			Note: joinNotes(g.droppedRules(t.table), g.partitionNote(t.table), g.versioningNote(t.table), g.lostValues(t.table))})
	}
	for _, t := range g.partitions() {
		g.add(Statement{Stage: Pre, Kind: "table", Name: t.schema + "." + t.table.Name, SQL: g.createPartition(t.schema, t.table)})
	}
	g.synonyms()
	return nil
}

// data writes the settings that go round the rows.
func (g *generator) data() {
	for _, text := range g.style.BeforeData {
		g.add(Statement{Stage: Data, Kind: "setting", SQL: text})
	}
	g.versioningOff()
}

// routine writes a routine from the definition that the source keeps. The text
// is the source's own, so it is marked Untranslated for another dialect.
func (g *generator) routine(stage Stage, schema string, r Routine) {
	name := schema + "." + r.Name
	def := strings.TrimSpace(r.Definition)
	if !strings.HasPrefix(strings.ToLower(def), "create") {
		g.add(Statement{Stage: stage, Kind: "routine", Name: name, Untranslated: g.cross,
			Note: "the source keeps no statement that creates this " + r.Kind + ", so it is not written"})
		return
	}
	g.use(stage, schema)
	if r.SQLMode != nil && g.style.SQLMode != nil && !g.cross {
		g.add(Statement{Stage: stage, Kind: "setting", Name: name, SQL: g.style.SQLMode(*r.SQLMode)})
		defer g.add(Statement{Stage: stage, Kind: "setting", Name: name, SQL: g.style.ResetSQLMode})
	}
	g.add(Statement{Stage: stage, Kind: "routine", Name: name, SQL: g.endOf(def), Untranslated: g.cross})
}

// use writes the statement that makes a schema the one that a name with no schema
// means, before a statement of the source's own text, when the product has one and
// the last statement of the stage named another schema.
func (g *generator) use(stage Stage, schema string) {
	if g.style.UseSchema == nil || schema == "" || g.used[stage] == schema {
		return
	}
	// A product whose schema is its database has the one default database of the
	// connection, unless the script makes several.
	if g.style.DefaultSchema == "*" && !g.severalDatabases() {
		return
	}
	if g.used == nil {
		g.used = map[Stage]string{}
	}
	g.used[stage] = schema
	g.add(Statement{Stage: stage, Kind: "setting", Name: schema, SQL: g.style.UseSchema(g.style.Quote(schema))})
}

// endOf takes the closing semicolons off the text of a stored object, and keeps the
// one that belongs to it when the product says that it does.
func (g *generator) endOf(def string) string {
	def = strings.TrimSpace(def)
	if g.style.KeepTerminator != nil && g.style.KeepTerminator(def) {
		return def
	}
	return strings.TrimRight(def, "; \n\t")
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
			if !t.IsPartition() {
				all = append(all, schemaTable{sc.Name, t})
			}
		}
	}
	// A product that writes its foreign keys after the tables has no order to keep, except
	// for a table of reference partitioning, which has its key in its own statement.
	referenceOnly := !g.style.InlineConstraints
	if referenceOnly {
		found := false
		for _, t := range all {
			if t.table.Partitioning != nil && t.table.Partitioning.Reference != "" {
				found = true
			}
		}
		if !found {
			return all, nil
		}
	}
	if g.style.OrderFree {
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
			if referenceOnly && (t.table.Partitioning == nil || t.table.Partitioning.Reference != c.Name) {
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

// column returns a column as the target writes it. For the same dialect it is
// the column as it was read. For another dialect the collation is dropped, a
// default is kept only when every product writes it the same way, and a
// generated column loses its expression, because dbmeta translates no SQL.
func (g *generator) column(c Column) Column {
	if !g.cross {
		return c
	}
	c.Collation = ""
	// Auto numbering is the one idiom that moves with the column. A serial column
	// of PostgreSQL is an integer with a default that reads a sequence, and the
	// sequence does not move, so the column is an identity column of the target.
	// Any identity of the source is one that a load can still give a value to.
	if c.Identity != "" {
		c.Identity = "by default"
	}
	if g.source == dbmeta.PostgreSQL && c.Default != nil && isNextval(*c.Default) && c.Identity == "" {
		c.Identity, c.Default = "by default", nil
	}
	switch {
	case c.Generated != "":
		c.Generated, c.Default, c.DefaultLiteral = "", nil, false
	case c.Default != nil:
		if text, ok := portableDefault(g.source, g.target, c); ok && text == "NULL" {
			// A default of NULL is the same as none.
			c.Default, c.DefaultLiteral = nil, false
		} else if ok {
			c.Default, c.DefaultLiteral = &text, false
		} else {
			c.Default, c.DefaultLiteral = nil, false
		}
	}
	return c
}

// inCreate says whether a constraint of this kind is written inside CREATE TABLE.
// A primary key always is. A product that cannot add a key to a table that exists
// writes unique keys and foreign keys there. For another dialect a check keeps the
// text of the source, so it is a statement of its own and can be refused alone.
func (g *generator) inCreate(kind string) bool {
	switch {
	case kind == "primary key":
		return true
	case !g.style.InlineConstraints:
		return false
	case g.cross:
		return kind != "check" && kind != "exclusion"
	}
	return true
}

// createTable writes the CREATE TABLE of a table.
func (g *generator) createTable(schema string, t Table) (string, error) {
	if t.OfType != "" && !g.cross && g.style.ObjectTables {
		owner, name, _ := strings.Cut(t.OfType, ".")
		return "CREATE TABLE " + g.qualified(schema, t.Name) + " OF " + g.qualified(owner, name), nil
	}
	if g.style.TableText && !g.cross && t.Definition != "" {
		return strings.TrimRight(strings.TrimSpace(t.Definition), "; \n\t"), nil
	}
	q, lit := g.style.Quote, g.style.Literal
	var parts []string
	pk := map[string]bool{}
	if g.style.PrimaryKeyNotNull {
		for _, c := range t.Constraints {
			if c.Kind == "primary key" {
				for _, name := range c.Columns {
					pk[name] = true
				}
			}
		}
	}
	for _, c := range t.Columns {
		cp, ok := g.plan[schema+"."+t.Name+"."+c.Name]
		if !ok {
			return "", fmt.Errorf("the column %s.%s.%s has no plan: %w", schema, t.Name, c.Name, ErrNoEquivalent)
		}
		col := g.column(c)
		if pk[c.Name] {
			col.Nullable = false
		}
		if g.style.Column != nil {
			parts = append(parts, g.style.Column(col, cp.SQL, q, lit))
		} else {
			parts = append(parts, genericColumn(col, cp.SQL, q))
		}
	}
	for _, c := range t.Constraints {
		// A table of reference partitioning has the foreign key that names its partitions in
		// the statement that makes it.
		inline := t.Partitioning != nil && t.Partitioning.Reference != "" && c.Name == t.Partitioning.Reference && c.Kind == "foreign key"
		if (!g.inCreate(c.Kind) && !inline) || g.refused(schema, t, c) != "" {
			continue
		}
		if text, ok := g.constraintSQL(schema, t, c); ok {
			parts = append(parts, text)
		}
	}
	if t.Versioning != nil && g.versioned() {
		parts = append(parts, g.style.SystemTime(*t.Versioning, q))
	}
	keyword := "TABLE"
	if g.style.TableKeyword != nil && !g.cross {
		keyword = g.style.TableKeyword(t)
	}
	sql := "CREATE " + keyword + " " + g.qualified(schema, t.Name) + " (\n  " + strings.Join(parts, ",\n  ") + "\n)"
	if g.style.TableOptions != nil {
		sql += g.style.TableOptions(t, lit)
	}
	if t.Partitioning != nil && g.partitioned() {
		sql += g.style.PartitionBy(*t.Partitioning, q, lit)
	}
	return sql, nil
}

// lostValues says which columns of a table the dump moves no value of, for a product
// that has such a column, so that a client can warn about it. See D295.
func (g *generator) lostValues(t Table) string {
	if g.style.LostValues == nil || g.cross {
		return ""
	}
	var names []string
	for _, c := range t.Columns {
		if g.style.LostValues(c) {
			names = append(names, c.Name)
		}
	}
	if len(names) == 0 {
		return ""
	}
	return "the dump moves no value of " + strings.Join(names, ", ") + ", so the copy has NULL"
}

// droppedRules says which foreign key rules a product that has none leaves out.
func (g *generator) droppedRules(t Table) string {
	if !g.style.NoForeignKeyActions {
		return ""
	}
	var names []string
	for _, c := range t.Constraints {
		if c.Kind != "foreign key" {
			continue
		}
		if del, upd := foreignKeyActions(c); (del != "" && del != "NO ACTION") || (upd != "" && upd != "NO ACTION") {
			names = append(names, c.Name)
		}
	}
	if len(names) == 0 {
		return ""
	}
	return "the target has no ON DELETE and ON UPDATE, so the rules of these foreign keys are not written: " + strings.Join(names, ", ")
}

// genericColumn writes a column as the name, the type, the default and NOT NULL.
func genericColumn(c Column, typeSQL string, q func(string) string) string {
	var b strings.Builder
	b.WriteString(q(c.Name) + " " + typeSQL)
	if c.Default != nil {
		b.WriteString(" DEFAULT " + *c.Default)
	}
	if !c.Nullable {
		b.WriteString(" NOT NULL")
	}
	return b.String()
}

// constraintSQL writes a constraint as it appears in CREATE TABLE and after ALTER
// TABLE ... ADD. The second result is false for a constraint that the source
// kept no text for, and the caller says so in a note.
//
// For the same dialect the definition of the source is used. For another dialect
// a primary key, a unique key and a foreign key are built from their columns,
// and a check keeps the text of the source.
func (g *generator) constraintSQL(schema string, t Table, c Constraint) (string, bool) {
	q := g.style.Quote
	cols := make([]string, len(c.Columns))
	for i, name := range c.Columns {
		cols[i] = q(name)
	}
	body := c.Definition
	switch c.Kind {
	case "primary key":
		body = "PRIMARY KEY" + g.keyKind(t, c) + " (" + strings.Join(cols, ", ") + ")"
	case "unique":
		if body == "" || g.cross {
			body = "UNIQUE" + g.keyKind(t, c) + " (" + strings.Join(cols, ", ") + ")"
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
		switch {
		case c.Ref != nil && (body == "" || g.cross):
			body = g.foreignKey(schema, c, cols)
		case c.Ref != nil && g.style.ReferenceTable != nil:
			body = g.style.ReferenceTable(body, *c.Ref, q)
		}
	}
	if body == "" {
		return "", false
	}
	// MySQL names every primary key PRIMARY, which is not a name that a person gave.
	if g.style.AnonymousConstraints || c.Name == "" || (c.Kind == "primary key" && c.Name == "PRIMARY") ||
		(g.cross && g.sourceNamesAreMadeUp()) {
		return body, true
	}
	return "CONSTRAINT " + q(c.Name) + " " + body, true
}

// sourceNamesAreMadeUp reports whether the source keeps no name that a person
// gave a constraint, so that the names in the snapshot are the model's own and
// another dialect does not keep them.
func (g *generator) sourceNamesAreMadeUp() bool {
	return g.source == dbmeta.SQLite3 || g.source == dbmeta.DuckDB
}

// foreignKey builds a foreign key from its columns and the rules that the source
// gave, with the words that every product knows.
func (g *generator) foreignKey(schema string, c Constraint, cols []string) string {
	q := g.style.Quote
	ref := make([]string, len(c.Ref.Columns))
	for i, name := range c.Ref.Columns {
		ref[i] = q(name)
	}
	refSchema := c.Ref.Schema
	if refSchema == "" {
		refSchema = schema
	}
	body := "FOREIGN KEY (" + strings.Join(cols, ", ") + ") REFERENCES " +
		g.qualified(refSchema, c.Ref.Table) + " (" + strings.Join(ref, ", ") + ")"
	del, upd := foreignKeyActions(c)
	if g.style.NoForeignKeyActions {
		return body
	}
	if g.style.NoRestrict {
		// RESTRICT is NO ACTION in a product that checks at the end of the statement.
		if del == "RESTRICT" {
			del = ""
		}
		if upd == "RESTRICT" {
			upd = ""
		}
	}
	if del != "" && del != "NO ACTION" {
		body += " ON DELETE " + del
	}
	if upd != "" && upd != "NO ACTION" {
		body += " ON UPDATE " + upd
	}
	if g.style.DeferrableKeys && c.Deferrable && !g.cross {
		body += " DEFERRABLE"
		if c.Deferred {
			body += " INITIALLY DEFERRED"
		}
	}
	return body
}

// post writes the statements that come after the rows.
func (g *generator) post() {
	for _, text := range g.style.AfterData {
		st := Statement{Stage: Post, Kind: "setting", SQL: text}
		if text != "" && text == g.style.CheckAfterData {
			st.Kind, st.Check = "check", true
			st.Note = "this statement returns a row for each fault in the data, and the dump fails when it returns one"
		}
		g.add(st)
	}
	g.constraints()
	g.indexes()
	g.versioningOn()
	if !g.cross {
		for _, sc := range g.snap.Schemas {
			for _, q := range sc.Sequences {
				if q.OwnedBy == "" || q.Identity || g.style.OwnedBy == nil {
					continue
				}
				g.add(Statement{Stage: Post, Kind: "sequence owner", Name: sc.Name + "." + q.Name,
					SQL: g.style.OwnedBy(g.qualified(sc.Name, q.Name), q.OwnedBy)})
			}
		}
	}
	g.identities()
	g.sequenceValues()
	g.viewsAndRoutines()
	for _, sc := range g.snap.Schemas {
		for _, t := range sc.Tables {
			for _, tg := range triggersInOrder(t.Triggers) {
				g.trigger(sc.Name, t, tg)
			}
		}
		for _, v := range sc.Views {
			for _, tg := range v.Triggers {
				g.trigger(sc.Name, Table{Name: v.Name}, tg)
			}
		}
		g.events(sc)
	}
	g.grants()
	g.policies()
	g.viewLogs()
	g.eventTriggers()
	g.comments()
}

// constraints writes the keys and checks that are not inside CREATE TABLE. A
// unique key and a check come before a foreign key, because a foreign key needs
// the key that it points at.
func (g *generator) constraints() {
	for _, kind := range []string{"unique", "exclusion", "check", "foreign key"} {
		if g.inCreate(kind) {
			continue
		}
		for _, sc := range g.snap.Schemas {
			for _, t := range sc.Tables {
				for _, c := range t.Constraints {
					if c.Kind != kind || (t.Partitioning != nil && t.Partitioning.Reference != "" && c.Name == t.Partitioning.Reference) {
						continue
					}
					name := sc.Name + "." + t.Name + "." + c.Name
					if why := g.refused(sc.Name, t, c); why != "" {
						g.add(Statement{Stage: Post, Kind: "constraint", Name: name, Note: why})
						continue
					}
					text, ok := g.constraintSQL(sc.Name, t, c)
					if !ok {
						g.add(Statement{Stage: Post, Kind: "constraint", Name: name,
							Note: "the source keeps no text for this " + c.Kind + ", so it is not written"})
						continue
					}
					if kind == "foreign key" && g.style.ReferenceGrant != nil && c.Ref != nil && c.Ref.Schema != "" && c.Ref.Schema != sc.Name {
						g.add(Statement{Stage: Post, Kind: "grant", Name: name,
							SQL: g.style.ReferenceGrant(g.qualified(c.Ref.Schema, c.Ref.Table), g.style.Quote(sc.Name))})
					}
					g.add(Statement{Stage: Post, Kind: "constraint", Name: name,
						SQL:          "ALTER TABLE " + g.qualified(sc.Name, t.Name) + " ADD " + text,
						Untranslated: g.cross && (kind == "check" || kind == "exclusion")})
				}
			}
		}
	}
}

// refused returns why the target cannot hold a constraint of a table, or the empty
// string when it can.
func (g *generator) refused(schema string, t Table, c Constraint) string {
	if g.style.SameSchemaKeys && c.Kind == "foreign key" && c.Ref != nil && c.Ref.Schema != "" && c.Ref.Schema != schema {
		return "the target has no foreign key across schemas, so " + c.Name + " is not written"
	}
	if g.style.RefuseConstraint == nil {
		return ""
	}
	return g.style.RefuseConstraint(t, c)
}

// indexes writes the indexes that no constraint writes.
func (g *generator) indexes() {
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
				if ix.Unique {
					cols := make([]string, len(ix.Columns))
					for i, c := range ix.Columns {
						cols[i] = c.Name
					}
					if why := g.refused(sc.Name, t, Constraint{Name: ix.Name, Kind: "unique", Columns: cols}); why != "" {
						g.add(Statement{Stage: Post, Kind: "index", Name: name, Note: why})
						continue
					}
				}
				if g.cross && len(ix.Columns) == 0 {
					ix.Columns = definitionColumns(ix.Definition)
				}
				if (g.cross || ix.Definition == "") && len(ix.Columns) == 0 {
					g.add(Statement{Stage: Post, Kind: "index", Name: name,
						Note: "the source keeps no columns for this index, so it is not written"})
					continue
				}
				st := Statement{Stage: Post, Kind: "index", Name: name, SQL: g.index(sc.Name, t, ix)}
				if g.cross && (ix.Predicate != "" || slices.ContainsFunc(ix.Columns, func(c IndexColumn) bool { return c.Expression != "" })) {
					st.Untranslated = true
				}
				g.add(st)
			}
		}
	}
}

// index writes a CREATE INDEX. For the same dialect the definition that the
// source keeps is used when there is one, because it holds the parts that the
// columns do not.
func (g *generator) index(schema string, t Table, ix Index) string {
	table := g.qualified(schema, t.Name)
	if ix.Definition != "" && !g.cross {
		def := strings.TrimRight(strings.TrimSpace(ix.Definition), "; \n\t")
		// The source writes ON ONLY for the index of a partitioned table, which makes
		// no index on the partitions. The partitions are there already, so the index is
		// made on all of them.
		if t.Partitioning != nil {
			def = strings.Replace(def, " ON ONLY ", " ON ", 1)
		}
		return def
	}
	sql := genericIndex(table, ix, g.style.Quote)
	if g.style.Index != nil {
		sql = g.style.Index(table, ix, g.style.Quote)
	}
	if ix.Invisible && g.style.InvisibleIndex != nil && !g.cross {
		sql += g.style.InvisibleIndex(g.snap.Server)
	}
	return sql
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

// identities writes the statements that set the next value of each identity
// column from the rows, for a product that does not follow the rows itself.
func (g *generator) identities() {
	if g.style.IdentityValue == nil && g.style.IdentityAfter == nil {
		return
	}
	for _, sc := range g.snap.Schemas {
		for _, t := range sc.Tables {
			if t.IsPartition() {
				continue
			}
			for _, c := range t.Columns {
				if c.Identity == "" && (g.cross || c.Default == nil || !isNextval(*c.Default)) {
					continue
				}
				var sql string
				if g.style.IdentityAfter != nil {
					sql = g.style.IdentityAfter(g.qualified(sc.Name, t.Name), g.style.Quote(c.Name), c)
				} else {
					sql = g.style.IdentityValue(g.qualified(sc.Name, t.Name), g.style.Quote(c.Name), c.Name, g.style.Literal)
				}
				g.add(Statement{Stage: Post, Kind: "sequence value", Name: sc.Name + "." + t.Name + "." + c.Name, SQL: sql})
			}
		}
	}
}

// views writes the views, each after the views that it reads. A source that did
// not say what a view reads gets the views in order of name, and the first
// statement says that the order is not checked, so that a client can run a view
// that failed again once the others exist.
func (g *generator) viewsAndRoutines() {
	type entry struct {
		schema string
		view   View
		// routine is set for a routine that is written with the views.
		routine *Routine
		words   map[string]bool
	}
	var all []entry
	index := map[string]int{}
	for _, sc := range g.snap.Schemas {
		for _, v := range sc.Views {
			index[sc.Name+"."+v.Name] = len(all)
			all = append(all, entry{schema: sc.Name, view: v})
		}
	}
	byName := map[string][]int{}
	if g.style.RoutinesAfterTables {
		early := g.earlyRoutineSet()
		for _, sc := range g.snap.Schemas {
			for _, r := range sc.Routines {
				if early[sc.Name+"."+r.Name] {
					continue
				}
				index[sc.Name+"."+r.Name] = len(all)
				all = append(all, entry{schema: sc.Name, routine: &r})
			}
		}
		// A routine and a view find each other by the names in their text.
		for i := range all {
			text := all[i].view.Definition
			if all[i].routine != nil {
				text = all[i].routine.Definition
			}
			all[i].words = identifiers(text)
			name := all[i].view.Name
			if all[i].routine != nil {
				name = all[i].routine.Name
			}
			byName[strings.ToLower(name)] = append(byName[strings.ToLower(name)], i)
		}
	}
	state := make([]int, len(all))
	var order []int
	var visit func(i int)
	visit = func(i int) {
		if state[i] != 0 {
			return
		}
		state[i] = 1
		for _, dep := range all[i].view.DependsOn {
			if j, ok := index[dep]; ok {
				visit(j)
			}
		}
		// The words are in order of the names, so two scripts of one snapshot are the same.
		for _, word := range slices.Sorted(maps.Keys(all[i].words)) {
			for _, j := range byName[word] {
				// An object names itself, and its body and its specification
				// follow the order of the snapshot.
				if j != i && !sameObject(all[i].schema, nameOf(all[i].view.Name, all[i].routine),
					all[j].schema, nameOf(all[j].view.Name, all[j].routine)) {
					visit(j)
				}
			}
		}
		order = append(order, i)
	}
	for i := range all {
		visit(i)
	}
	for n, i := range order {
		e := all[i]
		if e.routine != nil {
			g.grantsFor(e.schema, e.routine.Definition)
			g.routine(Post, e.schema, *e.routine)
			continue
		}
		g.grantsFor(e.schema, e.view.Definition)
		g.use(Post, e.schema)
		st := Statement{Stage: Post, Kind: "view", Name: e.schema + "." + e.view.Name, SQL: g.view(e.schema, e.view), Untranslated: g.cross}
		if e.view.Materialized {
			st.Kind = "materialized view"
			if !g.materialized() {
				st.Note = "the source has a materialized view here and the target has none, so this is a plain view that reads its tables each time"
			}
		}
		if n == 0 && len(all) > 1 && !g.snap.ViewDependencies {
			st.Note = "the order of the views is by name and is not checked, because the source did not say what each view reads"
		}
		g.add(st)
		if g.materialized() {
			for _, ix := range e.view.Indexes {
				g.add(Statement{Stage: Post, Kind: "index", Name: e.schema + "." + e.view.Name + "." + ix.Name,
					SQL: g.index(e.schema, Table{Name: e.view.Name}, ix)})
			}
		}
	}
}

// grantsFor writes the grants that a view or a routine needs on the objects of
// another schema that its text names, for a product that wants them given directly.
// The grant comes before the object, and is written once. See D294.
func (g *generator) grantsFor(schema, text string) {
	if g.style.ObjectGrant == nil || g.cross || text == "" {
		return
	}
	if g.granted == nil {
		g.granted = map[string]bool{}
	}
	mentions := func(other, name string) bool {
		re := regexp.MustCompile(`(?i)\b"?` + regexp.QuoteMeta(other) + `"?\s*\.\s*"?` + regexp.QuoteMeta(name) + `"?(?:[^A-Za-z0-9_$#]|$)`)
		return re.MatchString(text)
	}
	give := func(kind, other, name string) {
		key := schema + "<" + other + "." + name
		if g.granted[key] || !mentions(other, name) {
			return
		}
		g.granted[key] = true
		g.add(Statement{Stage: Post, Kind: "grant", Name: other + "." + name,
			SQL: g.style.ObjectGrant(kind, g.qualified(other, name), g.style.Quote(schema))})
	}
	for _, sc := range g.snap.Schemas {
		if sc.Name == schema {
			continue
		}
		for _, t := range sc.Tables {
			give("table", sc.Name, t.Name)
		}
		for _, v := range sc.Views {
			give("table", sc.Name, v.Name)
		}
		for _, q := range sc.Sequences {
			give("sequence", sc.Name, q.Name)
		}
		for _, r := range sc.Routines {
			give("routine", sc.Name, r.Name)
		}
	}
}

// view writes a view from its definition. For another dialect the head of the
// source's own statement is taken off, and the target writes its own.
func (g *generator) view(schema string, v View) string {
	name := g.qualified(schema, v.Name)
	// A product whose schema is its database keeps the name of the database in the
	// query of a view, and the copy lives in another database.
	if src, err := styleOf(g.source); err == nil && src.DefaultSchema == "*" && schema != "" && !g.severalDatabases() {
		v.Definition = strings.ReplaceAll(v.Definition, src.Quote(schema)+".", "")
	}
	if g.cross || (v.Materialized && !g.materialized()) {
		return "CREATE VIEW " + name + " AS " + viewQuery(v.Definition)
	}
	if v.Materialized {
		return g.style.MaterializedView(name, v)
	}
	if g.style.View != nil {
		return g.style.View(name, v)
	}
	def := strings.TrimRight(strings.TrimSpace(v.Definition), "; \n\t")
	sql := "CREATE VIEW " + name + " AS " + def
	// A view that checks the rows that go through it says so, and the source
	// keeps it apart from the query.
	if option := strings.ToLower(v.CheckOption); option == "local" || option == "cascaded" {
		sql += " WITH " + strings.ToUpper(option) + " CHECK OPTION"
	}
	return sql
}

// events writes the scheduled events of a schema, with the mode of the session that
// made each one.
func (g *generator) events(sc Schema) {
	if g.style.Event == nil || g.cross {
		return
	}
	for _, e := range sc.Events {
		g.use(Post, sc.Name)
		if e.SQLMode != nil && g.style.SQLMode != nil {
			g.add(Statement{Stage: Post, Kind: "setting", Name: sc.Name + "." + e.Name, SQL: g.style.SQLMode(*e.SQLMode)})
		}
		g.add(Statement{Stage: Post, Kind: "event", Name: sc.Name + "." + e.Name,
			SQL: g.style.Event(g.qualified(sc.Name, e.Name), e, g.style.Quote, g.style.Literal)})
		if e.SQLMode != nil && g.style.SQLMode != nil {
			g.add(Statement{Stage: Post, Kind: "setting", Name: sc.Name + "." + e.Name, SQL: g.style.ResetSQLMode})
		}
	}
}

// triggersInOrder returns the triggers of a table in the order that a script writes
// them. A trigger that has a position follows the one before it among those of the
// same timing and event, so it comes after that one and names it.
func triggersInOrder(triggers []Trigger) []Trigger {
	out := slices.Clone(triggers)
	slices.SortStableFunc(out, func(a, b Trigger) int { return cmp.Compare(a.Order, b.Order) })
	for i := range out {
		if out[i].Order < 2 {
			continue
		}
		key := triggerGroup(out[i])
		for j := range out {
			if out[j].Order == out[i].Order-1 && triggerGroup(out[j]) == key {
				out[i].Follows = out[j].Name
			}
		}
	}
	return out
}

// triggerGroup is the timing and the event of a trigger, from the head of its
// definition, such as AFTER UPDATE.
func triggerGroup(tg Trigger) string {
	words := strings.Fields(strings.ToUpper(tg.Definition))
	if len(words) < 2 {
		return ""
	}
	return words[0] + " " + words[1]
}

// trigger writes a trigger from what the source keeps.
func (g *generator) trigger(schema string, t Table, tg Trigger) {
	g.use(Post, schema)
	if tg.SQLMode != nil && g.style.SQLMode != nil && !g.cross {
		g.add(Statement{Stage: Post, Kind: "setting", Name: schema + "." + t.Name + "." + tg.Name, SQL: g.style.SQLMode(*tg.SQLMode)})
		defer g.add(Statement{Stage: Post, Kind: "setting", Name: schema + "." + t.Name + "." + tg.Name, SQL: g.style.ResetSQLMode})
	}
	name := schema + "." + t.Name + "." + tg.Name
	def := strings.TrimSpace(tg.Definition)
	hasStatement := strings.HasPrefix(strings.ToLower(def), "create")
	missing := Statement{Stage: Post, Kind: "trigger", Name: name, Untranslated: g.cross,
		Note: "the source keeps no statement that creates this trigger, so it is not written"}
	switch {
	case g.cross && hasStatement:
		g.add(Statement{Stage: Post, Kind: "trigger", Name: name, Untranslated: true, SQL: g.endOf(def)})
	case g.cross:
		g.add(missing)
	case g.style.Trigger != nil:
		sql := g.style.Trigger(g.qualified(schema, t.Name), tg, g.style.Quote)
		if sql == "" {
			missing.Note = "the definition of this trigger is not in a form that the dump reads, so it is not written"
			g.add(missing)
			return
		}
		g.add(Statement{Stage: Post, Kind: "trigger", Name: name, SQL: sql})
		g.triggerState(schema, t, tg)
		g.triggerPlacement(schema, tg)
	case hasStatement:
		g.add(Statement{Stage: Post, Kind: "trigger", Name: name, SQL: g.endOf(def)})
		g.triggerState(schema, t, tg)
		g.triggerPlacement(schema, tg)
	default:
		g.add(missing)
	}
}

// triggerState switches off a trigger that the source has switched off, or sets
// it for the replica role.
func (g *generator) triggerState(schema string, t Table, tg Trigger) {
	state := strings.ToLower(tg.Enabled)
	if g.style.TriggerState == nil || state == "" || state == "enabled" {
		return
	}
	g.add(Statement{Stage: Post, Kind: "trigger state", Name: schema + "." + t.Name + "." + tg.Name,
		SQL: g.style.TriggerState(g.style.Quote(schema), g.qualified(schema, t.Name), g.style.Quote(tg.Name), state)})
}

// comments writes the comment of each table and column, for a product that sets
// them with a statement.
func (g *generator) comments() {
	if g.style.Comment == nil {
		return
	}
	if !g.cross {
		g.typeComments()
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

// isNextval reports whether a default reads the next value of a sequence, which is
// how PostgreSQL writes a serial column.
func isNextval(def string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(def)), "nextval(")
}

// keyKind writes the word that says how the index of a key is stored, for a
// product whose key has one. It reads the index that has the name of the key, and
// is empty when the index is stored as the product stores a key by default or when
// the product has no such word.
func (g *generator) keyKind(t Table, c Constraint) string {
	if g.cross || g.style.KeyKind == nil {
		return ""
	}
	for _, ix := range t.Indexes {
		if ix.Name == c.Name {
			return g.style.KeyKind(c.Kind, ix)
		}
	}
	return ""
}

// nameOf is the name of a view or of a routine.
func nameOf(view string, r *Routine) string {
	if r != nil {
		return r.Name
	}
	return view
}

// synonyms writes the synonyms of the schemas, which need not point at an object
// that exists yet. Another product has no synonym, and the script says so.
func (g *generator) synonyms() {
	for _, sc := range g.snap.Schemas {
		for _, y := range sc.Synonyms {
			name := sc.Name + "." + y.Name
			if g.cross || g.style.Synonym == nil {
				g.add(Statement{Stage: Pre, Kind: "synonym", Name: name,
					Note: "the target has no synonym, so this one is not written. It names " + y.Target})
				continue
			}
			g.add(Statement{Stage: Pre, Kind: "synonym", Name: name, SQL: g.style.Synonym(g.qualified(sc.Name, y.Name), y, g.style.Quote)})
		}
	}
}

// sameObject says that two stored objects have one name, such as a package and its
// body, which follow the order of the snapshot and need no order of their own.
func sameObject(schemaA, nameA, schemaB, nameB string) bool {
	return schemaA == schemaB && strings.EqualFold(nameA, nameB)
}

// triggerPlacement makes a trigger first or last for an event, where the source did.
func (g *generator) triggerPlacement(schema string, tg Trigger) {
	if g.style.TriggerPlacement == nil || tg.Placement == "" || g.cross {
		return
	}
	for _, text := range g.style.TriggerPlacement(schema, tg.Name, tg.Placement, g.style.Quote) {
		g.add(Statement{Stage: Post, Kind: "trigger placement", Name: schema + "." + tg.Name, SQL: text})
	}
}

// partitionObjects writes the partition functions and the partition schemes, which a
// table that is laid out by a scheme needs first.
func (g *generator) partitionObjects() {
	if g.style.PartitionFunction == nil {
		return
	}
	for _, f := range g.snap.PartitionFunctions {
		g.add(Statement{Stage: Pre, Kind: "partition function", Name: f.Name, SQL: g.style.PartitionFunction(f, g.style.Quote)})
	}
	if g.style.PartitionScheme != nil {
		for _, sc := range g.snap.PartitionSchemes {
			g.add(Statement{Stage: Pre, Kind: "partition scheme", Name: sc.Name, SQL: g.style.PartitionScheme(sc, g.style.Quote)})
		}
	}
}

// xmlSchemas writes the XML schema collections, which a column of the type xml can
// name.
func (g *generator) xmlSchemas() {
	if g.style.XMLSchema == nil {
		return
	}
	for _, sc := range g.snap.Schemas {
		for _, x := range sc.XMLSchemas {
			g.add(Statement{Stage: Pre, Kind: "xml schema collection", Name: sc.Name + "." + x.Name,
				SQL: g.style.XMLSchema(g.qualified(sc.Name, x.Name), x, g.style.Literal)})
		}
	}
}

// policies writes the row level security policies of each table, after the tables and
// the functions that a predicate calls.
func (g *generator) policies() {
	if g.style.Policies == nil || g.cross {
		return
	}
	created := map[string]bool{}
	for _, sc := range g.snap.Schemas {
		for _, t := range sc.Tables {
			if len(t.Policies) == 0 && !t.RowSecurity {
				continue
			}
			for _, text := range g.style.Policies(sc.Name, t, created, g.style.Quote, g.style.Literal) {
				g.add(Statement{Stage: Post, Kind: "policy", Name: sc.Name + "." + t.Name, SQL: text})
			}
		}
	}
}

// eventTriggers writes the triggers that fire on a DDL event.
func (g *generator) eventTriggers() {
	if g.style.EventTrigger == nil || g.cross {
		return
	}
	for _, e := range g.snap.EventTriggers {
		if e.Schema != "" {
			g.use(Post, e.Schema)
		}
		text := g.style.EventTrigger(e, g.style.Quote, g.style.Literal)
		if text == "" {
			continue
		}
		g.add(Statement{Stage: Post, Kind: "event trigger", Name: e.Name, SQL: text})
		if state := strings.ToLower(e.Enabled); g.style.TriggerState != nil && state != "" && state != "enabled" && state != "o" && e.Schema == "" {
			if sql := g.style.TriggerState("", "", g.style.Quote(e.Name), state); sql != "" {
				g.add(Statement{Stage: Post, Kind: "trigger state", Name: e.Name, SQL: sql})
			}
		}
	}
}

// sequenceValues sets the next value of each sequence that the rows do not set, for a
// product that can.
func (g *generator) sequenceValues() {
	if g.style.SequenceValue == nil || g.cross {
		return
	}
	for _, sc := range g.snap.Schemas {
		for _, q := range sc.Sequences {
			if q.Last == "" || q.OwnedBy != "" || q.Identity {
				continue
			}
			sql := g.style.SequenceValue(g.qualified(sc.Name, q.Name), q, g.style.Literal)
			if sql == "" {
				continue
			}
			g.add(Statement{Stage: Post, Kind: "sequence value", Name: sc.Name + "." + q.Name, SQL: sql})
		}
	}
}

// viewLogs writes the logs of the changes to a table, after the tables.
func (g *generator) viewLogs() {
	if g.style.MaterializedViewLog == nil || g.cross {
		return
	}
	for _, sc := range g.snap.Schemas {
		for _, l := range sc.Logs {
			g.add(Statement{Stage: Post, Kind: "materialized view log", Name: sc.Name + "." + l.Master,
				SQL: g.style.MaterializedViewLog(g.qualified(sc.Name, l.Master), l)})
		}
	}
}

// roles writes the roles that the grants name, and the memberships among them, which
// come before the objects that the grants are on.
func (g *generator) roles() {
	if g.cross {
		return
	}
	g.principals()
	if g.style.Role != nil {
		for _, r := range g.snap.Roles {
			g.add(Statement{Stage: Pre, Kind: "role", Name: r.Name, SQL: g.style.Role(r, g.style.Quote, g.style.Literal)})
		}
	}
	if g.style.Membership == nil {
		return
	}
	for _, r := range g.snap.Roles {
		for _, of := range r.MemberOf {
			g.add(Statement{Stage: Pre, Kind: "role membership", Name: r.Name + "." + of, SQL: g.style.Membership(of, r.Name, g.style.Quote)})
		}
	}
}

// grants writes the grants of each object, after the objects.
func (g *generator) grants() {
	if g.style.Grant == nil || g.cross {
		return
	}
	type object struct{ schema, name, kind, arguments string }
	var order []object
	by := map[object][]GrantDef{}
	for _, gr := range g.snap.Grants {
		k := object{gr.Schema, gr.Object, gr.Kind, gr.Arguments}
		if _, ok := by[k]; !ok {
			order = append(order, k)
		}
		by[k] = append(by[k], gr)
	}
	for _, k := range order {
		qualified := g.qualified(k.schema, k.name)
		if k.kind == "schema" {
			qualified = ""
			if q := g.qualified(k.schema, "x"); q != g.style.Quote("x") {
				qualified = g.style.Quote(k.schema)
			}
		}
		for _, text := range g.style.Grant(k.schema, k.name, k.kind, qualified, by[k], g.style.Quote) {
			g.add(Statement{Stage: Post, Kind: "grant", Name: k.schema + "." + k.name, SQL: text})
		}
	}
}

// principals makes the roles and the accounts that the grants name, for a product that
// does not read its roles with the grants. A schema of the snapshot is made already.
func (g *generator) principals() {
	if g.style.Principal == nil || g.cross {
		return
	}
	schemas := map[string]bool{}
	for _, sc := range g.snap.Schemas {
		schemas[sc.Name] = true
	}
	seen := map[string]bool{}
	for _, gr := range g.snap.Grants {
		if gr.Grantee == "" || seen[gr.Grantee] || schemas[gr.Grantee] {
			continue
		}
		seen[gr.Grantee] = true
		g.add(Statement{Stage: Pre, Kind: "role", Name: gr.Grantee, SQL: g.style.Principal(gr.Grantee, gr.GranteeType, g.style.Quote)})
	}
}
