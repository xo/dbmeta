package dbmeta

import (
	"errors"
	"testing"
)

func TestParseVersion(t *testing.T) {
	t.Parallel()
	tests := []struct {
		s    string
		exp  Version
		fail bool
	}{
		{"18", Version{18, 0, 0}, false},
		{"18.6", Version{18, 6, 0}, false},
		{"9.6.24", Version{9, 6, 24}, false},
		{"  11.4.2 ", Version{11, 4, 2}, false},
		{"8.0.36", Version{8, 0, 36}, false},
		{"", Version{}, true},
		{"x", Version{}, true},
		{"1.2.3.4", Version{}, true},
		{"-1", Version{}, true},
		{"1..2", Version{}, true},
	}
	for _, test := range tests {
		t.Run(test.s, func(t *testing.T) {
			t.Parallel()
			ver, err := ParseVersion(test.s)
			switch {
			case test.fail && !errors.Is(err, ErrInvalidVersion):
				t.Fatalf("expected ErrInvalidVersion, got: %v", err)
			case test.fail:
				return
			case err != nil:
				t.Fatalf("expected no error, got: %v", err)
			case ver != test.exp:
				t.Errorf("expected %v, got %v", test.exp, ver)
			}
		})
	}
}

// TestParseVersionNum covers the PostgreSQL encoding change at release 10.
// Below 100000 the integer packs three numbers. At 100000 and above it packs
// two.
func TestParseVersionNum(t *testing.T) {
	t.Parallel()
	tests := []struct {
		num int
		exp Version
	}{
		{90600, Version{9, 6, 0}},
		{90624, Version{9, 6, 24}},
		{90300, Version{9, 3, 0}},
		{100000, Version{10, 0, 0}},
		{100003, Version{10, 3, 0}},
		{140000, Version{14, 0, 0}},
		{180006, Version{18, 6, 0}},
	}
	for _, test := range tests {
		ver, err := ParseVersionNum(test.num)
		if err != nil {
			t.Fatalf("%d: expected no error, got: %v", test.num, err)
		}
		if ver != test.exp {
			t.Errorf("%d: expected %v, got %v", test.num, test.exp, ver)
		}
	}
	if _, err := ParseVersionNum(-1); !errors.Is(err, ErrInvalidVersion) {
		t.Errorf("expected ErrInvalidVersion, got: %v", err)
	}
}

// TestVersionNumOrdering checks that the integers a PostgreSQL server reports
// keep their order across the release 10 encoding change. Selection depends on
// this.
func TestVersionNumOrdering(t *testing.T) {
	t.Parallel()
	nums := []int{90300, 90400, 90500, 90600, 100000, 110000, 120000, 130000, 140000, 150000, 160000, 170000, 180000}
	var last Version
	for _, num := range nums {
		ver, err := ParseVersionNum(num)
		if err != nil {
			t.Fatalf("%d: expected no error, got: %v", num, err)
		}
		if !ver.AtLeast(last) || ver == last {
			t.Fatalf("%d: expected %v to be newer than %v", num, ver, last)
		}
		last = ver
	}
}

// TestVersionSourcesAgree checks that the two ways of reading a version give
// the same answer for the same server. A model reads whichever the server
// offers, so they must not disagree.
func TestVersionSourcesAgree(t *testing.T) {
	t.Parallel()
	tests := []struct {
		num int
		s   string
	}{
		{90624, "9.6.24"},
		{100003, "10.3"},
		{140012, "14.12"},
		{180006, "18.6"},
	}
	for _, test := range tests {
		fromNum, err := ParseVersionNum(test.num)
		if err != nil {
			t.Fatalf("%d: expected no error, got: %v", test.num, err)
		}
		fromStr, err := ParseVersion(test.s)
		if err != nil {
			t.Fatalf("%s: expected no error, got: %v", test.s, err)
		}
		if fromNum != fromStr {
			t.Errorf("%d and %q disagree: %v and %v", test.num, test.s, fromNum, fromStr)
		}
	}
}

func TestVersionCompare(t *testing.T) {
	t.Parallel()
	v96, v10, v18 := Version{9, 6, 0}, Version{10, 0, 0}, Version{18, 0, 0}
	switch {
	case !v96.Before(v10):
		t.Error("expected 9.6 to be older than 10")
	case !v18.AtLeast(v10):
		t.Error("expected 18 to be at least 10")
	case !v10.AtLeast(v10):
		t.Error("expected 10 to be at least itself")
	case v96.Compare(v96) != 0:
		t.Error("expected 9.6 to compare equal to itself")
	case !Version{}.IsZero():
		t.Error("expected the zero version to report itself as zero")
	case v96.IsZero():
		t.Error("expected 9.6 not to report itself as zero")
	}
	// the patch must not outrank the minor
	if !(Version{9, 6, 24}).Before(Version{9, 7, 0}) {
		t.Error("expected 9.6.24 to be older than 9.7")
	}
}

func TestVersionString(t *testing.T) {
	t.Parallel()
	tests := []struct {
		ver Version
		exp string
	}{
		{Version{18, 0, 0}, "18.0"},
		{Version{18, 6, 0}, "18.6"},
		{Version{9, 6, 24}, "9.6.24"},
	}
	for _, test := range tests {
		if s := test.ver.String(); s != test.exp {
			t.Errorf("expected %q, got %q", test.exp, s)
		}
	}
}
