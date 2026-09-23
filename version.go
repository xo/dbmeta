package dbmeta

import (
	"strconv"
	"strings"
)

// Version is a database server version.
//
// A model reads the version from the server and converts it to this form. What
// counts as a major version differs per database, and this type does not
// decide that. It records three numbers and compares them in order.
//
// PostgreSQL is the case to watch. Before release 10 the major version is the
// first two numbers, so 9.6 is a major version and the third number is the
// patch. From release 10 the major version is the first number alone and the
// second is the patch. Use [ParseVersionNum] to read the integer that a
// PostgreSQL server reports, which handles both.
type Version struct {
	Major int
	Minor int
	Patch int
}

// ParseVersion parses a dotted version, such as "18.6" or "9.6.24". Leading
// and trailing space is ignored. A missing minor or patch is zero.
func ParseVersion(s string) (Version, error) {
	var ver Version
	dst := []*int{&ver.Major, &ver.Minor, &ver.Patch}
	if s = strings.TrimSpace(s); s == "" {
		return ver, ErrInvalidVersion
	}
	for i, part := range strings.SplitN(s, ".", 4) {
		if i == len(dst) {
			return Version{}, ErrInvalidVersion
		}
		n, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil || n < 0 {
			return Version{}, ErrInvalidVersion
		}
		*dst[i] = n
	}
	return ver, nil
}

// ParseVersionNum converts the integer a PostgreSQL server reports for
// server_version_num into a [Version].
//
// The encoding changed at release 10. Below 100000 the integer packs three
// numbers, so 90624 is 9.6.24. At 100000 and above it packs two, so 180006 is
// 18.6.
//
// The second number goes in Minor in both cases, so that ParseVersionNum and
// [ParseVersion] agree about the same server. PostgreSQL calls the 6 in 18.6 a
// patch release, and a person writing that version writes it where a minor
// goes. Matching what a person writes matters more here than matching what
// PostgreSQL calls it.
func ParseVersionNum(num int) (Version, error) {
	if num < 0 {
		return Version{}, ErrInvalidVersion
	}
	if num < 100000 {
		return Version{
			Major: num / 10000,
			Minor: num % 10000 / 100,
			Patch: num % 100,
		}, nil
	}
	return Version{
		Major: num / 10000,
		Minor: num % 10000,
	}, nil
}

// Compare compares v and ver. It returns a negative number when v is older, a
// positive number when v is newer, and zero when they are the same.
func (v Version) Compare(ver Version) int {
	if n := v.Major - ver.Major; n != 0 {
		return n
	}
	if n := v.Minor - ver.Minor; n != 0 {
		return n
	}
	return v.Patch - ver.Patch
}

// AtLeast reports whether v is ver or newer.
func (v Version) AtLeast(ver Version) bool {
	return v.Compare(ver) >= 0
}

// Before reports whether v is older than ver.
func (v Version) Before(ver Version) bool {
	return v.Compare(ver) < 0
}

// IsZero reports whether v is the zero version. A zero version means the
// version is not known.
func (v Version) IsZero() bool {
	return v == Version{}
}

// String satisfies the [fmt.Stringer] interface. It writes the patch only when
// it is not zero.
func (v Version) String() string {
	s := strconv.Itoa(v.Major) + "." + strconv.Itoa(v.Minor)
	if v.Patch != 0 {
		s += "." + strconv.Itoa(v.Patch)
	}
	return s
}
