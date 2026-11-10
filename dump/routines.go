package dump

import "strings"

// identifiers returns the names in a text, in lower case: the runs of letters,
// digits and underscores. A routine and a view find the objects that they read
// by these names, and a name that two schemas share finds both, which only makes
// the order stricter.
func identifiers(text string) map[string]bool {
	words := map[string]bool{}
	start := -1
	text = strings.ToLower(text)
	for i := 0; i <= len(text); i++ {
		var word bool
		if i < len(text) {
			c := text[i]
			word = c == '_' || c == '$' || c == '#' || c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 0x80
		}
		switch {
		case word && start < 0:
			start = i
		case !word && start >= 0:
			words[text[start:i]] = true
			start = -1
		}
	}
	return words
}

// earlyRoutineSet names the routines that a script writes before the tables, as
// schema.name. A product that reads the tables that a routine names needs the
// routine after them, but a column, a default or a check that calls a routine
// needs it first, and so does a column of a type that a person made, and a routine
// that one of these calls.
func (g *generator) earlyRoutineSet() map[string]bool {
	early := map[string]bool{}
	if !g.style.RoutinesAfterTables {
		return early
	}
	used := map[string]bool{}
	for _, sc := range g.snap.Schemas {
		for _, t := range sc.Tables {
			for _, c := range t.Columns {
				// A column of a type that a person made needs the type first.
				for w := range identifiers(c.Type.Raw) {
					used[w] = true
				}
				if c.Default != nil {
					for w := range identifiers(*c.Default) {
						used[w] = true
					}
				}
			}
			for _, c := range t.Constraints {
				for w := range identifiers(c.Definition) {
					used[w] = true
				}
			}
		}
	}
	for changed := true; changed; {
		changed = false
		for _, sc := range g.snap.Schemas {
			for _, r := range sc.Routines {
				key := sc.Name + "." + r.Name
				if early[key] || !used[strings.ToLower(r.Name)] {
					continue
				}
				early[key] = true
				changed = true
				for w := range identifiers(r.Definition) {
					used[w] = true
				}
			}
		}
	}
	return early
}

// earlyRoutines writes the routines that come before the tables. They are all of
// them for a product that has no need to wait, and in the order of their names.
func (g *generator) earlyRoutines() {
	early := g.earlyRoutineSet()
	type entry struct {
		schema string
		r      Routine
		words  map[string]bool
	}
	var all []entry
	byName := map[string][]int{}
	for _, sc := range g.snap.Schemas {
		for _, r := range sc.Routines {
			if g.rangeWritten[sc.Name+"."+r.Name] {
				continue
			}
			if g.style.RoutinesAfterTables && !early[sc.Name+"."+r.Name] {
				continue
			}
			byName[strings.ToLower(r.Name)] = append(byName[strings.ToLower(r.Name)], len(all))
			all = append(all, entry{sc.Name, r, nil})
		}
	}
	if g.style.RoutinesAfterTables {
		for i := range all {
			all[i].words = identifiers(all[i].r.Definition)
		}
	}
	state := make([]int, len(all))
	var visit func(i int)
	visit = func(i int) {
		if state[i] != 0 {
			return
		}
		state[i] = 1
		for w := range all[i].words {
			for _, j := range byName[w] {
				if j != i && !sameObject(all[i].schema, all[i].r.Name, all[j].schema, all[j].r.Name) {
					visit(j)
				}
			}
		}
		g.routine(Pre, all[i].schema, all[i].r)
	}
	for i := range all {
		visit(i)
	}
}

// rangeFunctions writes the functions that a range type names as its canonical function
// or its difference function, before the type. The canonical function returns the range
// type, so the type is first made as a shell, which the full statement fills in. See D297.
func (g *generator) rangeFunctions() {
	if g.style.Range == nil || g.cross {
		return
	}
	for _, sc := range g.snap.Schemas {
		for _, rt := range sc.Ranges {
			var funcs []string
			for _, ref := range []string{rt.Canonical, rt.SubtypeDiff} {
				if name, ok := g.userRoutine(sc.Name, ref); ok {
					funcs = append(funcs, name)
				}
			}
			if len(funcs) == 0 {
				continue
			}
			if _, ok := g.userRoutine(sc.Name, rt.Canonical); ok {
				g.add(Statement{Stage: Pre, Kind: "type", Name: sc.Name + "." + rt.Name, SQL: "CREATE TYPE " + g.qualified(sc.Name, rt.Name)})
			}
			for _, key := range funcs {
				schema, name, _ := strings.Cut(key, ".")
				for _, s2 := range g.snap.Schemas {
					if s2.Name != schema {
						continue
					}
					for _, r := range s2.Routines {
						if r.Name == name {
							g.routine(Pre, s2.Name, r)
						}
					}
				}
				if g.rangeWritten == nil {
					g.rangeWritten = map[string]bool{}
				}
				g.rangeWritten[key] = true
			}
		}
	}
}

// userRoutine finds the routine that a range type names, as schema.name, when the
// snapshot holds it. A name with no schema is in the schema of the type.
func (g *generator) userRoutine(schema, ref string) (string, bool) {
	ref = strings.TrimSpace(strings.ReplaceAll(ref, `"`, ""))
	if ref == "" {
		return "", false
	}
	if i := strings.IndexByte(ref, '('); i >= 0 {
		ref = ref[:i]
	}
	owner, name := schema, ref
	if o, n, ok := strings.Cut(ref, "."); ok {
		owner, name = o, n
	}
	for _, sc := range g.snap.Schemas {
		if sc.Name != owner {
			continue
		}
		for _, r := range sc.Routines {
			if r.Name == name {
				return owner + "." + name, true
			}
		}
	}
	return "", false
}
