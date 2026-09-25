// Package test holds the integration tests that need a real database.
//
// It is a separate module with its own go.mod, because it imports database
// drivers and D26 keeps those out of the root module. A consumer of `dbmeta`
// brings its own driver and chooses its own version, and nothing here reaches
// that consumer.
//
// The tests skip unless a server is running, and each dialect reads its
// connection string from its own variable: DBMETA_POSTGRES and DBMETA_MYSQL.
// A third, DBMETA_MYSQL_COMPARE, names a second server of the other product
// and turns on the comparison between MariaDB and MySQL.
//
// The simplest way to run them is dbrun, which starts what they need and
// removes it afterwards:
//
//	go run ./cmd/dbrun test all           every release of every product
//	go run ./cmd/dbrun test tested        the releases CI runs on every push
//	go run ./cmd/dbrun test mariadb-13.0  one release
//	go run ./cmd/dbrun start postgres     the newest PostgreSQL, left running
//	go run ./cmd/dbrun help               everything it does
//
// It takes the list from github.com/xo/dbmeta/container, which is where a
// release is added. Nothing else starts a container, which is D68, and
// docs/RUNNER.md is the design.
//
// `go test ./...` in the root module does not reach here, because the go
// command does not descend into a directory that has its own go.mod.
package test
