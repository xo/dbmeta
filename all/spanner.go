//go:build (!no_base || spanner) && !no_spanner

package all

import _ "github.com/xo/dbmeta/models/spanner"
