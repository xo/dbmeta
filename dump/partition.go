package dump

import (
	"slices"
	"strings"
)

// partitioned says that a table that is split into partitions is written with its
// partitions. A script for another product writes the table whole, because the
// rows are the same and the partitions are a choice about storage.
func (g *generator) partitioned() bool {
	return !g.cross && g.style.PartitionBy != nil
}

// materialized says that a materialized view is written as one.
func (g *generator) materialized() bool {
	return !g.cross && g.style.MaterializedView != nil
}

// partitionNote says what a table that is split into partitions loses.
func (g *generator) partitionNote(t Table) string {
	if t.Partitioning != nil && !g.partitioned() {
		return "the table is split into partitions in the source, and it is written as one table"
	}
	return ""
}

// joinNotes joins the notes that are not empty.
func joinNotes(notes ...string) string {
	var out []string
	for _, n := range notes {
		if n != "" {
			out = append(out, n)
		}
	}
	return strings.Join(out, ". ")
}

// partitions lists the tables that are partitions of another table, each after
// the table it belongs to, so that the parent exists when the partition is made.
func (g *generator) partitions() []schemaTable {
	var all []schemaTable
	made := map[string]bool{}
	for _, sc := range g.snap.Schemas {
		for _, t := range sc.Tables {
			if t.IsPartition() {
				all = append(all, schemaTable{sc.Name, t})
			} else {
				made[sc.Name+"."+t.Name] = true
			}
		}
	}
	var out []schemaTable
	for len(all) > 0 {
		var later []schemaTable
		for _, t := range all {
			if made[t.table.PartitionOf.Schema+"."+t.table.PartitionOf.Table] {
				made[t.schema+"."+t.table.Name] = true
				out = append(out, t)
			} else {
				later = append(later, t)
			}
		}
		// A parent that is not in the snapshot never appears, so the rest go in order.
		if len(later) == len(all) {
			return append(out, later...)
		}
		all = later
	}
	return out
}

// createPartition writes the CREATE TABLE of a partition.
func (g *generator) createPartition(schema string, t Table) string {
	p := t.PartitionOf
	sql := g.style.PartitionOf(g.qualified(schema, t.Name), g.qualified(p.Schema, p.Table), p.Bound)
	if t.Partitioning != nil {
		sql += g.style.PartitionBy(*t.Partitioning, g.style.Quote, g.style.Literal)
	}
	return sql
}

// withPartitions returns the snapshot that a script is made from. A partition
// gets its columns, its keys, its indexes and its triggers from its parent, so
// what the parent also has is taken off it, and the script does not make a key
// twice. A script that writes the tables whole has no partitions at all, because
// the rows of the parent are all the rows.
func withPartitions(s *Snapshot, whole bool) *Snapshot {
	found := false
	parents := map[string]Table{}
	for _, sc := range s.Schemas {
		for _, t := range sc.Tables {
			found = found || t.PartitionOf != nil
			parents[sc.Name+"."+t.Name] = t
		}
	}
	if !found {
		return s
	}
	out := *s
	out.Schemas = make([]Schema, len(s.Schemas))
	for i, sc := range s.Schemas {
		sc.Tables = make([]Table, 0, len(s.Schemas[i].Tables))
		for _, t := range s.Schemas[i].Tables {
			switch parent, ok := parents[partitionParent(t)]; {
			case t.PartitionOf == nil:
			case whole:
				continue
			case ok:
				t = inheritFrom(t, parent)
			}
			sc.Tables = append(sc.Tables, t)
		}
		out.Schemas[i] = sc
	}
	return &out
}

// partitionParent names the parent of a partition, and is empty for any other
// table.
func partitionParent(t Table) string {
	if t.PartitionOf == nil {
		return ""
	}
	return t.PartitionOf.Schema + "." + t.PartitionOf.Table
}

// inheritFrom returns a partition without the constraints, the indexes and the
// triggers that its parent already makes on it.
func inheritFrom(t, parent Table) Table {
	constraintKey := func(c Constraint) string {
		ref := ""
		if c.Ref != nil {
			ref = c.Ref.Schema + "." + c.Ref.Table + "(" + strings.Join(c.Ref.Columns, ",") + ")"
		}
		return c.Kind + "|" + strings.Join(c.Columns, ",") + "|" + c.Definition + "|" + ref
	}
	// An index is the same when everything from USING on is, because the name and
	// the table in front of it are the ones of each table.
	indexKey := func(ix Index) string {
		_, rest, _ := strings.Cut(ix.Definition, " USING ")
		return rest
	}
	inherited := map[string]bool{}
	for _, c := range parent.Constraints {
		inherited[constraintKey(c)] = true
	}
	t.Constraints = slices.DeleteFunc(slices.Clone(t.Constraints), func(c Constraint) bool { return inherited[constraintKey(c)] })
	indexes := map[string]bool{}
	for _, ix := range parent.Indexes {
		if k := indexKey(ix); k != "" {
			indexes[k] = true
		}
	}
	t.Indexes = slices.DeleteFunc(slices.Clone(t.Indexes), func(ix Index) bool { return indexes[indexKey(ix)] })
	triggers := map[string]bool{}
	for _, tg := range parent.Triggers {
		triggers[tg.Name] = true
	}
	t.Triggers = slices.DeleteFunc(slices.Clone(t.Triggers), func(tg Trigger) bool { return triggers[tg.Name] })
	return t
}
