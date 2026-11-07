package container_test

import (
	"regexp"
	"testing"

	"github.com/xo/dbmeta/container"
)

var digest = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

// TestEveryClusterIsPinned checks that each node image has a digest, because
// kind says that only the digest guarantees the image built for its release,
// and that the release is the line of the version.
func TestEveryClusterIsPinned(t *testing.T) {
	t.Parallel()
	seen := map[string]bool{}
	for _, c := range container.Clusters() {
		if !digest.MatchString(c.Digest) {
			t.Errorf("%s has the digest %q, and the expected form is sha256 and 64 hex digits", c.Name(), c.Digest)
		}
		if got := container.Line(c.Version); got != c.Release {
			t.Errorf("%s has the version %s, which is the line %s", c.Name(), c.Version, got)
		}
		if seen[c.Name()] {
			t.Errorf("%s is listed twice", c.Name())
		}
		seen[c.Name()] = true
	}
}

// TestAClusterIsNotAServer checks that no cluster is in the list of servers,
// because a selector such as all or tested must never start one.
func TestAClusterIsNotAServer(t *testing.T) {
	t.Parallel()
	for _, c := range container.Clusters() {
		for _, s := range container.All() {
			if s.Name() == c.Name() {
				t.Errorf("%s is a cluster and a server", c.Name())
			}
		}
	}
}
