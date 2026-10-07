//go:build (!no_base || elasticsearch) && !no_elasticsearch

package all

import _ "github.com/xo/dbmeta/models/elasticsearch"
