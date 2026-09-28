package main

import (
	"testing"

	"github.com/xo/dbmeta/container"
)

// TestEveryStagedTargetHasACadence holds D120 for every kind of target: the
// servers, the machines, the libraries and the hosted services. A project
// that runs Staged targets, such as dbimp, chooses which run on each push by
// the cadence, so a Staged target without one would run nowhere.
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
