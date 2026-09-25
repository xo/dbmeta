package container_test

import (
	"os"
	"strings"
	"testing"

	"github.com/xo/dbmeta/container"
)

// coverage is the document that states what each release is verified against.
const coverage = "../docs/COVERAGE.md"

// TestEveryVerifiedReleaseIsDocumented checks the Verified tier against the
// document that claims it.
//
// The Tested and Nightly tiers have a machine behind them: CI runs those
// releases and TestWorkflowMatchesTheList fails when the workflow and the Go
// list disagree. Verified has nothing. It means a person ran the release on a
// development machine, and no test can prove that.
//
// What a test can prove is that the two lists agree, and that is the failure
// that actually happens. Oracle 18c spent months connecting to the container
// root while every other release connected to a pluggable database, which
// nothing noticed because nothing compared the Go list to the document.
//
// So this checks one direction, which is the one that goes stale: a release
// the Go list calls Verified has to be named in docs/COVERAGE.md. Adding one
// and not writing it down fails here. See D64.
func TestEveryVerifiedReleaseIsDocumented(t *testing.T) {
	t.Parallel()
	body, err := os.ReadFile(coverage)
	if err != nil {
		t.Fatalf("reading %s: %v", coverage, err)
	}
	// Spaces are dropped from both sides before looking. The document writes
	// SQL Server 2008 R2 the way a person says it and the Go list writes
	// 2008R2, and they mean the same release.
	text := strings.ReplaceAll(string(body), " ", "")

	var checked int
	for _, s := range container.All() {
		if s.Tier != container.Verified {
			continue
		}
		checked++
		// The major is what a person calls it, such as 19c for 19.3.0, and
		// the release is the tag. Either spelling counts as naming it.
		if !named(text, s.Major) && !named(text, s.Release) {
			t.Errorf("%s is Verified and %s names neither %q nor %q",
				s.Name(), coverage, s.Major, s.Release)
		}
	}
	for _, vm := range container.WindowsVMs {
		if vm.Tier != container.Verified {
			continue
		}
		checked++
		if !named(text, vm.Release) {
			t.Errorf("%s is Verified and %s never names %q",
				vm.Name(), coverage, vm.Release)
		}
	}
	if checked == 0 {
		t.Error("no release is Verified, so this test is checking nothing." +
			" Either the tier is gone and this test should be too, or the" +
			" lists stopped being read.")
	}
	t.Logf("%d Verified releases, every one named in %s", checked, coverage)
}

// named reports whether the text names this release, with spaces already
// dropped from the text.
func named(text, release string) bool {
	if release == "" {
		return false
	}
	return strings.Contains(text, strings.ReplaceAll(release, " ", ""))
}
