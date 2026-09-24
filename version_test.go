package dbmeta

import "testing"

func TestParseVersion(t *testing.T) {
	t.Parallel()
	tests := []struct {
		s      string
		parts  []uint32
		suffix string
		unk    bool
	}{
		{"443", []uint32{443}, "", false},                        // trino
		{"0.287", []uint32{0, 287}, "", false},                   // presto
		{"16.2", []uint32{16, 2}, "", false},                     // postgres
		{"3.45.1", []uint32{3, 45, 1}, "", false},                // sqlite3
		{"11.4.2-MariaDB", []uint32{11, 4, 2}, "MariaDB", false}, // mariadb
		{"v1.1.3", []uint32{1, 1, 3}, "", false},                 // duckdb
		{"24.3.1.2672", []uint32{24, 3, 1, 2672}, "", false},     // clickhouse
		{"16.0.4115.5", []uint32{16, 0, 4115, 5}, "", false},     // sqlserver
		{"19.3.0.0.0", []uint32{19, 3, 0, 0, 0}, "", false},      // oracle
		{"<unknown>", nil, "<unknown>", true},                    // ydb
		{"", nil, "", true},
	}
	for _, test := range tests {
		t.Run(test.s, func(t *testing.T) {
			t.Parallel()
			ver := ParseVersion(test.s)
			if ver.Unknown != test.unk {
				t.Fatalf("expected unknown %v, got %v", test.unk, ver.Unknown)
			}
			if len(ver.Parts) != len(test.parts) {
				t.Fatalf("expected %v, got %v", test.parts, ver.Parts)
			}
			for i := range test.parts {
				if ver.Parts[i] != test.parts[i] {
					t.Fatalf("expected %v, got %v", test.parts, ver.Parts)
				}
			}
			if ver.Suffix != test.suffix {
				t.Errorf("expected suffix %q, got %q", test.suffix, ver.Suffix)
			}
		})
	}
}

// TestCompareDiffersInLength is the case a three field version type could not
// express. Oracle reports five components and SQL Server four.
func TestCompareDiffersInLength(t *testing.T) {
	t.Parallel()
	if ParseVersion("16.2").Compare(ParseVersion("16.2.0.0.0")) != 0 {
		t.Error("expected 16.2 to equal 16.2.0.0.0")
	}
	if !ParseVersion("19.3.0.0.1").AtLeast(ParseVersion("19.3")) {
		t.Error("expected 19.3.0.0.1 to be at least 19.3")
	}
	if ParseVersion("443").AtLeast(ParseVersion("444")) {
		t.Error("expected 443 not to be at least 444")
	}
}

// TestSuffixIsNotRanked checks that a flavor never changes the order. D14
// makes the flavor a separate axis.
func TestSuffixIsNotRanked(t *testing.T) {
	t.Parallel()
	if ParseVersion("11.4.2-MariaDB").Compare(ParseVersion("11.4.2")) != 0 {
		t.Error("expected the suffix to be recorded and not compared")
	}
}

// TestUnknownIsNewest checks the rule D21 sets above the ceiling: a database
// that reports no version is continuously released, so it is newest.
func TestUnknownIsNewest(t *testing.T) {
	t.Parallel()
	unk := ParseVersion("<unknown>")
	if !unk.AtLeast(ParseVersion("19.0")) {
		t.Error("expected an unknown version to be at least any known one")
	}
	if unk.Compare(ParseVersion("<unknown>")) != 0 {
		t.Error("expected two unknown versions to compare equal")
	}
}

// TestVersionNumMatchesText checks that the two ways of reading a PostgreSQL
// version agree, across the release 10 encoding change.
func TestVersionNumMatchesText(t *testing.T) {
	t.Parallel()
	for num, s := range map[int]string{
		90624:  "9.6.24",
		100003: "10.3",
		140012: "14.12",
		180006: "18.6",
	} {
		if ParseVersionNum(num).Compare(ParseVersion(s)) != 0 {
			t.Errorf("%d and %q disagree: %v and %v", num, s, ParseVersionNum(num), ParseVersion(s))
		}
	}
}

func TestVersionSet(t *testing.T) {
	t.Parallel()
	var s VersionSet
	s.Set("", ParseVersion("4.1.3"))
	s.Set("cql", ParseVersion("3.4.6"))
	s.Display = "Cassandra 4.1.3, CQL 3.4.6"
	if got := s.Main().String(); got != "4.1.3" {
		t.Errorf("expected 4.1.3, got %s", got)
	}
	if got := s.Get("cql").String(); got != "3.4.6" {
		t.Errorf("expected 3.4.6, got %s", got)
	}
	// a key the server never reported is unknown, so a fragment gating on it
	// is treated as newest rather than failing
	if !s.Get("protocol").Unknown {
		t.Error("expected an absent key to be unknown")
	}
	if got := s.String(); got != "Cassandra 4.1.3, CQL 3.4.6" {
		t.Errorf("expected the display line, got %s", got)
	}
}
