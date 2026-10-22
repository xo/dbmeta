package container_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/xo/dbmeta/container"
)

// argvPassword is every product whose native client takes the password in its
// arguments, where a process list shows it. Each one has no environment
// variable or file option that dbrun uses. A product is added here only with
// the reason written in its client. See D236.
var argvPassword = []string{
	"oracle", "cassandra", "scylla", "mongodb", "hana", "arangodb", "rqlite",
	"couchbase", "ksqldb", "exasol",
}

// TestEveryServerNamesAClientOrSaysWhy holds D236. A product either names the
// native client that dbrun cli runs, or says why it has none, and never both.
func TestEveryServerNamesAClientOrSaysWhy(t *testing.T) {
	t.Parallel()
	for _, s := range container.All() {
		c := s.Client
		switch {
		case c.Command == "" && c.Why == "":
			t.Errorf("%s names no native client and gives no reason", s.Name())
		case c.Command != "" && c.Why != "":
			t.Errorf("%s names the client %s and also gives the reason %q", s.Name(), c.Command, c.Why)
		}
	}
}

// TestTheNativeClientBuildsACall builds the call of every client for every
// principal, and checks that the password is in the arguments of exactly the
// products in argvPassword.
func TestTheNativeClientBuildsACall(t *testing.T) {
	t.Parallel()
	for i, s := range container.All() {
		if s.Client.Command == "" {
			continue
		}
		first := 55000 + i
		for _, p := range s.Principals() {
			conn, err := container.ParseConnection(p.URL(first), first)
			if err != nil {
				t.Errorf("%s: %v", s.Name(), err)
				continue
			}
			call := s.Client.Call(conn)
			if call.Command != s.Client.Command {
				t.Errorf("%s: the call runs %q and the client is %q", s.Name(), call.Command, s.Client.Command)
			}
			if len(call.Args) == 0 && len(call.Env) == 0 {
				t.Errorf("%s: the call of %s has no arguments and no environment", s.Name(), call.Command)
			}
			if got, want := call.InArgs(), slices.Contains(argvPassword, strings.ToLower(s.Product)) && conn.Password != ""; got != want {
				t.Errorf("%s as %s: the password is in the arguments is %t, and the expected value is %t",
					s.Name(), p.Role, got, want)
			}
			for _, a := range call.Masked() {
				if conn.Password != "" && strings.Contains(a, conn.Password) {
					t.Errorf("%s: the masked arguments still hold the password", s.Name())
				}
			}
			for _, name := range call.EnvNames() {
				if name == "" || strings.Contains(name, "=") {
					t.Errorf("%s: the environment name %q is not a name", s.Name(), name)
				}
			}
		}
	}
}

// TestParseConnection reads the parts of one URL.
func TestParseConnection(t *testing.T) {
	t.Parallel()
	c, err := container.ParseConnection("postgres://u%40x:p%21@127.0.0.1:55001/db?sslmode=disable", 0)
	if err != nil {
		t.Fatal(err)
	}
	if c.Host != "127.0.0.1" || c.Port != 55001 || c.First != 55001 || c.User != "u@x" ||
		c.Password != "p!" || c.Database != "db" || c.Query.Get("sslmode") != "disable" {
		t.Errorf("read %+v", c)
	}
}
