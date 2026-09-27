package container_test

import (
	"strings"
	"testing"

	"github.com/xo/dbmeta/container"
)

// TestEveryServerNamesItsPrincipals checks what dsn --json prints for each
// server: the administrator first, named, and each ordinary user after it
// with a connection string that holds its own name. See D102.
func TestEveryServerNamesItsPrincipals(t *testing.T) {
	t.Parallel()
	for _, s := range container.All() {
		ps := s.Principals()
		if len(ps) == 0 || ps[0].Role != container.Administrator {
			t.Errorf("%s: the first principal is not the administrator: %+v", s.Name(), ps)
			continue
		}
		if ps[0].User == "" {
			t.Errorf("%s: the administrator has no name in the URL %s", s.Name(), s.URL(1))
		}
		if ps[0].DSN(1) != s.DSN(1) {
			t.Errorf("%s: the administrator's DSN differs from the server's", s.Name())
		}
		for _, p := range ps[1:] {
			if p.Role != container.User {
				t.Errorf("%s: %s has the role %q after the administrator", s.Name(), p.User, p.Role)
			}
			if !strings.Contains(p.DSN(1), p.User) {
				t.Errorf("%s: the DSN of %s does not name it: %s", s.Name(), p.User, p.DSN(1))
			}
		}
	}
}

// TestCouchbaseListsItsOrdinaryUser pins the one server whose setup makes an
// ordinary user today.
func TestCouchbaseListsItsOrdinaryUser(t *testing.T) {
	t.Parallel()
	for _, s := range container.Couchbase {
		ps := s.Principals()
		if len(ps) != 2 || ps[1].User != container.CouchbaseUser {
			t.Errorf("%s: expected the administrator and %s, got %+v", s.Name(), container.CouchbaseUser, ps)
		}
	}
}
