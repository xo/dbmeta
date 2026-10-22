package main

import (
	"strings"
	"testing"

	"github.com/xo/dbmeta/container"
)

// argvPassword is every product whose client takes the password in its
// arguments. The same list is in the container package, and a product that
// is here and not there, or there and not here, fails one of the two tests.
var argvPassword = map[string]bool{
	"oracle": true, "cassandra": true, "scylla": true, "mongodb": true,
	"hana": true, "arangodb": true, "rqlite": true, "couchbase": true,
	"ksqldb": true, "exasol": true,
}

// TestTheNativeCallHidesThePassword builds the call of every container
// server and holds D236: the password is in the environment where the client
// reads one, and the printed form never holds a password or a value of an
// environment variable.
func TestTheNativeCallHidesThePassword(t *testing.T) {
	t.Parallel()
	var built int
	for _, x := range targets() {
		if x.Kind != kindContainer {
			continue
		}
		call, err := nativeCall(x, "")
		s, _ := serverOf(x)
		if s.Client.Command == "" {
			if err == nil || !strings.Contains(err.Error(), s.Client.Why) ||
				!strings.Contains(err.Error(), "dbrun usql "+x.Name) {
				t.Errorf("%s has no client, and the error is %v", x.Name, err)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s: %v", x.Name, err)
			continue
		}
		built++
		conn, err := container.ParseConnection(x.Principals[0].URL, 0)
		if err != nil {
			t.Fatal(err)
		}
		if got, want := call.InArgs(), argvPassword[x.Product] && conn.Password != ""; got != want {
			t.Errorf("%s: the password is in the arguments is %t, and the expected value is %t", x.Name, got, want)
		}
		out := describeCall(call)
		if conn.Password != "" && strings.Contains(out, conn.Password) {
			t.Errorf("%s prints the password: %s", x.Name, out)
		}
		for _, e := range call.Env {
			if _, value, _ := strings.Cut(e, "="); len(value) > 3 && strings.Contains(out, value) {
				t.Errorf("%s prints the value of an environment variable: %s", x.Name, out)
			}
		}
		if !strings.HasPrefix(out, call.Command) {
			t.Errorf("%s prints %q, which does not start with the command %s", x.Name, out, call.Command)
		}
	}
	if built < 20 {
		t.Errorf("built the call of %d servers, and the expected count is above 20", built)
	}
}

// TestThePasswordIsNotAnArgumentWhereAnEnvironmentVariableExists holds the
// rule for the clients that read one: the same fixed password goes in the
// environment, never in the arguments.
func TestThePasswordIsNotAnArgumentWhereAnEnvironmentVariableExists(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"postgres-18", "mariadb-13.0", "mysql-26.7", "sqlserver-2025", "clickhouse-26.9", "cockroachdb-26.3.2"} {
		x, ok := find(name)
		if !ok {
			t.Logf("%s is not in the list now", name)
			continue
		}
		call, err := nativeCall(x, "")
		if err != nil {
			t.Fatal(err)
		}
		if call.InArgs() {
			t.Errorf("%s puts the password in the arguments: %s", name, describeCall(call))
		}
		if !strings.Contains(strings.Join(call.Env, " "), container.Password) &&
			!strings.Contains(strings.Join(call.Env, " "), "P4ssw0rd%21x") {
			t.Errorf("%s puts the password nowhere: %v", name, call.EnvNames())
		}
	}
}

func find(name string) (target, bool) {
	for _, x := range targets() {
		if x.Name == name {
			return x, true
		}
	}
	return target{}, false
}

// TestPostgresCallIsTheNativeForm checks one call in full.
func TestPostgresCallIsTheNativeForm(t *testing.T) {
	t.Parallel()
	x := target{
		Name: "postgres-9", Product: "postgres", Kind: kindContainer,
		URL: "postgres://postgres:s3cret@127.0.0.1:55001/postgres?sslmode=disable",
		Principals: []principal{
			{Role: "administrator", User: "postgres", URL: "postgres://postgres:s3cret@127.0.0.1:55001/postgres?sslmode=disable"},
			{Role: "user", User: "reader", URL: "postgres://reader:r3ad@127.0.0.1:55001/postgres?sslmode=disable"},
		},
		Run: []string{"--publish", "55001:5432"},
	}
	call, err := nativeCall(x, "reader")
	if err != nil {
		t.Fatal(err)
	}
	want := "psql --host 127.0.0.1 --port 55001 --username reader --dbname postgres"
	if got := call.Command + " " + strings.Join(call.Args, " "); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if got := strings.Join(call.Env, " "); got != "PGPASSWORD=r3ad PGSSLMODE=disable" {
		t.Errorf("the environment is %q", got)
	}
	if got := describeCall(call); strings.Contains(got, "r3ad") || !strings.Contains(got, "PGPASSWORD PGSSLMODE") {
		t.Errorf("the printed form is %q", got)
	}
	if _, err := nativeCall(x, "nobody"); err == nil {
		t.Error("an unknown principal was accepted")
	}
}

// TestTheClickHouseClientUsesTheFirstPort holds that the native client of a
// product whose URL names the second port connects to the first one.
func TestTheClickHouseClientUsesTheFirstPort(t *testing.T) {
	t.Parallel()
	x, ok := find("clickhouse-26.9")
	if !ok {
		t.Skip("clickhouse-26.9 is not in the list")
	}
	call, err := nativeCall(x, "")
	if err != nil {
		t.Fatal(err)
	}
	want := x.wantPorts()["9000"]
	if !strings.Contains(strings.Join(call.Args, " "), "--port "+want) {
		t.Errorf("the arguments %v do not use the native host port %s", call.Args, want)
	}
}

// TestAnEmbeddedDatabaseOpensItsFile holds that the libraries with a shell
// open their file and the others say so.
func TestAnEmbeddedDatabaseOpensItsFile(t *testing.T) {
	t.Parallel()
	for name, command := range map[string]string{"sqlite3": "sqlite3", "moderncsqlite": "sqlite3", "duckdb": "duckdb"} {
		x, ok := find(name)
		if !ok {
			t.Fatalf("%s is not a target", name)
		}
		call, err := nativeCall(x, "")
		if err != nil || call.Command != command || len(call.Args) != 1 || call.Args[0] != x.DSN {
			t.Errorf("%s: %+v, %v", name, call, err)
		}
	}
	for _, name := range []string{"chai", "csvq"} {
		x, _ := find(name)
		if _, err := nativeCall(x, ""); err == nil || !strings.Contains(err.Error(), "dbrun usql "+name) {
			t.Errorf("%s: %v", name, err)
		}
	}
}

// fakeSecret is a connection string for tests. Neither the password nor the
// whole string can appear anywhere a person or a log can read.
const (
	fakePassword = "Zq9-fake-password"
	fakeSecret   = "redshift://admin:" + fakePassword + "@dbmeta.example.us-east-1.redshift.amazonaws.com:5439/dev"
)

// TestRedshiftGoesThroughTheEnvironment holds D236 for a hosted service: the
// password is only in PGPASSWORD, TLS is required, and nothing that is printed
// or returned as an error holds the password or the connection string.
func TestRedshiftGoesThroughTheEnvironment(t *testing.T) {
	t.Parallel()
	x := target{Name: "redshift", Product: "redshift", Kind: kindHosted, secret: fakeSecret}
	call, err := nativeCall(x, "")
	if err != nil {
		t.Fatal(err)
	}
	want := "psql --host dbmeta.example.us-east-1.redshift.amazonaws.com --port 5439 --username admin --dbname dev"
	if got := call.Command + " " + strings.Join(call.Args, " "); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if got := strings.Join(call.Env, " "); got != "PGPASSWORD="+fakePassword+" PGSSLMODE=require" {
		t.Error("the environment is not PGPASSWORD and PGSSLMODE=require")
	}
	if call.InArgs() {
		t.Error("the password is in the arguments")
	}
	for _, a := range call.Args {
		if strings.Contains(a, fakePassword) || strings.Contains(a, "redshift://") {
			t.Error("an argument holds the secret")
		}
	}
	out := describeCall(call)
	if strings.Contains(out, fakePassword) || !strings.Contains(out, "PGPASSWORD PGSSLMODE") {
		t.Errorf("the printed form is %q", out)
	}
}

// TestAHostedErrorNeverHoldsTheSecret holds that every refusal of a hosted
// service leaves the connection string out.
func TestAHostedErrorNeverHoldsTheSecret(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		x    target
		role string
	}{
		"as":       {target{Name: "redshift", Product: "redshift", Kind: kindHosted, secret: fakeSecret}, "reader"},
		"none":     {target{Name: "snowflake", Product: "snowflake", Kind: kindHosted, secret: fakeSecret}, ""},
		"unknown":  {target{Name: "nothing", Product: "nothing", Kind: kindHosted, secret: fakeSecret}, ""},
		"not url":  {target{Name: "redshift", Product: "redshift", Kind: kindHosted, secret: "redshift://admin:" + fakePassword + "@host:port/db"}, ""},
		"bad host": {target{Name: "redshift", Product: "redshift", Kind: kindHosted, secret: "redshift://admin:" + fakePassword + "@ho st/db%zz"}, ""},
	}
	for name, c := range cases {
		_, err := nativeCall(c.x, c.role)
		if err == nil {
			t.Errorf("%s: no error", name)
			continue
		}
		if strings.Contains(err.Error(), fakePassword) || strings.Contains(err.Error(), "redshift://") {
			t.Errorf("%s: the error holds the secret", name)
		}
	}
	if _, err := nativeCall(cases["none"].x, ""); err == nil || !strings.Contains(err.Error(), "dbrun usql snowflake") {
		t.Errorf("a hosted service with no client does not offer usql: %v", err)
	}
}

// TestAMissingClientNamesTheCommandAndOffersUsql holds the message for a host
// that lacks the client.
func TestAMissingClientNamesTheCommandAndOffersUsql(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	_, err := findClient(target{Name: "postgres-18", Product: "postgres"}, "psql")
	if err == nil {
		t.Fatal("a client was found on an empty path")
	}
	for _, want := range []string{"psql", "postgres", "dbrun usql postgres-18"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the message %q does not hold %q", err, want)
		}
	}
}
