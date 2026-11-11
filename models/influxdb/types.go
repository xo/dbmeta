package influxdb

import (
	"github.com/xo/dbmeta"
	"github.com/xo/dbmeta/dump"
	"github.com/xo/dbmeta/models/internal/typeparse"
)

// The type functions of the dump. InfluxDB is only a source, so no type has an equivalent. See
// D274, D305 and docs/DUMP.md.

func init() {
	dump.RegisterTypes(dbmeta.InfluxDB, dump.Types{Parse: typeparse.Arrow, Render: typeparse.NoRender, Text: "text"})
}
