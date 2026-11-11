package presto

import (
	"github.com/xo/dbmeta"
	"github.com/xo/dbmeta/dump"
	"github.com/xo/dbmeta/models/internal/typeparse"
)

// The type functions of the dump. Presto is only a source, so no type has an equivalent. See
// D274, D305 and docs/DUMP.md.

func init() {
	dump.RegisterTypes(dbmeta.Presto, dump.Types{Parse: typeparse.Presto, Render: typeparse.NoRender, Text: "text"})
}
