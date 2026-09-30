//go:build (!no_base || influxdb) && !no_influxdb

package all

import _ "github.com/xo/dbmeta/models/influxdb"
