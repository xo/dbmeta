package dbmeta

// Args builds the argument map for a query from the usual filter axes. It is a
// convenience: a caller can pass a plain map instead.
//
// Only the fields that are set are included, so a query that does not take a
// parameter never receives it.
type Args struct {
	// Catalog is the name pattern of the catalog an object belongs to.
	Catalog string
	// Schema is the name pattern of the schema an object belongs to.
	Schema string
	// Parent is the name pattern of the object that contains this one, such as
	// the table a column belongs to.
	Parent string
	// Name is the name pattern the object itself must match.
	Name string
	// Types narrows a table to the kinds named, by Table.Type, such as table
	// and view. Only Tables takes it, and every model's Tables does. It is
	// bound as one string with the items joined by commas. See D138.
	Types []string
	// WithSystem includes the objects the database keeps for itself.
	WithSystem bool
}

// Map returns the arguments as a map, leaving out every field that is unset.
func (a Args) Map() map[string]any {
	m := make(map[string]any, 6)
	for name, v := range map[string]string{
		"catalog": a.Catalog,
		"schema":  a.Schema,
		"parent":  a.Parent,
		"name":    a.Name,
	} {
		if v != "" {
			m[name] = v
		}
	}
	if len(a.Types) != 0 {
		m["types"] = a.Types
	}
	if a.WithSystem {
		m["with_system"] = true
	}
	return m
}

// InList is the SQL condition that item is one of the items of list, where
// list is a string of items joined by commas, which is how a list parameter
// such as types is bound. It joins strings with ||, which most products
// accept. A model whose product joins strings another way writes the same
// condition itself. It does not handle an empty list, which means every item,
// because a product spells the empty string differently: Oracle and Exasol
// read it as NULL. See D138.
func InList(list, item string) string {
	return `(',' || ` + list + ` || ',') LIKE ('%,' || ` + item + ` || ',%')`
}

// TypesParam is the parameter that narrows Tables to the kinds of relation
// named, by Table.Type. Every model's Tables takes it. See D138.
func TypesParam() Param {
	return Param{
		Name:    "types",
		Desc:    "the types to list, as Table.Type spells them, such as table and view, and empty for every type",
		Default: "",
	}
}
