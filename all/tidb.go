//go:build (!no_base || tidb) && !no_tidb

package all

import _ "github.com/xo/dbmeta/models/tidb"
