package dbmeta

import "strings"

// Syntax is the lexical forms a product's SQL has. A client that splits text
// into statements reads it to know which characters open a string, a
// comment or a quoted name, so that a semicolon inside one does not end the
// statement. usql's lexer took these per driver, and they are the product's
// grammar, the same for every driver of it. See D143.
type Syntax struct {
	// DollarQuotes is a string between $$ and $$, or between $tag$ and $tag$.
	DollarQuotes bool
	// BlockComments is a comment between /* and */.
	BlockComments bool
	// SlashComments is a comment from // to the end of the line.
	SlashComments bool
	// HashComments is a comment from # to the end of the line.
	HashComments bool
	// Backticks is a name between backticks.
	Backticks bool
}

// Terminator is what a product does with a semicolon that ends a statement.
type Terminator int

// Terminator values.
const (
	// TerminatorKept is a product that takes the semicolon, which is most.
	TerminatorKept Terminator = iota
	// TerminatorStripped is a product that refuses a semicolon at the end of
	// a statement, such as Trino and Presto, so a client removes it.
	TerminatorStripped
	// TerminatorStrippedUnlessEnd is Oracle, which refuses the semicolon at
	// the end of a statement and needs it at the end of a PL/SQL block, so a
	// client removes it unless the statement ends with END;.
	TerminatorStrippedUnlessEnd
)

// Batch is a statement that opens a batch and the one that closes it, such
// as CQL's BEGIN BATCH and APPLY BATCH.
type Batch struct {
	Begin string
	End   string
}

// Fold is what a product does to the case of a name that is not quoted.
type Fold int

// Fold values.
const (
	// FoldNone keeps a name as it is written, and the product either
	// compares names with their case or compares them without it. MySQL,
	// SQL Server and SQLite are the cases.
	FoldNone Fold = iota
	// FoldLower stores a name in lower case, as PostgreSQL does.
	FoldLower
	// FoldUpper stores a name in upper case, as Oracle does.
	FoldUpper
)

// Syntax returns the lexical forms of the dialect's SQL, and false when no
// model is registered for it.
func (d Dialect) Syntax() (Syntax, bool) {
	info, ok := d.Info()
	if !ok {
		return Syntax{}, false
	}
	return info.Syntax, true
}

// FoldIdentifier returns name as the product stores it, so that a caller can
// match it against the catalog. A name between double quotes, or between
// backticks where the product quotes with them, is returned without the
// quotes and as it is written, with a doubled quote made single. Any other
// name is folded as the product folds it. A dialect that no model is
// registered for returns name as it is.
//
// psql folds a pattern the same way before it matches the catalog, which is
// why \d book finds BOOK on Oracle. See D143.
func (d Dialect) FoldIdentifier(name string) string {
	info, ok := d.Info()
	if !ok {
		return name
	}
	if n := len(name); n >= 2 {
		for _, q := range []byte{'"', '`'} {
			if q == '`' && !info.Syntax.Backticks {
				continue
			}
			if name[0] == q && name[n-1] == q {
				return strings.ReplaceAll(name[1:n-1], string([]byte{q, q}), string(q))
			}
		}
	}
	switch info.Fold {
	case FoldLower:
		return strings.ToLower(name)
	case FoldUpper:
		return strings.ToUpper(name)
	}
	return name
}
