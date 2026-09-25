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
// tool/servers for the list as JSON and expands it into a matrix, so it names
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
	for _, tier := range []container.Tier{container.Tested, container.Nightly} {
		want := "./tool/servers --json " + string(tier)
		if !strings.Contains(text, want) {
			t.Errorf("the workflow never runs %q, so the %s tier is not read from"+
				" container.All and can drift from it", want, tier)
		}
	}
	if !strings.Contains(text, "fromJSON(needs.releases.outputs.tested)") {
		t.Error("the workflow does not expand the tested list into a matrix")
	}
	if !strings.Contains(text, "fromJSON(needs.releases.outputs.nightly)") {
		t.Error("the workflow does not expand the nightly list into a matrix")
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
// run.sh starts one. Those two are pinned in YAML and this is what stops them
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
