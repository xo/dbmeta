//go:build (!no_base || bigquery) && !no_bigquery

package all

import _ "github.com/xo/dbmeta/models/bigquery"
