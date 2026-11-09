package dump

import (
	"cmp"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"iter"
	"slices"
	"strings"

	"github.com/xo/dbmeta"
)

// Filter says which schemas and tables Read puts in the snapshot. A pattern is
// the pattern of SQL LIKE, where % is any run of characters and _ is any one,
// and a backslash makes the next one plain. The lists are empty for every one.
//
// A table pattern with a dot, such as app.%, is matched against the schema and
// the table together.
type Filter struct {
	// Schemas and Tables are the names to include.
	Schemas []string
	Tables  []string
	// ExcludeSchemas and ExcludeTables are the names to leave out.
	ExcludeSchemas []string
	ExcludeTables  []string
	// WithSystem includes the objects that the database keeps for itself.
	WithSystem bool
}

// Read reads the schema of a database into a [Snapshot], with one query for each
// kind of object and not one for each table. A kind that the product does not
// answer is empty, and the snapshot says nothing about it. Read never reads a row
// of data.
func Read(ctx context.Context, m *dbmeta.Meta, db dbmeta.Queryer, f Filter) (*Snapshot, error) {
	r := &reader{m: m, db: db, filter: f}
	if err := r.scope(ctx); err != nil {
		return nil, err
	}
	snap := &Snapshot{
		Version: SnapshotVersion,
		Dialect: string(m.Dialect()),
		Server:  m.Version().String(),
		Schemas: []Schema{},
	}
	if err := r.read(ctx, snap); err != nil {
		return nil, err
	}
	snap.ViewDependencies = r.viewDependencies
	return snap, nil
}

// scope narrows a read with no schema named to the database of the connection, for
// a product whose schema is its database, so that a dump of one database does not
// take every database of the server. A connection that names no database reads
// them all.
func (r *reader) scope(ctx context.Context) error {
	if len(r.filter.Schemas) > 0 || dbmeta.CurrentDatabase.Support(r.m) != dbmeta.Supported || !schemaIsDatabase(r.m.Dialect()) {
		return nil
	}
	rows, err := collect(dbmeta.CurrentDatabase.Each(ctx, r.m, r.db, dbmeta.Args{}))
	if err != nil {
		return fmt.Errorf("reading the current database: %w", err)
	}
	if len(rows) == 1 && rows[0].Name.Valid && rows[0].Name.V != "" {
		escaped := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(rows[0].Name.V)
		r.filter.Schemas = []string{escaped}
	}
	return nil
}

// schemaIsDatabase reports whether the schema of a product is its database.
func schemaIsDatabase(d dbmeta.Dialect) bool {
	style, err := styleOf(d)
	return err == nil && style.DefaultSchema == "*"
}

// tableKey names a table in a map.
type tableKey struct{ schema, table string }

// childKey names an index or a constraint of a table in a map.
type childKey struct{ schema, table, name string }

type reader struct {
	// viewDependencies is set when the source answered the dependencies of views.
	viewDependencies bool
	// columns are the columns that readColumns read, in order of position, which
	// the composite types are made from too.
	columns []dbmeta.Column

	m      *dbmeta.Meta
	db     dbmeta.Queryer
	filter Filter
}

// args is the argument that narrows a query to the one schema or table that the
// filter names, when it names exactly one. The filter is applied again over the
// rows, because a query takes one pattern and the filter takes a list.
func (r *reader) args() dbmeta.Args {
	a := dbmeta.Args{WithSystem: r.filter.WithSystem}
	if len(r.filter.Schemas) == 1 {
		a.Schema = r.filter.Schemas[0]
	}
	return a
}

// collect reads every row of a query. A query that the product does not answer
// gives no rows and no error.
func collect[T any](seq iter.Seq2[T, error]) ([]T, error) {
	var out []T
	for v, err := range seq {
		if err != nil {
			if errors.Is(err, dbmeta.ErrNotSupported) || errors.Is(err, dbmeta.ErrVersionTooOld) {
				return nil, nil
			}
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

// text returns the value of a nullable string, or the empty string.
func text(n sql.Null[string]) string {
	if n.Valid {
		return n.V
	}
	return ""
}

// wanted reports whether a schema passes the filter.
func (r *reader) wantSchema(name string) bool {
	if len(r.filter.Schemas) > 0 && !matchAny(r.filter.Schemas, name) {
		return false
	}
	return !matchAny(r.filter.ExcludeSchemas, name)
}

// wantTable reports whether a table passes the filter.
func (r *reader) wantTable(schema, name string) bool {
	if !r.wantSchema(schema) {
		return false
	}
	qualified := schema + "." + name
	hit := func(patterns []string) bool {
		for _, p := range patterns {
			if like(p, name) || strings.Contains(p, ".") && like(p, qualified) {
				return true
			}
		}
		return false
	}
	if len(r.filter.Tables) > 0 && !hit(r.filter.Tables) {
		return false
	}
	return !hit(r.filter.ExcludeTables)
}

func (r *reader) read(ctx context.Context, snap *Snapshot) error {
	a := r.args()
	schemas, err := collect(dbmeta.Schemas.Each(ctx, r.m, r.db, a))
	if err != nil {
		return fmt.Errorf("reading schemas: %w", err)
	}
	byName := map[string]*Schema{}
	add := func(name, comment string) *Schema {
		if s, ok := byName[name]; ok {
			return s
		}
		snap.Schemas = append(snap.Schemas, Schema{Name: name, Comment: comment})
		byName[name] = &snap.Schemas[len(snap.Schemas)-1]
		return byName[name]
	}
	// The slice can move when it grows, so the schemas are all added before any
	// pointer is kept.
	names := map[string]string{}
	for _, s := range schemas {
		if r.wantSchema(s.Name) {
			names[s.Name] = text(s.Comment)
		}
	}
	tables, err := collect(dbmeta.Tables.Each(ctx, r.m, r.db, a))
	if err != nil {
		return fmt.Errorf("reading tables: %w", err)
	}
	for _, t := range tables {
		if r.wantTable(t.Schema, t.Name) {
			if _, ok := names[t.Schema]; !ok {
				names[t.Schema] = ""
			}
		}
	}
	sorted := make([]string, 0, len(names))
	for n := range names {
		sorted = append(sorted, n)
	}
	slices.Sort(sorted)
	snap.Schemas = make([]Schema, 0, len(sorted))
	for _, n := range sorted {
		add(n, names[n])
	}

	kept := map[tableKey]*Table{}
	views := map[tableKey]*View{}
	for _, t := range tables {
		// The extension that owns a relation makes it again.
		if !r.wantTable(t.Schema, t.Name) || t.Extension.Valid {
			continue
		}
		s := byName[t.Schema]
		switch kind := strings.ToLower(strings.TrimSpace(t.Type)); kind {
		case "table", "base table":
			s.Tables = append(s.Tables, Table{Name: t.Name, Comment: text(t.Comment), Columns: []Column{}})
		case "partitioned table":
			s.Tables = append(s.Tables, Table{Name: t.Name, Comment: text(t.Comment), Columns: []Column{}})
		case "view":
			s.Views = append(s.Views, View{Name: t.Name, Comment: text(t.Comment)})
		case "materialized view":
			s.Views = append(s.Views, View{Name: t.Name, Comment: text(t.Comment), Materialized: true})
		case "sequence":
			// A sequence is read with the objects of its schema.
		default:
			snap.Skipped = append(snap.Skipped, Skipped{Schema: t.Schema, Name: t.Name, Kind: kind,
				Reason: "this kind of relation is not read"})
		}
	}
	// Pointers into the slices are taken after the last append.
	for i := range snap.Schemas {
		s := &snap.Schemas[i]
		slices.SortFunc(s.Tables, func(a, b Table) int { return cmp.Compare(a.Name, b.Name) })
		slices.SortFunc(s.Views, func(a, b View) int { return cmp.Compare(a.Name, b.Name) })
		for j := range s.Tables {
			kept[tableKey{s.Name, s.Tables[j].Name}] = &s.Tables[j]
		}
		for j := range s.Views {
			views[tableKey{s.Name, s.Views[j].Name}] = &s.Views[j]
		}
	}
	if err := r.readPartitions(ctx, a, kept, snap); err != nil {
		return err
	}
	if err := r.readColumns(ctx, a, kept); err != nil {
		return err
	}
	if err := r.readConstraints(ctx, a, kept); err != nil {
		return err
	}
	if err := r.readIndexes(ctx, a, kept, views); err != nil {
		return err
	}
	if err := r.readViews(ctx, a, views); err != nil {
		return err
	}
	if err := r.readTriggers(ctx, a, kept); err != nil {
		return err
	}
	if err := r.readObjects(ctx, a, byName); err != nil {
		return err
	}
	if err := r.readExtensions(ctx, snap); err != nil {
		return err
	}
	slices.SortFunc(snap.Skipped, func(a, b Skipped) int {
		return cmp.Or(cmp.Compare(a.Schema, b.Schema), cmp.Compare(a.Name, b.Name))
	})
	return nil
}

// readPartitions says how each partitioned table is split. PostgreSQL has a
// table for each partition, and the partition names its parent. MySQL keeps its
// partitions inside the table, so the table lists them. A product that has a
// partitioned table and is not one of these two is read as a plain table.
func (r *reader) readPartitions(ctx context.Context, a dbmeta.Args, kept map[tableKey]*Table, snap *Snapshot) error {
	d := r.m.Dialect()
	if d != dbmeta.PostgreSQL && d != dbmeta.MySQL {
		return nil
	}
	parents, err := collect(dbmeta.PartitionedTables.Each(ctx, r.m, r.db, a))
	if err != nil {
		return fmt.Errorf("reading partitioned tables: %w", err)
	}
	parts, err := collect(dbmeta.Partitions.Each(ctx, r.m, r.db, a))
	if err != nil {
		return fmt.Errorf("reading partitions: %w", err)
	}
	for _, p := range parents {
		t, ok := kept[tableKey{p.Schema, p.Name}]
		if !ok || p.Type == "partitioned index" {
			continue
		}
		if d == dbmeta.PostgreSQL {
			// pg_get_partkeydef writes the strategy and then the key in brackets.
			strategy, key, _ := strings.Cut(strings.TrimSpace(p.Expression), " ")
			t.Partitioning = &Partitioning{Strategy: strings.ToLower(strategy), Expression: unbracket(key)}
		} else {
			t.Partitioning = &Partitioning{Strategy: strings.ToLower(p.Strategy), Expression: p.Expression}
		}
	}
	for _, p := range parts {
		if d == dbmeta.PostgreSQL {
			child, ok := kept[tableKey{p.PartitionSchema, p.Partition}]
			if _, parentKept := kept[tableKey{p.Schema, p.Table}]; ok && parentKept {
				child.PartitionOf = &PartitionOf{Schema: p.Schema, Table: p.Table, Bound: partitionBound(p)}
			}
			continue
		}
		t, ok := kept[tableKey{p.Schema, p.Table}]
		if !ok || t.Partitioning == nil {
			continue
		}
		if p.Type == "subpartition" || p.Partitioned {
			snap.Skipped = append(snap.Skipped, Skipped{Schema: p.Schema, Name: p.Table, Kind: "subpartition",
				Reason: "the subpartitions of a table are not written, so the table is written with its partitions only"})
			continue
		}
		t.Partitioning.Parts = append(t.Partitioning.Parts, Part{Name: p.Partition, Bound: text(p.Bound)})
	}
	return nil
}

// partitionBound returns the text that follows the name of a partition.
func partitionBound(p dbmeta.Partition) string {
	b := text(p.Bound)
	if b == "" || strings.EqualFold(b, "default") {
		return "DEFAULT"
	}
	return b
}

// unbracket takes the outer brackets off a key.
func unbracket(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && s[0] == '(' && s[len(s)-1] == ')' {
		return s[1 : len(s)-1]
	}
	return s
}

// readColumns puts each column in its table, in order of position.
func (r *reader) readColumns(ctx context.Context, a dbmeta.Args, kept map[tableKey]*Table) error {
	cols, err := collect(dbmeta.Columns.Each(ctx, r.m, r.db, a))
	if err != nil {
		return fmt.Errorf("reading columns: %w", err)
	}
	slices.SortStableFunc(cols, func(x, y dbmeta.Column) int { return cmp.Compare(x.Ordinal, y.Ordinal) })
	r.columns = cols
	for _, c := range cols {
		t, ok := kept[tableKey{c.Schema, c.Table}]
		if !ok {
			continue
		}
		col, err := r.column(c)
		if err != nil {
			return err
		}
		t.Columns = append(t.Columns, col)
	}
	return nil
}

// column reads a column of a table or of a composite type.
func (r *reader) column(c dbmeta.Column) (Column, error) {
	typ, err := ParseType(r.m.Dialect(), c.DataType)
	if err != nil {
		return Column{}, fmt.Errorf("reading the type of %s.%s.%s: %w", c.Schema, c.Table, c.Name, err)
	}
	col := Column{
		Name: c.Name, Type: typ, Nullable: c.Nullable,
		Identity: text(c.Identity), Generated: text(c.Generated),
		Collation: text(c.Collation), Comment: text(c.Comment),
		DefaultLiteral: c.DefaultIsLiteral.Valid && c.DefaultIsLiteral.V,
	}
	if c.Default.Valid {
		d := c.Default.V
		col.Default = &d
	}
	return col, nil
}

// readConstraints puts each constraint in its table, with its columns in order.
func (r *reader) readConstraints(ctx context.Context, a dbmeta.Args, kept map[tableKey]*Table) error {
	cons, err := collect(dbmeta.Constraints.Each(ctx, r.m, r.db, a))
	if err != nil {
		return fmt.Errorf("reading constraints: %w", err)
	}
	ccols, err := collect(dbmeta.ConstraintColumns.Each(ctx, r.m, r.db, a))
	if err != nil {
		return fmt.Errorf("reading constraint columns: %w", err)
	}
	slices.SortStableFunc(ccols, func(x, y dbmeta.ConstraintColumn) int { return cmp.Compare(x.Ordinal, y.Ordinal) })
	byCons := map[childKey][]dbmeta.ConstraintColumn{}
	for _, c := range ccols {
		k := childKey{c.Schema, c.Table, c.Constraint}
		byCons[k] = append(byCons[k], c)
	}
	for _, c := range cons {
		t, ok := kept[tableKey{c.Schema, c.Table}]
		if !ok {
			continue
		}
		// A default constraint of SQL Server is the default of its column, which
		// the column already holds.
		if strings.EqualFold(strings.TrimSpace(c.Type), "default") {
			continue
		}
		// MariaDB adds a check of json_valid to a column that is declared json, named
		// for the column, and it reports the column as longtext. The check is no
		// constraint of the table, so the column is the type json again, which
		// brings the check back when the table is written for MariaDB.
		if r.m.Dialect() == dbmeta.MySQL && strings.EqualFold(strings.TrimSpace(c.Type), "check") &&
			strings.HasPrefix(strings.ToLower(text(c.Definition)), "json_valid(") {
			for i := range t.Columns {
				if t.Columns[i].Name == c.Name && t.Columns[i].Type.Kind == String {
					t.Columns[i].Type = Type{Kind: JSON}
				}
			}
			continue
		}
		con := Constraint{
			Name: c.Name, Kind: strings.ToLower(strings.TrimSpace(c.Type)),
			Definition: text(c.Definition), OnUpdate: text(c.OnUpdate), OnDelete: text(c.OnDelete),
			Deferrable: c.Deferrable, Deferred: c.Deferred, Comment: text(c.Comment),
		}
		for _, cc := range byCons[childKey{c.Schema, c.Table, c.Name}] {
			con.Columns = append(con.Columns, cc.Name)
			if cc.ForeignTable.Valid {
				if con.Ref == nil {
					con.Ref = &Reference{Schema: text(cc.ForeignSchema), Table: cc.ForeignTable.V}
				}
				con.Ref.Columns = append(con.Ref.Columns, text(cc.ForeignName))
			}
		}
		t.Constraints = append(t.Constraints, con)
	}
	for _, t := range kept {
		slices.SortFunc(t.Constraints, func(x, y Constraint) int { return cmp.Compare(x.Name, y.Name) })
	}
	return nil
}

// readIndexes puts each index in its table, with its columns in order.
func (r *reader) readIndexes(ctx context.Context, a dbmeta.Args, kept map[tableKey]*Table, views map[tableKey]*View) error {
	idxs, err := collect(dbmeta.Indexes.Each(ctx, r.m, r.db, a))
	if err != nil {
		return fmt.Errorf("reading indexes: %w", err)
	}
	icols, err := collect(dbmeta.IndexColumns.Each(ctx, r.m, r.db, a))
	if err != nil {
		return fmt.Errorf("reading index columns: %w", err)
	}
	slices.SortStableFunc(icols, func(x, y dbmeta.IndexColumn) int { return cmp.Compare(x.Ordinal, y.Ordinal) })
	byIdx := map[childKey][]IndexColumn{}
	for _, c := range icols {
		k := childKey{c.Schema, c.Table, c.Index}
		col := IndexColumn{Descending: c.Descending.Valid && c.Descending.V, Include: c.Include}
		// A plain column has the expression that equals its name. An included column
		// is a plain one, whatever a model puts in its expression.
		if c.Name.Valid && (c.Include || !c.Expression.Valid || c.Expression.V == c.Name.V) {
			col.Name = c.Name.V
		} else {
			col.Expression = text(c.Expression)
			if col.Expression == "" {
				col.Name = text(c.Name)
			}
		}
		byIdx[k] = append(byIdx[k], col)
	}
	for _, i := range idxs {
		t, ok := kept[tableKey{i.Schema, i.Table}]
		if !ok {
			// The indexes of a materialized view are the view's own.
			if v, isView := views[tableKey{i.Schema, i.Table}]; isView && v.Materialized {
				v.Indexes = append(v.Indexes, Index{
					Name: i.Name, Type: i.Type, Unique: i.Unique,
					Columns:   byIdx[childKey{i.Schema, i.Table, i.Name}],
					Predicate: text(i.Predicate), Definition: text(i.Definition), Comment: text(i.Comment),
				})
			}
			continue
		}
		// SingleStore keeps an index of its own for the shard key of a table, which a
		// person did not write and which the product makes again.
		if r.m.Dialect() == dbmeta.MemSQL && i.Name == "__SHARDKEY" {
			continue
		}
		t.Indexes = append(t.Indexes, Index{
			Name: i.Name, Type: i.Type, Unique: i.Unique, Primary: i.Primary,
			Columns:   byIdx[childKey{i.Schema, i.Table, i.Name}],
			Predicate: text(i.Predicate), Definition: text(i.Definition),
			Constraint: text(i.ConstraintType), Comment: text(i.Comment),
		})
	}
	for _, t := range kept {
		slices.SortFunc(t.Indexes, func(x, y Index) int { return cmp.Compare(x.Name, y.Name) })
	}
	for _, v := range views {
		slices.SortFunc(v.Indexes, func(x, y Index) int { return cmp.Compare(x.Name, y.Name) })
	}
	return nil
}

// readViews puts the definition in each view that a table row named.
func (r *reader) readViews(ctx context.Context, a dbmeta.Args, views map[tableKey]*View) error {
	rows, err := collect(dbmeta.Views.Each(ctx, r.m, r.db, a))
	if err != nil {
		return fmt.Errorf("reading views: %w", err)
	}
	if err := r.readViewDependencies(ctx, a, views); err != nil {
		return err
	}
	for _, v := range rows {
		if view, ok := views[tableKey{v.Schema, v.Name}]; ok {
			view.Definition = text(v.Definition)
			view.CheckOption = text(v.CheckOption)
			if view.Comment == "" {
				view.Comment = text(v.Comment)
			}
		}
	}
	return nil
}

// readTriggers puts each trigger in its table.
func (r *reader) readTriggers(ctx context.Context, a dbmeta.Args, kept map[tableKey]*Table) error {
	rows, err := collect(dbmeta.Triggers.Each(ctx, r.m, r.db, a))
	if err != nil {
		return fmt.Errorf("reading triggers: %w", err)
	}
	for _, g := range rows {
		if t, ok := kept[tableKey{g.Schema, g.Table}]; ok {
			t.Triggers = append(t.Triggers, Trigger{
				Name: g.Name, Enabled: g.Enabled, Definition: g.Definition, Comment: text(g.Comment),
			})
		}
	}
	for _, t := range kept {
		slices.SortFunc(t.Triggers, func(x, y Trigger) int { return cmp.Compare(x.Name, y.Name) })
	}
	return nil
}

// readObjects reads the objects that belong to a schema and not to a table:
// sequences, enums and routines.
func (r *reader) readObjects(ctx context.Context, a dbmeta.Args, schemas map[string]*Schema) error {
	seqs, err := collect(dbmeta.Sequences.Each(ctx, r.m, r.db, a))
	if err != nil {
		return fmt.Errorf("reading sequences: %w", err)
	}
	for _, q := range seqs {
		if s, ok := schemas[q.Schema]; ok {
			s.Sequences = append(s.Sequences, Sequence{
				Name: q.Name, DataType: text(q.DataType), Start: text(q.Start), Minimum: text(q.Minimum),
				Maximum: text(q.Maximum), Increment: text(q.Increment),
				Cycles: q.Cycles.Valid && q.Cycles.V, OwnedBy: q.OwnedBy, Identity: q.Identity.Valid && q.Identity.V,
				Comment: text(q.Comment),
			})
		}
	}
	types, err := collect(dbmeta.Types.Each(ctx, r.m, r.db, a))
	if err != nil {
		return fmt.Errorf("reading types: %w", err)
	}
	labels, err := collect(dbmeta.EnumValues.Each(ctx, r.m, r.db, a))
	if err != nil {
		return fmt.Errorf("reading enum values: %w", err)
	}
	slices.SortStableFunc(labels, func(x, y dbmeta.EnumValue) int { return cmp.Compare(x.Ordinal, y.Ordinal) })
	byEnum := map[tableKey][]string{}
	for _, l := range labels {
		k := tableKey{l.Schema, l.Enum}
		byEnum[k] = append(byEnum[k], l.Label)
	}
	doms, err := collect(dbmeta.Domains.Each(ctx, r.m, r.db, a))
	if err != nil {
		return fmt.Errorf("reading domains: %w", err)
	}
	byDomain := map[tableKey]dbmeta.Domain{}
	for _, d := range doms {
		byDomain[tableKey{d.Schema, d.Name}] = d
	}
	byComposite := map[tableKey][]Column{}
	for _, c := range r.columns {
		col, err := r.column(c)
		if err != nil {
			return err
		}
		byComposite[tableKey{c.Schema, c.Table}] = append(byComposite[tableKey{c.Schema, c.Table}], col)
	}
	for _, t := range types {
		s, ok := schemas[t.Schema]
		// The extension that owns a type makes it again.
		if !ok || t.Extension.Valid {
			continue
		}
		switch strings.ToLower(t.Kind) {
		case "enum":
			s.Enums = append(s.Enums, Enum{Name: t.Internal, Values: byEnum[tableKey{t.Schema, t.Internal}]})
		case "domain":
			if d, found := byDomain[tableKey{t.Schema, t.Internal}]; found {
				dom := Domain{Name: d.Name, Type: d.DataType, Collation: d.Collation, NotNull: !d.Nullable,
					Constraints: d.Constraints, Comment: text(d.Comment)}
				if d.Default.Valid {
					v := d.Default.V
					dom.Default = &v
				}
				s.Domains = append(s.Domains, dom)
			}
		case "composite":
			cols := byComposite[tableKey{t.Schema, t.Internal}]
			if cols == nil {
				cols = []Column{}
			}
			s.Composites = append(s.Composites, Composite{Name: t.Internal, Comment: text(t.Comment), Columns: cols})
		}
	}
	fns, err := collect(dbmeta.Functions.Each(ctx, r.m, r.db, a))
	if err != nil {
		return fmt.Errorf("reading routines: %w", err)
	}
	for _, f := range fns {
		if s, ok := schemas[f.Schema]; ok && !f.Extension.Valid {
			def := text(f.Definition)
			if def == "" {
				def = text(f.Source)
			}
			s.Routines = append(s.Routines, Routine{
				Name: f.Name, Kind: f.Kind, Arguments: text(f.ArgTypes),
				ResultType: text(f.ResultType), Language: f.Language, Definition: def, Comment: text(f.Comment),
			})
		}
	}
	for _, s := range schemas {
		slices.SortFunc(s.Sequences, func(x, y Sequence) int { return cmp.Compare(x.Name, y.Name) })
		slices.SortFunc(s.Enums, func(x, y Enum) int { return cmp.Compare(x.Name, y.Name) })
		slices.SortFunc(s.Domains, func(x, y Domain) int { return cmp.Compare(x.Name, y.Name) })
		slices.SortFunc(s.Composites, func(x, y Composite) int { return cmp.Compare(x.Name, y.Name) })
		slices.SortFunc(s.Routines, func(x, y Routine) int {
			return cmp.Or(cmp.Compare(x.Name, y.Name), cmp.Compare(x.Arguments, y.Arguments))
		})
	}
	return nil
}

// readExtensions lists the installed extensions that live in a schema of the
// filter. The language that every database has is left out.
func (r *reader) readExtensions(ctx context.Context, snap *Snapshot) error {
	exts, err := collect(dbmeta.Extensions.Each(ctx, r.m, r.db, dbmeta.Args{}))
	if err != nil {
		return fmt.Errorf("reading extensions: %w", err)
	}
	for _, e := range exts {
		if e.Schema == "pg_catalog" || !r.wantSchema(e.Schema) {
			continue
		}
		snap.Extensions = append(snap.Extensions, Extension{Name: e.Name, Schema: e.Schema, Version: e.Version, Comment: text(e.Comment)})
	}
	slices.SortFunc(snap.Extensions, func(x, y Extension) int { return cmp.Compare(x.Name, y.Name) })
	return nil
}

// matchAny reports whether any pattern matches s.
func matchAny(patterns []string, s string) bool {
	for _, p := range patterns {
		if like(p, s) {
			return true
		}
	}
	return false
}

// likeToken is one element of a pattern of SQL LIKE.
type likeToken struct {
	// any is a %, one is a _, and otherwise the token is the rune r.
	any, one bool
	r        rune
}

// like reports whether s matches a pattern of SQL LIKE. A backslash makes the
// next character plain.
func like(pattern, s string) bool {
	var tokens []likeToken
	runes := []rune(pattern)
	for i := 0; i < len(runes); i++ {
		switch c := runes[i]; {
		case c == '\\' && i+1 < len(runes):
			i++
			tokens = append(tokens, likeToken{r: runes[i]})
		case c == '%':
			tokens = append(tokens, likeToken{any: true})
		case c == '_':
			tokens = append(tokens, likeToken{one: true})
		default:
			tokens = append(tokens, likeToken{r: c})
		}
	}
	text := []rune(s)
	// next[j] is whether the tokens after the current one match text[j:].
	next := make([]bool, len(text)+1)
	next[len(text)] = true
	for _, tok := range slices.Backward(tokens) {
		cur := make([]bool, len(text)+1)
		if tok.any {
			cur[len(text)] = next[len(text)]
		}
		for j := len(text) - 1; j >= 0; j-- {
			if tok.any {
				cur[j] = next[j] || cur[j+1]
			} else {
				cur[j] = (tok.one || tok.r == text[j]) && next[j+1]
			}
		}
		next = cur
	}
	return next[0]
}

// readViewDependencies puts the views that each view reads in DependsOn. The
// snapshot says whether the source answered, so that a missing answer is not
// read as a view that reads nothing.
func (r *reader) readViewDependencies(ctx context.Context, a dbmeta.Args, views map[tableKey]*View) error {
	if dbmeta.ViewDependencies.Support(r.m) != dbmeta.Supported {
		return nil
	}
	rows, err := collect(dbmeta.ViewDependencies.Each(ctx, r.m, r.db, a))
	if err != nil {
		return fmt.Errorf("reading view dependencies: %w", err)
	}
	r.viewDependencies = true
	for _, d := range rows {
		view, ok := views[tableKey{d.Schema, d.View}]
		if !ok {
			continue
		}
		if _, isView := views[tableKey{d.ReferencedSchema, d.Referenced}]; !isView {
			continue
		}
		name := d.ReferencedSchema + "." + d.Referenced
		if !slices.Contains(view.DependsOn, name) {
			view.DependsOn = append(view.DependsOn, name)
		}
	}
	for _, v := range views {
		slices.Sort(v.DependsOn)
	}
	return nil
}
