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
	// Types narrows the kinds of object returned, such as a table against a
	// view.
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
