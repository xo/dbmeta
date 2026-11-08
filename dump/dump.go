// Package dump builds the statements of a dump and of a copy, and runs
// nothing. It takes no connection for anything it builds. A client reads the
// rows, writes the file and runs the statements. See D274 and docs/DUMP.md.
//
// The package uses the standard library and dbmeta, and imports no database
// driver and no Go type of a database library (D275). It names the Go type of
// a kind as a string, in [GoType].
package dump

// Error is an error.
type Error string

// Error satisfies the error interface.
func (err Error) Error() string {
	return string(err)
}

// Error values.
const (
	// ErrNoEquivalent is the no equivalent error. The target has no type that
	// stands for the source type, at the level that the caller allows.
	ErrNoEquivalent Error = "no equivalent type"
	// ErrUnknownType is the unknown type error. The text is not a type that
	// the model of the dialect knows how to read.
	ErrUnknownType Error = "unknown type"
	// ErrNoTypes is the no types error. No model registered the type functions
	// of the dialect, because the model was left out of the binary, or the
	// model has none.
	ErrNoTypes Error = "no type functions"
	// ErrNoStyle is the no style error. No model registered the statement style
	// of the dialect, so the dump cannot write for it.
	ErrNoStyle Error = "no statement style"
	// ErrCycle is the cycle error. The foreign keys of the tables run in a
	// circle, and the product cannot add a foreign key after its table exists.
	ErrCycle Error = "foreign keys form a cycle"
)
