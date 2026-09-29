//go:build (!no_base || redshift) && !no_redshift

package all

import _ "github.com/xo/dbmeta/models/redshift"
