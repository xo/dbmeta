package container_test

import (
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/xo/dbmeta"
	"github.com/xo/dbmeta/container"
)

// README.md writes the tiers twice: once as a table saying what each one
// means, and once per product as a table of which releases are in which. Both
// were written by hand and neither was read by anything.
//
// Both were wrong. The meaning table named three tiers and the list has had
// four since D42 added Nightly, and the PostgreSQL table called six Nightly
// releases Verified, which is the one tier that means CI does not run them.
// A reader deciding whether to rely on PostgreSQL 14 got the opposite of the
// truth.

const readme = "../README.md"

// tierRow matches a row of a per product release table, such as
// "| 9.6, 12, 15, 18 | Tested |".
var tierRow = regexp.MustCompile(`(?m)^\| ([^|]+?)\s*\| (Tested|Nightly|Verified)\s*\|`)

// heading matches a level two heading, which is how README.md says which
// product the table under it is about.
var heading = regexp.MustCompile(`(?m)^## (.+)$`)

func readFile(t *testing.T, path string) string {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	return string(body)
}

func readmeText(t *testing.T) string {
	t.Helper()
	return readFile(t, readme)
}

// TestTheReadmeNamesEveryTier checks that the table explaining the tiers
// names every tier the list can hold.
//
// Archived is not a container.Tier and is not checked here. An archived
// release is one the list does not name at all, which is why it has no value.
func TestTheReadmeNamesEveryTier(t *testing.T) {
	t.Parallel()
	body := readmeText(t)
	for _, tier := range []container.Tier{container.Tested, container.Nightly, container.Verified} {
		name := strings.ToUpper(string(tier)[:1]) + string(tier)[1:]
		if !strings.Contains(body, "| "+name+" ") {
			t.Errorf("README.md has no row for the %s tier, and the list can hold one", name)
		}
	}
}

// TestTheReadmeTierTablesMatchTheList checks each per product table of
// releases against the releases this package names for that product.
func TestTheReadmeTierTablesMatchTheList(t *testing.T) {
	t.Parallel()
	dialects := map[string]dbmeta.Dialect{
		"PostgreSQL": dbmeta.PostgreSQL,
		"SQL Server": dbmeta.SQLServer,
	}
	body := readmeText(t)
	var checked int
	for _, m := range tierRow.FindAllStringSubmatchIndex(body, -1) {
		releases, tier := body[m[2]:m[3]], container.Tier(strings.ToLower(body[m[4]:m[5]]))
		dialect, ok := dialects[sectionOf(body, m[0])]
		if !ok {
			// A table under a heading that is not a product, such as the one
			// saying what each tier means.
			continue
		}
		checked++
		got := majorsAtTier(dialect, tier)
		want := strings.Split(releases, ", ")
		slices.Sort(got)
		slices.Sort(want)
		if !slices.Equal(got, want) {
			t.Errorf("README.md says %s is %v at the %s tier and the list says %v",
				sectionOf(body, m[0]), want, tier, got)
		}
	}
	if checked == 0 {
		t.Error("README.md has no per product tier table any more, so this guards nothing")
	}
}

// sectionOf returns the level two heading above an offset.
func sectionOf(body string, at int) string {
	var out string
	for _, m := range heading.FindAllStringSubmatchIndex(body, -1) {
		if m[0] > at {
			break
		}
		out = body[m[2]:m[3]]
	}
	return out
}

// majorsAtTier returns the releases of one product at one tier, as a person
// writes them.
func majorsAtTier(d dbmeta.Dialect, tier container.Tier) []string {
	var out []string
	for _, s := range container.ForDialect(d) {
		if s.Tier == tier {
			out = append(out, s.Major)
		}
	}
	return out
}

// TestEveryProductIsEvaluated checks that every product in the list has a row
// in the table of evaluated databases in docs/EVALUATION.md.
//
// Step 2 of docs/DIALECT.md sends a new database through that procedure and
// the table is where the result goes. It held six rows while the list held
// twelve products, and said in its own words that nothing was unevaluated.
//
// The row is matched by name with the spaces and punctuation removed, so
// "SAP HANA" answers for hana and "Apache Hive" for hive, and no second map
// of display names has to be kept in step with the first.
func TestEveryProductIsEvaluated(t *testing.T) {
	t.Parallel()
	var rows []string
	for _, m := range regexp.MustCompile(`(?m)^\| ([A-Za-z0-9 ]+?) \| `).
		FindAllStringSubmatch(readFile(t, "../docs/EVALUATION.md"), -1) {
		rows = append(rows, strings.ToLower(strings.ReplaceAll(m[1], " ", "")))
	}
	if len(rows) == 0 {
		t.Fatal("docs/EVALUATION.md has no table of evaluated databases")
	}
	for _, s := range container.All() {
		if !slices.ContainsFunc(rows, func(row string) bool {
			return strings.Contains(row, s.Product)
		}) {
			t.Errorf("docs/EVALUATION.md has no row for %s. Every product goes"+
				" through the procedure before it gets a container entry.", s.Product)
		}
	}
}

// TestEveryTierHasAName exists so that adding a tier to the list is not
// silently absent from the documentation. It fails naming the tier.
func TestEveryTierHasAName(t *testing.T) {
	t.Parallel()
	seen := map[container.Tier]bool{}
	for _, s := range container.All() {
		seen[s.Tier] = true
	}
	known := []container.Tier{container.Tested, container.Nightly, container.Verified}
	for tier := range seen {
		if !slices.Contains(known, tier) {
			t.Errorf("the list holds the tier %q, which this test does not know about."+
				" Add it here and to the table in README.md.", tier)
		}
	}
}
