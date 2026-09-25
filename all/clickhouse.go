//go:build (!no_base || clickhouse) && !no_clickhouse

package all

import _ "github.com/xo/dbmeta/models/clickhouse"
