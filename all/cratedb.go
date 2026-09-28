//go:build (!no_base || cratedb) && !no_cratedb

package all

import _ "github.com/xo/dbmeta/models/cratedb"
