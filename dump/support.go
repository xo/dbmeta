package dump

import (
	"fmt"

	"github.com/xo/dbmeta"
)

// ErrNoCatalog is the no catalog error. The model of the dialect answers no
// question about tables or columns, so there is nothing to read a schema from.
const ErrNoCatalog Error = "no reader of tables and columns"

// Capability says what a dump can do with a dialect, and why when it cannot, so
// that a client refuses with a clear reason. See D274.
type Capability struct {
	// Source is true when the schema of a database of the dialect can be read into
	// a snapshot: the model answers the tables and the columns, and the types of
	// the dialect can be read.
	Source bool
	// SourceWhy says why not, and is nil when Source is true. It wraps
	// [dbmeta.ErrModelNotBuilt], [ErrNoCatalog] or [ErrNoTypes].
	SourceWhy error

	// Target is true when a script can be written for the dialect: it has type
	// functions, a statement style and a binary literal.
	Target bool
	// TargetWhy says why not, and is nil when Target is true. It wraps
	// [dbmeta.ErrModelNotBuilt], [ErrNoTypes] or [ErrNoStyle].
	TargetWhy error
}

// Support reports what a dump can do with a dialect that is built into this
// binary. It reads the models and asks the server nothing.
func Support(d dbmeta.Dialect) Capability {
	var c Capability
	if _, built := d.Info(); !built {
		why := fmt.Errorf("the model of %s: %w", d, dbmeta.ErrModelNotBuilt)
		return Capability{SourceWhy: why, TargetWhy: why}
	}
	m, err := dbmeta.New(d, dbmeta.VersionSet{})
	switch {
	case err != nil:
		c.SourceWhy = fmt.Errorf("the model of %s: %w", d, err)
	case dbmeta.Tables.Support(m) != dbmeta.Supported || dbmeta.Columns.Support(m) != dbmeta.Supported:
		c.SourceWhy = fmt.Errorf("%s: %w", d, ErrNoCatalog)
	default:
		if _, terr := typesOf(d); terr != nil {
			c.SourceWhy = terr
		} else {
			c.Source = true
		}
	}
	style, serr := styleOf(d)
	_, terr := typesOf(d)
	switch {
	case terr != nil:
		c.TargetWhy = terr
	case serr != nil:
		c.TargetWhy = serr
	case style.Binary == nil:
		c.TargetWhy = fmt.Errorf("%s has no binary literal: %w", d, ErrNoStyle)
	default:
		c.Target = true
	}
	return c
}
