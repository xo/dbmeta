package container

import (
	"fmt"
	"net/url"
	"time"

	"github.com/xo/dbmeta"
)

// The Vertica releases dbmeta is tested against.
//
// # The image, and why this one
//
// D66 recorded Vertica as unable to start. vertica/vertica-ce was withdrawn,
// opentext/vertica-k8s runs only under the Kubernetes operator, and the
// Community Edition download that one-node-ce builds from went away when
// Rocket Software took Vertica over. D88 is the decision that changes that.
//
// docker.io/ratiopbc/vertica-ce is a copy of the image Vertica's own
// vertica-containers/one-node-ce Dockerfile builds: its entrypoint carries the
// Open Text copyright and Apache licence, its layers are that Dockerfile's
// steps, and its binary reports Vertica Analytic Database v25.1.0-0. It was
// built on 2024-12-17 and pushed on 2025-11-17, and it has not been rebuilt,
// so it is pinned by digest as well as by tag: a push to the same tag would
// otherwise change what was tested without anything here changing.
//
// # The range, and where it comes from
//
// Step 2 of docs/EVALUATION.md gives a floor of one for a maintained image,
// because nothing current but 25.1.0-0 can be started outside Kubernetes.
// Ken chose to go further back with three community images of older
// Community Edition releases, each pushed once by the person who built it,
// so that a version gate in the model has servers older than 25.1 to answer
// against. D88 records that, and why D66's objection to exactly this kind of
// image no longer decides it.
//
//	7.2.1   docker.io/colemantw/vertica, built 2016-01-21
//	9.1.0   docker.io/iamamr/vertica, built 2018-08-10
//	10.1.1  docker.io/saadmairaj/vertica, built 2021-05-22
//	25.1.0  docker.io/ratiopbc/vertica-ce, built 2024-12-17
//
// Every one is pinned by digest as well as by tag, for the reason above.
var vertica = product{
	dialect: dbmeta.Vertica,
	name:    "vertica",
	image:   "docker.io/ratiopbc/vertica-ce",
	port:    5433,
	env: map[string]string{
		// The entrypoint creates this user with PSEUDOSUPERUSER and sets its
		// password, which is how a password reaches the image. dbadmin, the
		// superuser the database is created with, has none and cannot be
		// given one from outside.
		"APP_DB_USER":     "dbmeta",
		"APP_DB_PASSWORD": Password,
	},
	// Over TCP as the user the DSN names, with its password. dbadmin can
	// connect over the local socket as soon as the database exists, which
	// is before the entrypoint has loaded VMart and created this user, and
	// a check that passed then reported the server up while the first
	// connection was refused. This user is the last thing the entrypoint
	// makes, so answering as it is answering for everything.
	ready: []string{
		"/opt/vertica/bin/vsql", "-h", "127.0.0.1", "-U", "dbmeta", "-w", Password,
		"-c", "SELECT 1",
	},
	// Creating the database and loading VMart is most of it.
	startup: 5 * time.Minute,
	dsn: func(port int) string {
		return fmt.Sprintf("vertica://dbmeta:%s@127.0.0.1:%d/VMart",
			url.QueryEscape(Password), port)
	},
}

// verticaLegacy is the older images, which share one entrypoint, from
// jbfavre/docker-vertica.
//
// It creates a database called docker with a dbadmin that has no password,
// and 7.2.1 and 9.1.0 take nothing from the environment at all. So a user
// with the project password is made once the server answers, as Init, the
// same user the 25.1 image makes for itself. Every Vertica here is then
// reached as dbmeta with [Password], whatever release it is.
var verticaLegacy = product{
	dialect: dbmeta.Vertica,
	name:    "vertica",
	port:    5433,
	// dbadmin over the local socket, which needs no password on these
	// images. The database is built into the image, so this answers within
	// seconds of the start.
	ready: []string{"/opt/vertica/bin/vsql", "-U", "dbadmin", "-d", "docker", "-c", "SELECT 1"},
	// Safe to run twice, because start runs it every time: it creates the
	// user only when there is none.
	init: []string{"sh", "-c", `v="/opt/vertica/bin/vsql -U dbadmin -d docker -At"` +
		` && [ "$($v -c "SELECT COUNT(*) FROM users WHERE user_name = 'dbmeta'")" = 1 ]` +
		` || $v -c "CREATE USER dbmeta IDENTIFIED BY '` + Password + `';` +
		` GRANT PSEUDOSUPERUSER TO dbmeta; ALTER USER dbmeta DEFAULT ROLE ALL;"`},
	startup: 5 * time.Minute,
	dsn: func(port int) string {
		return fmt.Sprintf("vertica://dbmeta:%s@127.0.0.1:%d/docker",
			url.QueryEscape(Password), port)
	},
}

// Vertica is every Vertica release dbmeta is tested against.
//
// 25.1 is the one current release and runs on every push. The three older
// ones run nightly: they are the gates' floor rather than what a consumer
// connects to today, and they are large.
var Vertica = list{}.add(vertica, Tested, "25.1").
	add(verticaLegacy, Nightly, "7.2", "9.1", "10.1").
	// The tag is the release with a v and a hotfix number, and the digest is
	// the one image ever pushed under it.
	on("25.1", func(s *Server) {
		s.Tag = "v25.1.0-0@sha256:0753e11d9413c1e8ed4394ac5b820f919e40ed1ca70220e7dad2e3bd92d446a8"
	}).
	on("7.2", func(s *Server) {
		s.Image = "docker.io/colemantw/vertica"
		s.Tag = "latest@sha256:9b4f896536b49433f8fb10417f04be04dc140109496740dda00b0bbc2c97bdd7"
	}).
	on("9.1", func(s *Server) {
		s.Image = "docker.io/iamamr/vertica"
		s.Tag = "9.1.0-0@sha256:bfa9ff9c947d1f53f28839aed3fa30da9247335ce570f6398d1994c9a9143f72"
	}).
	on("10.1", func(s *Server) {
		s.Image = "docker.io/saadmairaj/vertica"
		s.Tag = "10.1.1-RHEL6@sha256:2d638b1e38b7139ab4ac64bb7ecd9748ae7c264b2cb1c173b50d4a578e54f182"
	})
