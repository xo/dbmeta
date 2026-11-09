package dump

import "strings"

// versioned says that a table of the source that keeps its history is written as
// one that keeps its history. Another product has no such table, and a product
// that has one takes it only from the same product.
func (g *generator) versioned() bool {
	return !g.cross && g.style.SystemTime != nil && g.style.VersioningOff != nil && g.style.VersioningOn != nil
}

// versioningNote says what a system versioned table loses.
func (g *generator) versioningNote(t Table) string {
	if t.Versioning != nil && !g.versioned() {
		return "the table keeps its history in the source, and it is written as a plain table. " +
			"The history table is another plain table"
	}
	return ""
}

// versionedTables lists the system versioned tables of the snapshot, in order of
// schema and name.
func (g *generator) versionedTables() []schemaTable {
	var out []schemaTable
	if !g.versioned() {
		return out
	}
	for _, sc := range g.snap.Schemas {
		for _, t := range sc.Tables {
			if t.Versioning != nil {
				out = append(out, schemaTable{sc.Name, t})
			}
		}
	}
	return out
}

// versioningOff frees the period columns of each system versioned table, so that
// the rows that a load gives keep the instants that the source held. The server
// refuses a value for a period column while versioning is on or the period exists.
func (g *generator) versioningOff() {
	for _, t := range g.versionedTables() {
		g.add(Statement{Stage: Data, Kind: "setting", Name: t.schema + "." + t.table.Name,
			SQL:  g.style.VersioningOff(g.qualified(t.schema, t.table.Name)),
			Note: "the period of the table is dropped while the rows load, and it is made again after them"})
	}
}

// versioningOn makes the period again and turns versioning on, after the rows of
// the table and of its history table are loaded.
func (g *generator) versioningOn() {
	for _, t := range g.versionedTables() {
		v := *t.table.Versioning
		for _, sql := range g.style.VersioningOn(g.qualified(t.schema, t.table.Name), g.qualified(v.Schema, v.Table), v, g.style.Quote) {
			g.add(Statement{Stage: Post, Kind: "versioning", Name: t.schema + "." + t.table.Name, SQL: sql})
		}
	}
}

// computedColumn says that the product computes the value of a column, so that a
// load must leave it out. A period column of a system versioned table is
// generated too, but a load gives it the instant that the source held.
func computedColumn(c Column) bool {
	switch strings.ToLower(c.Generated) {
	case "", "row start", "row end":
		return false
	}
	return true
}
