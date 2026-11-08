package dump

import (
	"fmt"
	"sync"

	"github.com/xo/dbmeta"
)

// Types are the type functions of one dialect. A model registers them from
// init, beside its queries, because the knowledge of a product lives with its
// model. See D274.
type Types struct {
	// Parse reads the text of a type as the catalog of the product gives it,
	// such as character varying(10) or decimal(10,2) unsigned.
	Parse func(text string) (Type, error)
	// Render writes a type in this dialect. It returns [ErrNoEquivalent] when
	// the dialect has no type of the kind at the Exact or the Lossy level.
	Render func(t Type) (Rendering, error)
	// Text is the string type with no limit, such as text or longtext, which
	// stands for any type that Render refuses when the client allows the Text
	// level.
	Text string
}

var (
	typesMu sync.RWMutex
	types   = map[dbmeta.Dialect]Types{}
)

// RegisterTypes registers the type functions of a dialect. Registering twice
// panics, as [dbmeta.RegisterDialect] does.
func RegisterTypes(d dbmeta.Dialect, t Types) {
	typesMu.Lock()
	defer typesMu.Unlock()
	if _, ok := types[d]; ok {
		panic("dump: types registered twice: " + string(d))
	}
	types[d] = t
}

func typesOf(d dbmeta.Dialect) (Types, error) {
	typesMu.RLock()
	defer typesMu.RUnlock()
	t, ok := types[d]
	if !ok {
		return Types{}, fmt.Errorf("reading types of %s: %w", d, ErrNoTypes)
	}
	return t, nil
}

// ParseType reads the text of a type from the catalog of d. The text is kept in
// [Type.Raw].
func ParseType(d dbmeta.Dialect, text string) (Type, error) {
	t, err := typesOf(d)
	if err != nil {
		return Type{}, err
	}
	v, err := t.Parse(text)
	if err != nil {
		return Type{}, fmt.Errorf("parsing %q for %s: %w", text, d, err)
	}
	v.Raw = text
	return v, nil
}

// RenderType writes a type in the dialect d, without the rule for the same
// dialect. [TypeFor] has that rule.
func RenderType(d dbmeta.Dialect, t Type) (Rendering, error) {
	f, err := typesOf(d)
	if err != nil {
		return Rendering{}, err
	}
	r, err := f.Render(t)
	if err != nil {
		return Rendering{}, fmt.Errorf("rendering %s for %s: %w", describe(t), d, err)
	}
	return r, nil
}

// TextFor returns the string type of d, at the Text level, for a type that d has
// no type for. A client allows this level on its own, and dbmeta never picks it.
func TextFor(d dbmeta.Dialect, t Type) (Rendering, error) {
	f, err := typesOf(d)
	if err != nil {
		return Rendering{}, err
	}
	if f.Text == "" {
		return Rendering{}, fmt.Errorf("writing %s as text for %s: %w", describe(t), d, ErrNoEquivalent)
	}
	return Rendering{
		SQL:   f.Text,
		Level: Text,
		Note:  fmt.Sprintf("%s has no type for %s, so it is written as %s", d, describe(t), f.Text),
	}, nil
}

// TypeFor returns the type of a column in the target dialect. A column that
// comes from the same dialect is written back as it was read, which is Exact by
// definition, and the version of the server is not checked. Any other column
// goes through the neutral kind.
func TypeFor(source, target dbmeta.Dialect, t Type) (Rendering, error) {
	if source == target && t.Raw != "" {
		return Rendering{SQL: t.Raw, Level: Exact}, nil
	}
	return RenderType(target, t)
}

// describe names a type in an error, from the text that was read when there is
// one.
func describe(t Type) string {
	if t.Raw != "" {
		return t.Raw
	}
	return string(t.Kind)
}
