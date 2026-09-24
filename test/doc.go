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
// The simplest way to run them is the script beside this file, which starts
// what it needs and removes it afterwards:
//
//	./run.sh                 every release of every product
//	./run.sh tested          the releases CI runs on every push
//	./run.sh mariadb-13.0    one release
//
// It takes the list from github.com/xo/dbmeta/container, which is where a
// release is added. To start a server by hand instead, read the same list:
//
//	go run ./tool/servers postgres-18
//
// `go test ./...` in the root module does not reach here, because the go
// command does not descend into a directory that has its own go.mod.
package test
