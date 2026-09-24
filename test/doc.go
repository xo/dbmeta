// Package test holds the integration tests that need a real database.
//
// It is a separate module with its own go.mod, because it imports database
// drivers and D26 keeps those out of the root module. A consumer of `dbmeta`
// brings its own driver and chooses its own version, and nothing here reaches
// that consumer.
//
// The tests skip unless a server is running. Start one with podman and point
// the test at it:
//
//	podman run -d --rm --name dbmeta-pg18 -e POSTGRES_PASSWORD=P4ssw0rd \
//	    -p 55432:5432 docker.io/library/postgres:18
//	DBMETA_POSTGRES=postgres://postgres:P4ssw0rd@localhost:55432/postgres?sslmode=disable \
//	    go test ./...
//
// Run every supported release with the script beside this file:
//
//	./run.sh
//
// `go test ./...` in the root module does not reach here, because the go
// command does not descend into a directory that has its own go.mod.
package test
