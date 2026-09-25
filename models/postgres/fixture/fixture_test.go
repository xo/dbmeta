package fixture_test

import (
	"strings"
	"testing"

	"github.com/xo/dbmeta"
	"github.com/xo/dbmeta/models/postgres/fixture"
)

func versions(s string) dbmeta.VersionSet {
	var v dbmeta.VersionSet
	v.Set("", dbmeta.ParseVersion(s))
	return v
}

// TestTriggerSyntaxFollowsTheRelease is the case that started this. Release 11
// takes EXECUTE FUNCTION and everything below it takes EXECUTE PROCEDURE.
// Writing the deprecated form everywhere would test syntax nobody writes.
func TestTriggerSyntaxFollowsTheRelease(t *testing.T) {
	t.Parallel()
	tests := map[string]string{
		"9.6.24": "EXECUTE PROCEDURE",
		"10.23":  "EXECUTE PROCEDURE",
		"11.22":  "EXECUTE FUNCTION",
		"18.6":   "EXECUTE FUNCTION",
	}
	for ver, want := range tests {
		steps, err := fixture.Everything.ResolveSetup(versions(ver))
		if err != nil {
			t.Fatalf("%s: expected no error, got: %v", ver, err)
		}
		var found bool
		for _, s := range steps {
			if s.Name != "trigger" {
				continue
			}
			found = true
			if !strings.Contains(s.Query, want) {
				t.Errorf("%s: expected %q, got:\n%s", ver, want, s.Query)
			}
		}
		if !found {
			t.Errorf("%s: expected a trigger step", ver)
		}
	}
}

// TestOldReleasesSkipRatherThanFail checks the one way a fixture differs from
// a query. A query for an object the server does not have is refused. A step
// that creates one is skipped, because there is nothing to create.
func TestOldReleasesSkipRatherThanFail(t *testing.T) {
	t.Parallel()
	steps, err := fixture.Everything.ResolveSetup(versions("9.6.24"))
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	skipped := map[string]bool{}
	for _, s := range steps {
		if s.Skipped {
			skipped[s.Name] = true
			if s.Query != "" {
				t.Errorf("%s: a skipped step must have no SQL", s.Name)
			}
			if s.Reason == "" {
				t.Errorf("%s: a skipped step must say why", s.Name)
			}
		}
	}
	for _, name := range []string{
		"identity column", "generated column", "partitioned table",
		"partition", "publication", "extended statistics",
	} {
		if !skipped[name] {
			t.Errorf("expected %q to be skipped on 9.6", name)
		}
	}
	// and the newest release skips nothing
	steps, err = fixture.Everything.ResolveSetup(versions("18.6"))
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	for _, s := range steps {
		if s.Skipped {
			t.Errorf("expected %q to run on 18.6", s.Name)
		}
	}
}

// TestEveryStepResolvesOrSkips checks that no release produces a step that is
// neither runnable nor skipped.
func TestEveryStepResolvesOrSkips(t *testing.T) {
	t.Parallel()
	for _, ver := range []string{"9.6.24", "10.23", "11.22", "12.18", "13.15", "14.12", "15.7", "16.2", "17.4", "18.6"} {
		for _, phase := range []struct {
			name  string
			steps func(dbmeta.VersionSet) ([]fixture.Result, error)
		}{
			{"setup", fixture.Everything.ResolveSetup},
			{"teardown", fixture.Everything.ResolveTeardown},
		} {
			steps, err := phase.steps(versions(ver))
			if err != nil {
				t.Fatalf("%s %s: expected no error, got: %v", ver, phase.name, err)
			}
			if len(steps) == 0 {
				t.Errorf("%s %s: expected steps", ver, phase.name)
			}
			for _, s := range steps {
				if s.Skipped == (s.Query != "") {
					t.Errorf("%s %s: %q is neither runnable nor skipped", ver, phase.name, s.Name)
				}
			}
		}
	}
}

// TestSchemaIsNamed checks that everything the fixture builds lives in the one
// schema it declares, so a caller can read that schema and see all of it, and
// drop it to remove all of it.
func TestSchemaIsNamed(t *testing.T) {
	t.Parallel()
	steps, err := fixture.Everything.ResolveSetup(versions("18.6"))
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range steps {
		// a publication is a database wide object and cannot be schema
		// qualified, so it carries the schema in its name instead
		if s.Name == "publication" {
			continue
		}
		if !strings.Contains(s.Query, fixture.Everything.Schema) {
			t.Errorf("%q does not name the fixture schema:\n%s", s.Name, s.Query)
		}
	}
}
