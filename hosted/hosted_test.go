package hosted_test

import (
	"testing"

	"github.com/xo/dbmeta/container"
	"github.com/xo/dbmeta/hosted"

	_ "github.com/xo/dbmeta/all"
)

// TestEveryEmulatorHasAnEntry holds that a service which names an emulator
// names a product that container lists, so that a person without an account
// finds the entry the service points at.
func TestEveryEmulatorHasAnEntry(t *testing.T) {
	t.Parallel()
	products := map[string]bool{}
	for _, s := range container.All() {
		products[s.Product] = true
	}
	for _, s := range hosted.All() {
		if s.Emulator != "" && !products[s.Emulator] {
			t.Errorf("%s names the emulator %s, which container does not list", s.Name, s.Emulator)
		}
	}
}

// TestAServiceIsStagedExactlyWhenNoModelReadsIt holds the rule of D119 for a
// hosted service: Verified when a model reads it, and Staged when none does.
// Neither is a tier CI runs.
func TestAServiceIsStagedExactlyWhenNoModelReadsIt(t *testing.T) {
	t.Parallel()
	for _, s := range hosted.All() {
		want := container.Staged
		if _, read := s.Dialect.Info(); read {
			want = container.Verified
		}
		if s.Tier != want {
			t.Errorf("%s is %s, and it must be %s", s.Name, s.Tier, want)
		}
	}
}
