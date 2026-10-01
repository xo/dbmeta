package container

import (
	"fmt"
	"maps"
	"net/url"
	"time"

	"github.com/xo/dbmeta"
)

// The SingleStore releases dbmeta is tested against.
//
// SingleStore speaks MySQL's protocol, and dburl names its scheme memsql, the
// name SingleStore had until 2020. The dialect is memsql (D125).
//
// # The image
//
// ghcr.io/singlestore-labs/singlestoredb-dev runs a master aggregator and a
// leaf in one container, and needs no license on a machine with at most 8
// cores and 64 GB of memory, as its README says. The image is tagged by its
// own version and not by the engine's, so each release pins the image tag,
// and SINGLESTORE_VERSION names an engine other than the one the image
// ships, which the container downloads when it starts. Ken asked on
// 2026-09-30 for SingleStore to be tried without a license key, which
// amends D118 (D141).
//
// # The port
//
// SingleStore serves MySQL's protocol on 3306. The image also serves Studio
// on 8080 and the Data API on 9000, which the entry does not publish.

// singlestoreImage is the image tag every release pins.
const singlestoreImage = "0.2.85"

// singlestore is the SingleStore development image.
var singlestore = product{
	dialect: dbmeta.MemSQL,
	name:    "singlestore",
	image:   "ghcr.io/singlestore-labs/singlestoredb-dev",
	port:    3306,
	env:     map[string]string{"ROOT_PASSWORD": Password},
	ready:   []string{"sh", "-c", "singlestore -h127.0.0.1 -uroot -p'" + Password + "' -e 'SELECT 1'"},
	// 9.0 downloads its engine when it starts, which the README says takes
	// about a minute.
	startup: 5 * time.Minute,
	dsn: func(port int) string {
		return fmt.Sprintf("root:%s@tcp(127.0.0.1:%d)/?parseTime=true", Password, port)
	},
	url: func(port int) string {
		return fmt.Sprintf("memsql://root:%s@127.0.0.1:%d/", url.QueryEscape(Password), port)
	},
}

// SingleStore is every SingleStore release dbmeta is tested against. Both
// are Tested, which is the cadence they kept while they were Staged (D120).
//
// Image 0.2.85 ships 9.1.1, measured on 2026-09-30, and 9.0 is the line
// before it, which the container downloads.
var SingleStore = list{}.add(singlestore, Tested, "9.0", "9.1").
	on("9.0", func(s *Server) {
		s.Tag = singlestoreImage
		s.Env = maps.Clone(s.Env)
		s.Env["SINGLESTORE_VERSION"] = "9.0"
	}).
	on("9.1", func(s *Server) { s.Tag = singlestoreImage })
