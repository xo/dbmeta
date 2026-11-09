package dump

import (
	"regexp"
	"strings"
)

// customType is a domain or a composite type that a script makes.
type customType struct {
	schema, name string
	// uses is the text of the types that it is built on, which decides what it must
	// follow.
	uses string
	sql  string
}

// customTypes writes the domains and the composite types of the snapshot. A domain
// can be built on another domain or on a composite type, and a composite type can
// have an attribute of either, so each one follows the ones that its text names.
func (g *generator) customTypes() {
	var all []customType
	for _, sc := range g.snap.Schemas {
		if g.style.Domain != nil {
			for _, d := range sc.Domains {
				all = append(all, customType{sc.Name, d.Name, d.Type, g.style.Domain(g.qualified(sc.Name, d.Name), d, g.style.Quote, g.style.Literal)})
			}
		}
		if g.style.Composite != nil {
			for _, c := range sc.Composites {
				var uses []string
				for _, col := range c.Columns {
					uses = append(uses, col.Type.Raw)
				}
				all = append(all, customType{sc.Name, c.Name, strings.Join(uses, " "), g.style.Composite(g.qualified(sc.Name, c.Name), c, g.style.Quote)})
			}
		}
		if g.style.TableType != nil {
			for _, t := range sc.TableTypes {
				var uses []string
				for _, col := range t.Columns {
					uses = append(uses, col.Type.Raw)
				}
				all = append(all, customType{sc.Name, t.Name, strings.Join(uses, " "), g.tableType(sc.Name, t)})
			}
		}
	}
	state := make([]int, len(all))
	var visit func(i int)
	visit = func(i int) {
		if state[i] != 0 {
			return
		}
		state[i] = 1
		for j := range all {
			if j != i && mentions(all[i].uses, all[j]) {
				visit(j)
			}
		}
		g.add(Statement{Stage: Pre, Kind: "type", Name: all[i].schema + "." + all[i].name, SQL: all[i].sql})
	}
	for i := range all {
		visit(i)
	}
}

// mentions reports whether a type text names another type, by its name alone or
// with its schema, with or without quotes.
func mentions(text string, t customType) bool {
	if text == "" {
		return false
	}
	text = strings.ReplaceAll(text, `"`, "")
	re := regexp.MustCompile(`(?:^|[^A-Za-z0-9_])(?:` + regexp.QuoteMeta(t.schema) + `\.)?` + regexp.QuoteMeta(t.name) + `(?:$|[^A-Za-z0-9_])`)
	return re.MatchString(text)
}

// typeComments writes the comments of the extensions, the domains and the
// composite types.
func (g *generator) typeComments() {
	for _, e := range g.snap.Extensions {
		if e.Comment != "" && g.style.Extension != nil {
			g.add(Statement{Stage: Post, Kind: "comment", Name: e.Name,
				SQL: g.style.Comment("EXTENSION", g.style.Quote(e.Name), e.Comment, g.style.Literal)})
		}
	}
	for _, sc := range g.snap.Schemas {
		for _, d := range sc.Domains {
			if d.Comment != "" && g.style.Domain != nil {
				g.add(Statement{Stage: Post, Kind: "comment", Name: sc.Name + "." + d.Name,
					SQL: g.style.Comment("DOMAIN", g.qualified(sc.Name, d.Name), d.Comment, g.style.Literal)})
			}
		}
		for _, c := range sc.Composites {
			if c.Comment != "" && g.style.Composite != nil {
				g.add(Statement{Stage: Post, Kind: "comment", Name: sc.Name + "." + c.Name,
					SQL: g.style.Comment("TYPE", g.qualified(sc.Name, c.Name), c.Comment, g.style.Literal)})
			}
		}
	}
}

// tableType writes a table type from its columns and its keys. The same product
// reads the type of each column as the source wrote it. A key that has a name of
// its own in the source has none here, because the server names it.
func (g *generator) tableType(schema string, t Table) string {
	q, lit := g.style.Quote, g.style.Literal
	var parts []string
	pk := map[string]bool{}
	for _, c := range t.Constraints {
		if c.Kind == "primary key" {
			for _, name := range c.Columns {
				pk[name] = true
			}
		}
	}
	for _, c := range t.Columns {
		if pk[c.Name] {
			c.Nullable = false
		}
		if g.style.Column != nil {
			parts = append(parts, g.style.Column(c, c.Type.Raw, q, lit))
		} else {
			parts = append(parts, genericColumn(c, c.Type.Raw, q))
		}
	}
	for _, c := range t.Constraints {
		c.Name = ""
		switch c.Kind {
		case "primary key", "unique", "check":
			if text, ok := g.constraintSQL(schema, t, c); ok {
				parts = append(parts, text)
			}
		}
	}
	return g.style.TableType(g.qualified(schema, t.Name), parts, t, q)
}
