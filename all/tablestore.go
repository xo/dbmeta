//go:build (!no_base || tablestore) && !no_tablestore

package all

import _ "github.com/xo/dbmeta/models/tablestore"
