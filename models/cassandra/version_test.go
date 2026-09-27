package cassandra

import (
	"errors"
	"testing"

	"github.com/xo/dbmeta"
)

// The row versionQuery returns, as each product sent it. The token lists are
// cut to two entries and the truncation map to one, and nothing else is
// changed.
const (
	cassandraRow = `{"key": "local", "bootstrapped": "COMPLETED",` +
		` "cluster_name": "Test Cluster", "cql_version": "3.4.7",` +
		` "data_center": "datacenter1", "native_protocol_version": "5",` +
		` "partitioner": "org.apache.cassandra.dht.Murmur3Partitioner",` +
		` "rack": "rack1", "release_version": "5.0.9",` +
		` "tokens": ["-2376720142537761243", "-3221096790028922999"],` +
		` "truncated_at": {"176c39cd-b93d-33a5-a218-8eb06a56f66e":` +
		` "0x000001a0e0784a77000002af000001a0e0784eae"}}`
	scyllaRow = `{"key": "local", "bootstrapped": "COMPLETED",` +
		` "cluster_name": "", "cql_version": "3.3.1",` +
		` "data_center": "datacenter1", "native_protocol_version": "4",` +
		` "partitioner": "org.apache.cassandra.dht.Murmur3Partitioner",` +
		` "rack": "rack1", "release_version": "3.0.8",` +
		` "supported_features": "CDC,MATERIALIZED_VIEWS,TABLETS,UDF",` +
		` "tokens": ["-1086239105645841032", "-116781751103807603"],` +
		` "truncated_at": null}`
)

// TestParseVersion reads each product's row and checks which product the
// model decides it is talking to. ScyllaDB is told apart by the
// supported_features column, and nothing else in the row says so.
func TestParseVersion(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name    string
		row     string
		release string
		scylla  bool
		display string
	}{
		{
			name: "cassandra", row: cassandraRow, release: "5.0.9",
			display: "Cassandra 5.0.9, CQL 3.4.7, Protocol v5",
		},
		{
			name: "scylla", row: scyllaRow, release: "3.0.8", scylla: true,
			display: "ScyllaDB, compatible with Cassandra 3.0.8, CQL 3.3.1, Protocol v4",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			s, err := parseVersion([]string{test.row})
			if err != nil {
				t.Fatalf("parsing: %v", err)
			}
			if got := s.Main().String(); got != test.release {
				t.Errorf("main version: got %s, want %s", got, test.release)
			}
			if got := s.Has(Scylla); got != test.scylla {
				t.Errorf("scylla key: got %v, want %v", got, test.scylla)
			}
			if s.Display != test.display {
				t.Errorf("display: got %q, want %q", s.Display, test.display)
			}
			for _, key := range []string{"cql", "protocol"} {
				if !s.Has(key) {
					t.Errorf("expected the %s version to be recorded", key)
				}
			}
		})
	}
}

// TestParseVersionRefusesAPartialRow checks that a row missing a version is an
// error rather than a version of zero.
func TestParseVersionRefusesAPartialRow(t *testing.T) {
	t.Parallel()
	for _, cols := range [][]string{
		nil,
		{`not json`},
		{`{"release_version": "5.0.9", "cql_version": "3.4.7"}`},
	} {
		if _, err := parseVersion(cols); !errors.Is(err, dbmeta.ErrInvalidVersion) {
			t.Errorf("%q: got %v, want ErrInvalidVersion", cols, err)
		}
	}
}

// TestParseFollowUp reads the ScyllaDB release out of system.versions and
// checks that it lands under the Scylla key and in the display line, and that
// the main version stays the Cassandra release the first row reported.
func TestParseFollowUp(t *testing.T) {
	t.Parallel()
	first, err := parseVersion([]string{scyllaRow})
	if err != nil {
		t.Fatalf("parsing the first row: %v", err)
	}
	if q, n := followUpQuery(first); q == "" || n != 1 {
		t.Fatalf("expected a follow-up of one column, got %q and %d", q, n)
	}
	s, err := parseFollowUp(first, []string{"2026.3.1-0.20260904.97cbf7898aae"})
	if err != nil {
		t.Fatalf("parsing the follow-up: %v", err)
	}
	if got := s.Get(Scylla).Parts; len(got) < 2 || got[0] != 2026 || got[1] != 3 {
		t.Errorf("scylla release: got %v, want 2026.3", got)
	}
	if got := s.Main().String(); got != "3.0.8" {
		t.Errorf("main version: got %s, want 3.0.8", got)
	}
	want := "ScyllaDB 2026.3.1-0.20260904.97cbf7898aae, compatible with Cassandra 3.0.8," +
		" CQL 3.3.1, Protocol v4"
	if s.Display != want {
		t.Errorf("display: got %q, want %q", s.Display, want)
	}
	if _, err := parseFollowUp(first, []string{""}); !errors.Is(err, dbmeta.ErrInvalidVersion) {
		t.Errorf("an empty release: got %v, want ErrInvalidVersion", err)
	}
	// Cassandra is asked nothing more.
	cass, err := parseVersion([]string{cassandraRow})
	if err != nil {
		t.Fatalf("parsing the Cassandra row: %v", err)
	}
	if q, _ := followUpQuery(cass); q != "" {
		t.Errorf("expected no follow-up for Cassandra, got %q", q)
	}
}
