package container_test

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/xo/dbmeta/container"
)

// The CI workflow used to name every release and every image in YAML, beside
// the Go list here, and a test compared the two so they could not drift.
//
// They cannot drift now, because there is only one copy. The workflow asks
// dbrun for the list as JSON and expands it into a matrix, so it names
// no release and no image of its own. These tests hold that property in
// place, which is a smaller thing to check than two lists agreeing and a
// stronger one to have. See D69.
//
// They read the YAML with a line scan rather than a parser, because the root
// module depends on the standard library and on nothing else.
const workflow = "../.github/workflows/test.yml"

// imageLine matches an image a service block names.
var imageLine = regexp.MustCompile(`(?m)^\s*image:\s*(\S+)\s*$`)

func workflowText(t *testing.T) string {
	t.Helper()
	body, err := os.ReadFile(workflow)
	if err != nil {
		t.Fatalf("reading the workflow: %v", err)
	}
	return string(body)
}

// TestWorkflowReadsTheList checks that the workflow builds its matrix from
// this package rather than from a list of its own.
func TestWorkflowReadsTheList(t *testing.T) {
	t.Parallel()
	text := workflowText(t)
	// The command dbrun is invoked by is matched from the subcommand on,
	// because the workflow runs a binary it built earlier rather than
	// `go run ./cmd/dbrun`, and what matters is that dbrun answers the
	// question rather than where its binary sits. See D82.
	for _, tier := range []container.Tier{container.Tested, container.Nightly} {
		want := "dbrun\" list --json --names " + string(tier)
		if !strings.Contains(text, want) {
			t.Errorf("the workflow never runs dbrun %q, so the %s tier is not read"+
				" from container.All and can drift from it", want, tier)
		}
	}
	if !strings.Contains(text, "fromJSON(needs.releases.outputs.tested)") {
		t.Error("the workflow does not expand the tested list into a matrix")
	}
	if !strings.Contains(text, "fromJSON(needs.releases.outputs.nightly)") {
		t.Error("the workflow does not expand the nightly list into a matrix")
	}
}

// TestTheMatrixJobsCompileNothing checks that a job in either release matrix
// runs the binaries the build job made, rather than compiling its own.
//
// This is the whole of D82 and it is invisible from a passing run: a job that
// compiles still gives the right answer, it just costs ninety seconds to do
// it, and there are more than twenty of them. Measured before the change, one
// nightly job spent 95 seconds of 103 compiling and 1.4 testing.
//
// The unit job is not covered and must not be. It compiles on purpose, which
// is what it is for.
func TestTheMatrixJobsCompileNothing(t *testing.T) {
	t.Parallel()
	text := workflowText(t)
	for _, job := range []string{"server:", "nightly:"} {
		_, rest, ok := strings.Cut(text, "\n  "+job+"\n")
		if !ok {
			t.Errorf("the workflow has no %s job", strings.TrimSuffix(job, ":"))
			continue
		}
		// Everything up to the next job, which starts at the same indent.
		body, _, _ := strings.Cut(rest, "\n  compare:")
		if i := strings.Index(body, "\n  nightly:"); job == "server:" && i >= 0 {
			body = body[:i]
		}
		name := strings.TrimSuffix(job, ":")
		if strings.Contains(body, "go run ") || strings.Contains(body, "go test ") {
			t.Errorf("the %s job compiles. It runs the binaries the build job"+
				" uploaded, because compiling the test module costs about ninety"+
				" seconds and every job in the matrix would pay it. See D82.", name)
		}
		if !strings.Contains(body, "DBMETA_TEST_BINARY") {
			t.Errorf("the %s job does not set DBMETA_TEST_BINARY, so dbrun will"+
				" fall back to `go test` and compile the tests itself. See D82.", name)
		}
		if !strings.Contains(body, "actions/download-artifact") {
			t.Errorf("the %s job does not download the built binaries", name)
		}
	}
	if !strings.Contains(text, "actions/upload-artifact") {
		t.Error("nothing uploads the built binaries, so the matrix jobs have" +
			" nothing to download")
	}
}

// TestWorkflowImagesAreQualified checks that every image the workflow does
// name carries its registry.
//
// An unqualified name resolves against whatever the runner's search list
// happens to be, which is podman's unqualified-search-registries locally and
// Docker Hub on the runner. The same YAML then means two things. Every image
// in container.All is written out in full and the few left in the workflow
// have to match.
func TestWorkflowImagesAreQualified(t *testing.T) {
	t.Parallel()
	text := workflowText(t)
	found := imageLine.FindAllStringSubmatch(text, -1)
	if len(found) == 0 {
		t.Skip("the workflow names no image, which is the direction of travel")
	}
	for _, m := range found {
		image := m[1]
		if strings.Contains(image, "${{") {
			continue
		}
		host, _, ok := strings.Cut(image, "/")
		if !ok || !strings.Contains(host, ".") {
			t.Errorf("%s is not fully qualified: an image here needs its registry,"+
				" because an unqualified name resolves against the runner's search"+
				" list and means one thing locally and another in CI", image)
		}
	}
}

// TestWorkflowImagesAreInTheList checks that an image the workflow still names
// is one this package knows about.
//
// Only the comparison job names any, because it needs two servers at once and
// dbrun starts one. Those two are pinned in YAML and this is what stops them
// drifting from the releases everything else tests.
func TestWorkflowImagesAreInTheList(t *testing.T) {
	t.Parallel()
	text := workflowText(t)
	known := map[string]bool{}
	for _, s := range container.All() {
		known[s.Image+":"+s.Tag] = true
	}
	for _, m := range imageLine.FindAllStringSubmatch(text, -1) {
		image := m[1]
		if strings.Contains(image, "${{") {
			continue
		}
		if !known[image] {
			t.Errorf("the workflow names %s and container.All does not have it,"+
				" so the two disagree about what is tested", image)
		}
	}
}
