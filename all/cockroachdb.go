//go:build (!no_base || cockroachdb) && !no_cockroachdb

package all

import _ "github.com/xo/dbmeta/models/cockroachdb"
