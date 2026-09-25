//go:build (!no_base || cassandra) && !no_cassandra

package all

import _ "github.com/xo/dbmeta/models/cassandra"
