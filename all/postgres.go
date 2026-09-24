//go:build (!no_base || postgres) && !no_postgres

package all

import _ "github.com/xo/dbmeta/models/postgres"
