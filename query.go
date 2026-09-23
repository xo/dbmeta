package dbmeta

import "strings"

// Fragment is one alternative for one piece of a query. It applies when the
// server is at Min or newer.
//
// A Fragment with a zero Min applies to every server.
type Fragment struct {
	// Min is the oldest server version this fragment applies to.
	Min Version
	// SQL is the text of the piece.
	SQL string
}

// Choice holds the alternatives for one piece of a query. Exactly one
// alternative is used: the one with the highest Min that the server meets.
//
// A piece is usually one column of the select list, because that is what
// changes between releases. Keep the piece small. A Choice covering a whole
// statement forces every release to repeat every line of it.
//
// The alternatives of a Choice must all produce the same column, under the
// same name. That rule is what lets one Go type read the result of every
// supported version, and nothing here can check it for you. Where a server is
// too old to have a source for a column, its alternative selects a literal
// NULL under that name. Where a server is too new to have one, the newer
// alternative does the same. Where the type differs between releases, each
// alternative casts to one common type.
//
// The order of the alternatives does not matter.
type Choice []Fragment

// Resolve returns the SQL of the alternative that applies to ver.
//
// If no alternative applies, the server is older than anything this piece
// supports, and Resolve returns [ErrVersionTooOld].
func (c Choice) Resolve(ver Version) (string, error) {
	best, found := -1, false
	for i, f := range c {
		if !ver.AtLeast(f.Min) {
			continue
		}
		if !found || c[best].Min.Before(f.Min) {
			best, found = i, true
		}
	}
	if !found {
		return "", ErrVersionTooOld
	}
	return c[best].SQL, nil
}

// Query is the pieces of one statement, in order.
type Query []Choice

// SQL resolves every piece of q against ver and joins the results with a
// newline.
//
// Each piece carries its own punctuation. A piece that adds a column to a
// select list begins with a comma.
//
// SQL returns [ErrEmptyQuery] when q has no pieces or when every piece
// resolves to nothing. It returns [ErrVersionTooOld] when a piece has no
// alternative for ver.
func (q Query) SQL(ver Version) (string, error) {
	if len(q) == 0 {
		return "", ErrEmptyQuery
	}
	parts := make([]string, 0, len(q))
	for _, c := range q {
		s, err := c.Resolve(ver)
		if err != nil {
			return "", err
		}
		if s = strings.TrimRight(s, " \t"); s != "" {
			parts = append(parts, s)
		}
	}
	if len(parts) == 0 {
		return "", ErrEmptyQuery
	}
	return strings.Join(parts, "\n"), nil
}

// Always returns a Query of one piece that is the same for every version. Use
// it for a statement that no release has changed.
func Always(sql string) Query {
	return Query{Choice{{SQL: sql}}}
}
