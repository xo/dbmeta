package dbmeta

import (
	"slices"
	"strconv"
	"strings"
)

// Version is one version a server reports.
//
// The number of components differs per database. Trino reports one, PostgreSQL
// two, SQLite3 three, SQL Server four, and Oracle five. Comparison pads the
// shorter list with zeros, so 16.2 and 16.2.0 are the same version.
//
// Suffix holds any text after the numbers, such as the "MariaDB" in
// "11.4.2-MariaDB". It is recorded and never compared. A flavor is a separate
// axis and belongs to the dialect.
type Version struct {
	// Raw is the text the server reported.
	Raw string
	// Parts are the numeric components, most significant first.
	Parts []uint32
	// Suffix is any text after the numbers.
	Suffix string
	// Unknown reports that the server gave no usable version. A serverless
	// database is the usual case. An unknown version sorts above every known
	// one, because a continuously released database is newest.
	Unknown bool
}

// V builds a Version from its numeric components. Use it for the minimum
// version of a fragment, where only the numbers matter.
func V(parts ...uint32) Version {
	return Version{Parts: parts}
}

// ParseVersion reads a version from the text a server reported. It takes the
// leading numeric components and records the rest as the suffix.
//
// It accepts a leading "v", which DuckDB reports. It returns a version marked
// Unknown, and no error, for text with no leading number, which is how YDB
// reports "<unknown>".
func ParseVersion(s string) Version {
	ver := Version{Raw: s}
	rest := strings.TrimSpace(s)
	rest = strings.TrimPrefix(rest, "v")
	for {
		i := 0
		for i < len(rest) && rest[i] >= '0' && rest[i] <= '9' {
			i++
		}
		if i == 0 {
			break
		}
		n, err := strconv.ParseUint(rest[:i], 10, 32)
		if err != nil {
			break
		}
		ver.Parts = append(ver.Parts, uint32(n))
		rest = rest[i:]
		if !strings.HasPrefix(rest, ".") {
			break
		}
		rest = rest[1:]
	}
	ver.Suffix = strings.TrimLeft(strings.TrimSpace(rest), "-_ ")
	ver.Unknown = len(ver.Parts) == 0
	return ver
}

// ParseVersionNum reads the integer a PostgreSQL server reports for
// server_version_num.
//
// The encoding changed at release 10. Below 100000 the integer packs three
// numbers, so 90624 is 9.6.24. At 100000 and above it packs two, so 180006 is
// 18.6. The second number lands in the same position either way, so this
// agrees with [ParseVersion] about the same server.
func ParseVersionNum(num int) Version {
	switch {
	case num < 0:
		return Version{Unknown: true}
	case num < 100000:
		return Version{Parts: []uint32{uint32(num / 10000), uint32(num % 10000 / 100), uint32(num % 100)}}
	}
	return Version{Parts: []uint32{uint32(num / 10000), uint32(num % 10000)}}
}

// Compare compares v and ver, padding the shorter component list with zeros.
// It returns a negative number when v is older, a positive number when v is
// newer, and zero when they are the same. An unknown version is newer than
// every known one.
func (v Version) Compare(ver Version) int {
	switch {
	case v.Unknown && ver.Unknown:
		return 0
	case v.Unknown:
		return 1
	case ver.Unknown:
		return -1
	}
	for i := range max(len(v.Parts), len(ver.Parts)) {
		a, b := v.part(i), ver.part(i)
		if a != b {
			return int(a) - int(b)
		}
	}
	return 0
}

// part returns the component at i, or zero when v is shorter than that.
func (v Version) part(i int) uint32 {
	if i < len(v.Parts) {
		return v.Parts[i]
	}
	return 0
}

// AtLeast reports whether v is ver or newer.
func (v Version) AtLeast(ver Version) bool {
	return v.Compare(ver) >= 0
}

// IsZero reports whether v carries no information at all.
func (v Version) IsZero() bool {
	return len(v.Parts) == 0 && v.Suffix == "" && v.Raw == "" && !v.Unknown
}

// String satisfies the [fmt.Stringer] interface.
func (v Version) String() string {
	if v.Unknown {
		return "unknown"
	}
	s := make([]string, len(v.Parts))
	for i, p := range v.Parts {
		s[i] = strconv.FormatUint(uint64(p), 10)
	}
	out := strings.Join(s, ".")
	if v.Suffix != "" {
		out += "-" + v.Suffix
	}
	return out
}

// VersionSet is every version one server reports.
//
// Most databases report one, under the empty key. Cassandra reports three that
// move independently, under the keys "release", "cql" and "protocol", so a
// fragment says which one it gates on.
type VersionSet struct {
	// Versions holds each reported version by name. The empty name is the
	// main one.
	Versions map[string]Version
	// Display is the line to show a person, built by the dialect that parsed
	// it, such as "Microsoft SQL Server 16.0.4115.5, RTM-CU12, Developer
	// Edition".
	Display string
}

// Set records ver under key. The empty key is the main version.
func (s *VersionSet) Set(key string, ver Version) {
	if s.Versions == nil {
		s.Versions = make(map[string]Version)
	}
	s.Versions[key] = ver
}

// Has reports whether key was recorded.
//
// A named key is a fact about the server, so its absence is a fact too. A
// server that reports no "mariadb" version is not a MariaDB server, and a
// fragment naming that key must not apply to it. Read this rather than [Get],
// which cannot tell an absent key from an unreadable version. See D44.
func (s VersionSet) Has(key string) bool {
	_, ok := s.Versions[key]
	return ok
}

// Keys returns every key recorded, sorted, so that a caller can show what the
// server reported.
func (s VersionSet) Keys() []string {
	out := make([]string, 0, len(s.Versions))
	for key := range s.Versions {
		out = append(out, key)
	}
	slices.Sort(out)
	return out
}

// Get returns the version recorded under key. A key that was never set returns
// an unknown version, so a fragment gating on a version the server does not
// report is treated as newest rather than failing.
//
// A caller deciding whether a fragment applies calls [Has] first. An unknown
// version is newer than every known one, so Get alone makes an absent key look
// like the newest possible server.
func (s VersionSet) Get(key string) Version {
	if ver, ok := s.Versions[key]; ok {
		return ver
	}
	return Version{Unknown: true}
}

// Main returns the version under the empty key.
func (s VersionSet) Main() Version {
	return s.Get("")
}

// String satisfies the [fmt.Stringer] interface. It returns Display when the
// dialect set one.
func (s VersionSet) String() string {
	if s.Display != "" {
		return s.Display
	}
	return s.Main().String()
}
