package main

import (
	"testing"

	"github.com/xo/dbmeta/container"
)

// TestEveryStagedTargetHasACadence holds D120 for every kind of target: the
// servers, the machines, the libraries and the hosted services. A project
// that runs Staged targets, such as dbimp, chooses which run on each push by
// the cadence, so a Staged target without one runs nowhere.
func TestEveryStagedTargetHasACadence(t *testing.T) {
	for _, x := range targets() {
		switch {
		case x.Tier == container.Staged && x.Cadence == "":
			t.Errorf("%s is Staged and has no cadence", x.Name)
		case x.Tier != container.Staged && x.Cadence != "":
			t.Errorf("%s is %s and has the cadence %s, which only a Staged target has", x.Name, x.Tier, x.Cadence)
		}
	}
}

// TestAServerConnectsWithItsURLWhenItSaysSo holds D160. The tests and dbrun
// version connect to libSQL with the libsql:// URL, which is the only form
// dbimp's driver takes, and dsn prints the http:// address as before, which
// dbimp's recorder reads (D153).
func TestAServerConnectsWithItsURLWhenItSaysSo(t *testing.T) {
	var found bool
	for _, x := range targets() {
		if x.Kind != kindContainer {
			continue
		}
		want := x.DSN
		if x.connectURL {
			want = x.URL
		}
		if got := x.connectDSN(); got != want {
			t.Errorf("%s connects with %q, and the expected string is %q", x.Name, got, want)
		}
		if x.Product != "libsql" {
			continue
		}
		found = true
		if !x.connectURL {
			t.Errorf("%s does not connect with its URL", x.Name)
		}
		if env := x.env()[0]; env != x.Env+"="+x.URL {
			t.Errorf("%s: the test variable is %q, and the expected value is the URL", x.Name, env)
		}
		if x.DSN == x.URL {
			t.Errorf("%s: the DSN is the URL, %q, and dbimp's recorder reads the http:// address", x.Name, x.DSN)
		}
	}
	if !found {
		t.Error("expected a libSQL server")
	}
}

// TestEveryModelledServerHasADriver holds that dbrun version can read every
// server a model reads. A dialect missing from drivers answered "no driver
// for cockroachdb" the day its model arrived.
func TestEveryModelledServerHasADriver(t *testing.T) {
	for _, s := range container.All() {
		if _, read := s.Dialect.Info(); !read {
			continue
		}
		if _, ok := drivers[s.Dialect]; !ok {
			t.Errorf("%s: a model reads %s and dbrun has no driver to open it with", s.Name(), s.Dialect)
		}
	}
}
