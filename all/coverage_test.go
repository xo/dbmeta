package all_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/xo/dbmeta"
	_ "github.com/xo/dbmeta/all"
)

// How many questions each model answers is written in prose, in a table in
// README.md, in a table in docs/COVERAGE.md and in every model's package
// comment. That is about thirty numbers and nothing derived them.
//
// Adding the 55th kind proved the point. One test caught the change, the one
// in models/sqlserver that happened to state its own count, and every other
// number went stale at once and had to be found by grep. The same class of
// drift had already been caught once in the decision count.
//
// This derives every number from the models and fails naming the file, so a
// count cannot be wrong and cannot be missed.
//
// It lives here because this package is the one that registers every model.
// The shared information_schema model is not registered by importing all, so
// its row is checked by TestTheSharedModelAnswersWhatItSays in its own
// package, where its profiles already exist.

// sharedModel is the one row in each table this package cannot count.
const sharedModel = "informationschema"

// answers returns how many questions each product answers, by asking.
func answers(t *testing.T) map[string]int {
	t.Helper()
	out := map[string]int{}
	for _, c := range []struct {
		name    string
		dialect dbmeta.Dialect
		// key is the product key a shared dialect gates on. See D44.
		key string
	}{
		{name: "postgres", dialect: dbmeta.PostgreSQL},
		{name: "mariadb", dialect: dbmeta.MySQL, key: "mariadb"},
		{name: "mysql", dialect: dbmeta.MySQL, key: "mysql"},
		{name: "sqlite3", dialect: dbmeta.SQLite3},
		{name: "duckdb", dialect: dbmeta.DuckDB},
		{name: "sqlserver", dialect: dbmeta.SQLServer},
		{name: "oracle", dialect: dbmeta.Oracle},
		{name: "cassandra", dialect: dbmeta.Cassandra},
		{name: "clickhouse", dialect: dbmeta.ClickHouse},
		{name: "trino", dialect: dbmeta.Trino},
		{name: "presto", dialect: dbmeta.Presto},
		{name: "firebird", dialect: dbmeta.Firebird},
	} {
		// The newest release of each, because a count is what the model can
		// do and not what an old server allows.
		var set dbmeta.VersionSet
		set.Set("", dbmeta.V(9999))
		if c.key != "" {
			set.Set(c.key, dbmeta.V(9999))
		}
		m, err := dbmeta.New(c.dialect, set)
		if err != nil {
			t.Fatalf("building the metadata for %s: %v", c.name, err)
		}
		for _, q := range dbmeta.Queries() {
			if q.Support(m) == dbmeta.Supported {
				out[c.name]++
			}
		}
	}
	return out
}

// TestTheCoverageTableIsRight checks the count table in docs/COVERAGE.md.
func TestTheCoverageTableIsRight(t *testing.T) {
	t.Parallel()
	got, total := answers(t), len(dbmeta.Queries())
	// | `models/postgres` | 55 | 55 | PostgreSQL 9.6 through 18 |
	row := regexp.MustCompile(`(?m)^\| ` + "`" + `models/(\w+)` + "`" + ` \| ([^|]+?) \| (\d+) \|`)
	body := read(t, filepath.Join("docs", "COVERAGE.md"))
	seen := map[string]bool{}
	for _, m := range row.FindAllStringSubmatch(body, -1) {
		model, stated, of := m[1], m[2], m[3]
		seen[model] = true
		if of != strconv.Itoa(total) {
			t.Errorf("docs/COVERAGE.md: the %s row is out of %s and there are %d kinds",
				model, of, total)
		}
		if model != sharedModel {
			checkCell(t, model, stated, got)
		}
	}
	for model := range got {
		if model == "mariadb" || model == "mysql" {
			model = "mysql"
		}
		if !seen[model] {
			t.Errorf("docs/COVERAGE.md has no row for models/%s", model)
		}
	}
	if !seen[sharedModel] {
		t.Errorf("docs/COVERAGE.md has no row for models/%s", sharedModel)
	}
}

// checkCell checks every number in a table cell against what the models
// answer. The mysql row states two, because one model serves two products.
func checkCell(t *testing.T, model, cell string, got map[string]int) {
	t.Helper()
	nums := regexp.MustCompile(`\d+`).FindAllString(cell, -1)
	var want []int
	switch model {
	case "mysql":
		want = []int{got["mariadb"], got["mysql"]}
	default:
		want = []int{got[model]}
	}
	if len(nums) != len(want) {
		t.Errorf("docs/COVERAGE.md: the %s row states %d numbers and %d were expected: %q",
			model, len(nums), len(want), cell)
		return
	}
	for i, n := range nums {
		if n != strconv.Itoa(want[i]) {
			t.Errorf("docs/COVERAGE.md: the %s row says %s and the model answers %d",
				model, n, want[i])
		}
	}
}

// TestEveryModelIsInTheVersionTable checks that each model's version query has
// been compared against the one usql runs for the same product.
//
// D38 makes that comparison part of adding a dialect, and this is what holds
// it. The comparison itself cannot be automated, because usql is not a
// dependency here and must not become one, so what is checked is that
// somebody wrote the row down.
//
// It exists because the comparison was skipped and the gap was invisible from
// the other end: docs/USQL.md compared the printed version lines and looked
// complete, while usql had no version query for Oracle at all.
func TestEveryModelIsInTheVersionTable(t *testing.T) {
	t.Parallel()
	body := read(t, filepath.Join("docs", "USQL.md"))
	_, table, ok := strings.Cut(body, "### The statements, compared")
	if !ok {
		t.Fatal("docs/USQL.md has no statements table. D38 requires one.")
	}
	table, _, _ = strings.Cut(table, "\n## ")
	for name := range answers(t) {
		// mariadb and mysql are one model and share a row.
		product := map[string]string{
			"postgres": "PostgreSQL", "mariadb": "MariaDB", "mysql": "MySQL",
			"sqlite3": "SQLite", "duckdb": "DuckDB", "sqlserver": "SQL Server",
			"oracle": "Oracle", "cassandra": "Cassandra",
			"clickhouse": "ClickHouse", "trino": "Trino", "presto": "Presto",
			"firebird": "Firebird",
		}[name]
		if product == "" {
			t.Errorf("%s has no product name here, so its version query cannot be"+
				" looked for. Add it. See D38.", name)
			continue
		}
		if !strings.Contains(table, "| "+product+" ") {
			t.Errorf("docs/USQL.md: no row for %s in the statements table."+
				" Compare its version query against usql's and record it. See D38.",
				product)
		}
	}
}

// TestEveryModelSaysWhetherDbtplCanUseIt checks that each model has a verdict
// in docs/DBTPL.md, and not only a count.
//
// The count and the verdict are different questions and the second does not
// follow from the first. A database can answer most of the nine reads and
// still be useless to a generator, if it has no foreign key and so no
// relationship to follow. See D38 for the same rule about usql.
func TestEveryModelSaysWhetherDbtplCanUseIt(t *testing.T) {
	t.Parallel()
	body := read(t, filepath.Join("docs", "DBTPL.md"))
	_, table, ok := strings.Cut(body, "### Whether dbtpl could generate for each database")
	if !ok {
		t.Fatal("docs/DBTPL.md has no verdict table. Adding a dialect requires one.")
	}
	table, _, _ = strings.Cut(table, "\n## ")
	for name := range answers(t) {
		product := map[string]string{
			"postgres": "PostgreSQL", "mariadb": "MySQL and MariaDB",
			"mysql": "MySQL and MariaDB", "sqlite3": "SQLite", "duckdb": "DuckDB",
			"sqlserver": "SQL Server", "oracle": "Oracle", "cassandra": "Cassandra",
			"clickhouse": "ClickHouse", "trino": "Trino", "presto": "Presto",
			"firebird": "Firebird",
		}[name]
		if product == "" {
			t.Errorf("%s has no product name here, so its dbtpl verdict cannot be"+
				" looked for. Add it.", name)
			continue
		}
		if !strings.Contains(table, "| "+product+" |") {
			t.Errorf("docs/DBTPL.md: no verdict for %s. Say whether dbtpl could"+
				" generate from it and why.", product)
		}
	}
}

// TestTheReadmeTableIsRight checks the support table in README.md.
func TestTheReadmeTableIsRight(t *testing.T) {
	t.Parallel()
	got := answers(t)
	// | PostgreSQL | native             | 55      | Complete    |
	row := regexp.MustCompile(`(?m)^\| ([A-Za-z0-9 ]+?)\s*\| native\s*\| (\d+)\s*\|`)
	body := read(t, "README.md")
	names := map[string]string{
		"PostgreSQL": "postgres", "MariaDB": "mariadb", "MySQL": "mysql",
		"SQLite3": "sqlite3", "DuckDB": "duckdb", "SQL Server": "sqlserver",
		"Oracle": "oracle", "Cassandra": "cassandra", "ClickHouse": "clickhouse",
		"Trino": "trino", "Presto": "presto", "Firebird": "firebird",
	}
	var checked int
	for _, m := range row.FindAllStringSubmatch(body, -1) {
		model, ok := names[m[1]]
		if !ok {
			// a native model with no count here yet, such as a planned one
			continue
		}
		checked++
		if m[2] != strconv.Itoa(got[model]) {
			t.Errorf("README.md says %s answers %s and the model answers %d",
				m[1], m[2], got[model])
		}
	}
	if checked != len(got) {
		t.Errorf("README.md has %d native rows with a count and there are %d models",
			checked, len(got))
	}
}

// TestEveryPackageCommentStatesItsCount checks the number each model's own
// documentation quotes, which is the copy a reader of go doc sees.
//
// Three models write the count as a word and one writes digits, so the words
// are turned into digits before looking.
func TestEveryPackageCommentStatesItsCount(t *testing.T) {
	t.Parallel()
	got, total := answers(t), len(dbmeta.Queries())
	for _, pkg := range []string{"mysql", "sqlite3", "duckdb", "sqlserver", "oracle"} {
		doc, _, _ := strings.Cut(read(t, filepath.Join("models", pkg, pkg+".go")), "\npackage ")
		doc = spellOut(doc)
		if !strings.Contains(doc, "of the "+strconv.Itoa(total)) {
			t.Errorf("models/%s: the package comment does not say how many of the %d it answers",
				pkg, total)
			continue
		}
		// One model serves two products and its comment names both.
		want := []int{got[pkg]}
		if pkg == "mysql" {
			want = []int{got["mariadb"], got["mysql"]}
		}
		for _, n := range want {
			if !strings.Contains(doc, strconv.Itoa(n)) {
				t.Errorf("models/%s: the package comment never says %d, which is what it answers",
					pkg, n)
			}
		}
	}
}

// spellOut rewrites a number written as a word into digits, so that a comment
// reading "Thirty two of the 55" can be checked against 32.
var numberWords = map[string]int{
	"fourteen": 14, "fifteen": 15, "sixteen": 16, "seventeen": 17,
	"eighteen": 18, "nineteen": 19, "twenty": 20, "thirty": 30,
}

func spellOut(doc string) string {
	for word, n := range numberWords {
		for _, unit := range []struct {
			suffix string
			add    int
		}{
			{" one", 1}, {" two", 2}, {" three", 3}, {" four", 4}, {" five", 5},
			{" six", 6}, {" seven", 7}, {" eight", 8}, {" nine", 9}, {"", 0},
		} {
			// the same word capitalised, because a comment starts a
			// sentence with it. These are ASCII words from the map above.
			upper := strings.ToUpper(word[:1]) + word[1:]
			for _, w := range []string{word + unit.suffix, upper + unit.suffix} {
				doc = strings.ReplaceAll(doc, w+" of the ", strconv.Itoa(n+unit.add)+" of the ")
			}
		}
	}
	return doc
}

func read(t *testing.T, path string) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join("..", path))
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	return string(body)
}
