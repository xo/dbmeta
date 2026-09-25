package container

import (
	"fmt"
	"net/url"

	"github.com/xo/dbmeta"
)

// PostgreSQL is the primary model. D9 makes it the shape every other
// database's answers follow, and D20 takes it back to 9.6.

// postgres is the official image, which is the only one anybody runs.
var postgres = product{
	dialect: dbmeta.PostgreSQL,
	name:    "postgres",
	image:   "docker.io/library/postgres",
	port:    5432,
	env:     map[string]string{"POSTGRES_PASSWORD": Password},
	// -h forces TCP. On the local socket pg_isready reports ready during
	// the bootstrap phase, before the server restarts to accept network
	// connections, and a test that connects then is refused.
	ready: []string{"pg_isready", "-U", "postgres", "-h", "127.0.0.1"},
	dsn: func(port int) string {
		return fmt.Sprintf("postgres://postgres:%s@127.0.0.1:%d/postgres?sslmode=disable",
			url.QueryEscape(Password), port)
	},
}

// PostgreSQL is every PostgreSQL release dbmeta supports.
//
// All ten majors from 9.6, which is further back than psql itself goes: psql
// dropped 9.6 in release 20, so the queries for it are translated from an
// older checkout. See D20 and hard rule 5.
var PostgreSQL = list{}.add(postgres, Tested, "9.6", "12", "15", "18").
	add(postgres, Nightly, "10", "11", "13", "14", "16", "17")
