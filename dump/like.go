package dump

import (
	"fmt"

	"github.com/xo/dbmeta"
)

// RegisterLike registers a dialect that speaks the SQL of another one, such as
// CockroachDB and PostgreSQL or TiDB and MySQL. The type functions and the style
// of like are copied, and adjust changes the copy. The model of d calls it from
// init after the model of like has registered, which its import guarantees.
// Either argument of adjust can be nil.
//
// A product with the same SQL but its own type names gives its own Parse, so that
// the text of its catalog reads as the kind it is. See D280.
func RegisterLike(d, like dbmeta.Dialect, adjustTypes func(*Types), adjustStyle func(*Style)) {
	t, err := typesOf(like)
	if err != nil {
		panic(fmt.Sprintf("dump: %s is registered like %s, which has no types: %v", d, like, err))
	}
	s, err := styleOf(like)
	if err != nil {
		panic(fmt.Sprintf("dump: %s is registered like %s, which has no style: %v", d, like, err))
	}
	if adjustTypes != nil {
		adjustTypes(&t)
	}
	if adjustStyle != nil {
		adjustStyle(&s)
	}
	RegisterTypes(d, t)
	RegisterStyle(d, s)
}
