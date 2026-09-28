package container_test

import (
	"testing"

	"github.com/xo/dbmeta/container"

	_ "github.com/xo/dbmeta/all"
)

// TestAReleaseIsStagedExactlyWhenNoModelReadsIt holds the rule of D119. A
// release that no model reads tests nothing in CI, so it must be Staged, and
// a release that a model reads must not be, or CI would skip a model it has.
// The test fails in the change that adds a model and leaves its releases
// Staged, and in the change that adds an entry with no model to CI.
//
// A model is one that all registers, which is every model in the module.
func TestAReleaseIsStagedExactlyWhenNoModelReadsIt(t *testing.T) {
	t.Parallel()
	for _, s := range container.All() {
		_, read := s.Dialect.Info()
		switch {
		case read && s.Tier == container.Staged:
			t.Errorf("%s is Staged, and the model for %s reads it. Move it to a tier CI runs",
				s.Name(), s.Dialect)
		case !read && s.Tier != container.Staged:
			t.Errorf("%s is %s, and no model reads it, so CI would test nothing. Make it Staged",
				s.Name(), s.Tier)
		}
	}
	for _, m := range container.Machines() {
		if _, read := m.Dialect.Info(); !read {
			t.Errorf("%s is a machine, and no model reads %s", m.Name(), m.Dialect)
		}
	}
}
