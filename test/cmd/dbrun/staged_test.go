package main

import (
	"strings"
	"testing"

	"github.com/xo/dbmeta"
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

// TestTheDSNIsWhatTheDriverTakes holds D167. Every command and test connects
// with the DSN. A product whose driver takes another form than its HTTP
// address has that form as its DSN, and gives the HTTP address, which
// dbimp's tools read, as its API. A server whose DSN is an HTTP address
// gives that as its API.
func TestTheDSNIsWhatTheDriverTakes(t *testing.T) {
	ownAPI := map[string]bool{
		"libsql": true, "neo4j": true, "arangodb": true, "surrealdb": true,
		"druid": true, "drill": true, "solr": true, "elasticsearch": true,
		"opensearch": true, "dynamodb": true, "alternator": true,
	}
	var found int
	for _, x := range targets() {
		if x.Kind != kindContainer {
			continue
		}
		if got := x.connectDSN(); got != x.DSN {
			t.Errorf("%s connects with %q, and the expected string is its DSN %q", x.Name, got, x.DSN)
		}
		http := strings.HasPrefix(x.DSN, "http://") || strings.HasPrefix(x.DSN, "https://")
		if http && x.API != x.DSN {
			t.Errorf("%s: the API is %q, and the expected value is the DSN %q", x.Name, x.API, x.DSN)
		}
		// InfluxDB 3 is the same product and takes its URL as its DSN.
		own := ownAPI[x.Product] || x.Product == "influxdb" && x.Dialect == dbmeta.InfluxQL
		if !own {
			continue
		}
		found++
		if http {
			t.Errorf("%s: the DSN is the HTTP address %q, and the driver takes another form", x.Name, x.DSN)
		}
		if len(x.Principals) == 0 {
			t.Errorf("%s names no principal", x.Name)
		}
		for _, p := range x.Principals {
			if !strings.HasPrefix(p.API, "http://") {
				t.Errorf("%s: the %s has the API %q, and the expected value is an http:// address", x.Name, p.Role, p.API)
			}
		}
	}
	if found == 0 {
		t.Error("expected servers that give an API of their own")
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
