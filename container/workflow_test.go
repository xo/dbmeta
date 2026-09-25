package container_test

import (
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/xo/dbmeta/container"
)

// The CI workflow names its releases in YAML and this package names them in
// Go. Two copies drift, and the copy that drifts quietly is the one that
// decides what a release was tested against. This fails when they disagree.
//
// It reads the YAML with a line scan rather than a parser, because the root
// module depends on the standard library and dburl and on nothing else, and
// the lines it reads are one flat list each.
const workflow = "../.github/workflows/test.yml"

func TestWorkflowMatchesTheList(t *testing.T) {
	t.Parallel()
	body, err := os.ReadFile(workflow)
	if err != nil {
		t.Fatalf("reading the workflow: %v", err)
	}
	text := string(body)

	for _, c := range []struct {
		key     string
		servers []container.Server
		tier    container.Tier
		// tagged says the list holds image:tag rather than a bare release.
		tagged bool
	}{
		{key: "postgres", servers: container.PostgreSQL, tier: container.Tested},
		{key: "mariadb", servers: container.MariaDB, tier: container.Tested},
		{key: "mysql", servers: container.MySQL, tier: container.Tested},
		{key: "sqlserver", servers: container.SQLServer, tier: container.Tested},
		{key: "image", servers: slices.Concat(container.MariaDB, container.MySQL),
			tier: container.Nightly, tagged: true},
	} {
		want := make([]string, 0, len(c.servers))
		for _, s := range c.servers {
			if s.Tier != c.tier {
				continue
			}
			if c.tagged {
				want = append(want, s.Ref())
			} else {
				want = append(want, s.Release)
			}
		}
		got, ok := matrixList(text, c.key)
		if !ok {
			t.Errorf("the workflow has no %q matrix", c.key)
			continue
		}
		if strings.Join(got, ",") != strings.Join(want, ",") {
			t.Errorf("the %s matrix and container.go disagree\n workflow %v\n go       %v\n"+
				"change container.go and then the workflow, in that order", c.key, got, want)
		}
	}

	// the nightly PostgreSQL job runs every release, tested and nightly alike
	got, ok := matrixList(text, "postgres")
	if !ok {
		t.Fatal("expected a postgres matrix")
	}
	all, ok := lastMatrixList(text, "postgres")
	if !ok || len(all) <= len(got) {
		t.Fatal("expected the nightly job to list more releases than the push job")
	}
	if strings.Join(all, ",") != strings.Join(container.Releases(container.PostgreSQL), ",") {
		t.Errorf("the nightly postgres matrix and container.go disagree\n workflow %v\n go       %v",
			all, container.Releases(container.PostgreSQL))
	}

	// every image the workflow names must be one this package knows
	for _, s := range container.All() {
		if !strings.Contains(text, s.Image+":") {
			t.Errorf("the workflow never names the %s image", s.Image)
		}
	}
}

// matrixList returns the first `key: ["a", "b"]` list in the workflow.
func matrixList(text, key string) ([]string, bool) {
	lists := allMatrixLists(text, key)
	if len(lists) == 0 {
		return nil, false
	}
	return lists[0], true
}

// lastMatrixList returns the last such list, which is the nightly one.
func lastMatrixList(text, key string) ([]string, bool) {
	lists := allMatrixLists(text, key)
	if len(lists) == 0 {
		return nil, false
	}
	return lists[len(lists)-1], true
}

func allMatrixLists(text, key string) [][]string {
	var out [][]string
	for line := range strings.SplitSeq(text, "\n") {
		line = strings.TrimSpace(line)
		rest, found := strings.CutPrefix(line, key+": [")
		if !found {
			continue
		}
		rest, found = strings.CutSuffix(rest, "]")
		if !found {
			continue
		}
		var items []string
		for item := range strings.SplitSeq(rest, ",") {
			items = append(items, strings.Trim(strings.TrimSpace(item), `"`))
		}
		out = append(out, items)
	}
	return out
}

// TestEveryServerIsUsable checks the parts a caller needs to start one.
func TestEveryServerIsUsable(t *testing.T) {
	t.Parallel()
	seen := make(map[string]bool)
	for _, s := range container.All() {
		if seen[s.Name()] {
			t.Errorf("%s is listed twice", s.Name())
		}
		seen[s.Name()] = true
		if s.Image == "" || s.Tag == "" || s.Port == 0 || len(s.Env) == 0 || len(s.Ready) == 0 {
			t.Errorf("%s is missing something it needs to start: %+v", s.Name(), s)
		}
		if got := s.DSN(1234); !strings.Contains(got, "1234") {
			t.Errorf("%s: expected the port in the connection string, got %s", s.Name(), got)
		}
		args := s.RunArgs(s.Name(), 1234)
		if !slices.Contains(args, s.Qualified()) {
			t.Errorf("%s: expected the qualified image in %v", s.Name(), args)
		}
		if !slices.Contains(args, "1234:"+itoa(s.Port)) {
			t.Errorf("%s: expected the published port in %v", s.Name(), args)
		}
	}
}

// TestHealthCmdSurvivesAnArgumentWithASpace is the guard for a fault that cost
// a whole matrix run.
//
// Every readiness command joined its arguments with a space, and every
// argument happened to have none, until SQL Server arrived with the query
// "SELECT 1". The receiving shell split it, sqlcmd got a command it could not
// run, and all four SQL Server releases reported "never became ready" while
// the servers were up and answering.
//
// A command is a list of arguments and a string is not, so anything that
// flattens one into the other has to say what it does with a space.
func TestHealthCmdSurvivesAnArgumentWithASpace(t *testing.T) {
	t.Parallel()
	var checked int
	for _, s := range container.All() {
		got := s.HealthCmd()
		for _, arg := range s.Ready {
			if !strings.Contains(arg, " ") {
				continue
			}
			checked++
			if !strings.Contains(got, "'"+arg+"'") {
				t.Errorf("%s: the argument %q has a space and is not quoted in %q",
					s.Name(), arg, got)
			}
		}
	}
	// The guard is worth nothing if no server exercises it, and one does.
	if checked == 0 {
		t.Error("no readiness command has an argument with a space any more. " +
			"Drop this test or find the one that does, because it is guarding nothing.")
	}
}

// TestReleasesAreOrdered checks that the numbers sort by value, which is the
// thing MySQL's move to a year based release broke for anything sorting text.
func TestReleasesAreOrdered(t *testing.T) {
	t.Parallel()
	if got := container.Releases(container.PostgreSQL); got[0] != "9.6" || got[len(got)-1] != "18" {
		t.Errorf("expected 9.6 first and 18 last, got %v", got)
	}
	// 9.7 sorts below 26.7, which a text sort gets backwards
	got := container.Releases(container.MySQL)
	if slices.Index(got, "9.7") > slices.Index(got, "26.7") {
		t.Errorf("expected 9.7 before 26.7, got %v", got)
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for ; n > 0; n /= 10 {
		b = append([]byte{byte('0' + n%10)}, b...)
	}
	return string(b)
}
