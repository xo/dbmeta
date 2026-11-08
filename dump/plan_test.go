package dump_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/xo/dbmeta"
	"github.com/xo/dbmeta/dump"
)

// mysqlSnapshot is a MySQL schema with the columns that the plan has to decide.
func mysqlSnapshot() *dump.Snapshot {
	col := func(name, raw string) dump.Column {
		typ, err := dump.ParseType(dbmeta.MySQL, raw)
		if err != nil {
			panic(err)
		}
		return dump.Column{Name: name, Type: typ, Nullable: true}
	}
	return &dump.Snapshot{
		Version: dump.SnapshotVersion, Dialect: "mysql",
		Schemas: []dump.Schema{{
			Name: "shop",
			Tables: []dump.Table{{
				Name: "item",
				Columns: []dump.Column{
					col("id", "int"), col("ok", "tinyint(1)"), col("stamp", "timestamp"), col("tags", "set('a','b')"),
					col("where", "point"), col("name", "varchar(20)"),
				},
				Constraints: []dump.Constraint{
					{Name: "PRIMARY", Kind: "primary key", Columns: []string{"id"}},
					{Name: "chk", Kind: "check", Definition: "(`id` > 0)"},
				},
			}},
			Views: []dump.View{{Name: "v", Definition: "select 1"}},
		}},
	}
}

func TestAPlanListsEveryColumnWithNoAllowedLevel(t *testing.T) {
	t.Parallel()
	_, err := dump.MakePlan(mysqlSnapshot(), dbmeta.PostgreSQL, dump.Policy{})
	var pe *dump.PlanError
	if !errors.As(err, &pe) || !errors.Is(err, dump.ErrNoEquivalent) {
		t.Fatalf("got %v, want a PlanError that wraps ErrNoEquivalent", err)
	}
	var names []string
	for _, f := range pe.Failures {
		names = append(names, f.Name)
	}
	got := strings.Join(names, " ")
	// The timestamp is Lossy in PostgreSQL? It is not: an instant is exact. The
	// set and the point have no type, and both are named at once.
	for _, want := range []string{"shop.item.tags", "shop.item.where"} {
		if !strings.Contains(got, want) {
			t.Errorf("the failures %q do not name %s", got, want)
		}
	}
	if strings.Contains(got, "shop.item.id") || strings.Contains(got, "shop.item.ok") {
		t.Errorf("the failures %q name a column that has an exact type", got)
	}
}

func TestALevelAllowsOnlyItself(t *testing.T) {
	t.Parallel()
	// Lossy does not allow Text, and Text does not allow Lossy.
	lossy := dump.Policy{Default: dump.Allow{Lossy: true}}
	if _, err := dump.MakePlan(mysqlSnapshot(), dbmeta.PostgreSQL, lossy); err == nil {
		t.Error("Lossy alone allowed a column that has no type")
	}
	text := dump.Policy{Default: dump.Allow{Text: true}}
	plan, err := dump.MakePlan(mysqlSnapshot(), dbmeta.PostgreSQL, text)
	if err != nil {
		t.Fatalf("Text: %v", err)
	}
	levels := map[string]dump.Level{}
	for _, c := range plan.Columns {
		levels[c.Name] = c.Level
	}
	if levels["shop.item.tags"] != dump.Text || levels["shop.item.where"] != dump.Text {
		t.Errorf("the set and the point got %v and %v, want Text", levels["shop.item.tags"], levels["shop.item.where"])
	}
	if levels["shop.item.id"] != dump.Exact || levels["shop.item.stamp"] != dump.Exact {
		t.Errorf("an exact column got %v and %v", levels["shop.item.id"], levels["shop.item.stamp"])
	}
}

func TestTheMostSpecificScopeWins(t *testing.T) {
	t.Parallel()
	p := dump.Policy{
		Default: dump.Allow{Text: true},
		Kinds:   map[dump.Kind]dump.Allow{dump.Geometry: {}},
	}
	// The kind says no, and it beats the default that says Text.
	if _, err := dump.MakePlan(mysqlSnapshot(), dbmeta.PostgreSQL, p); err == nil {
		t.Fatal("a kind that allows nothing did not beat a default that allows Text")
	}
	// The column says yes, and it beats the kind.
	p.Columns = map[string]dump.Allow{"shop.item.where": {Text: true}}
	if _, err := dump.MakePlan(mysqlSnapshot(), dbmeta.PostgreSQL, p); err != nil {
		t.Errorf("a column that allows Text did not beat the kind: %v", err)
	}
}

func TestAPlanListsWhatIsNotTranslated(t *testing.T) {
	t.Parallel()
	plan, err := dump.MakePlan(mysqlSnapshot(), dbmeta.PostgreSQL, dump.Policy{Default: dump.Allow{Text: true}})
	if err != nil {
		t.Fatal(err)
	}
	kinds := map[string]bool{}
	for _, o := range plan.Untranslated {
		kinds[o.Kind+" "+o.Name] = true
	}
	for _, want := range []string{"check shop.item.chk", "view shop.v"} {
		if !kinds[want] {
			t.Errorf("the plan does not list %s as untranslated: %v", want, plan.Untranslated)
		}
	}
	same, err := dump.MakePlan(mysqlSnapshot(), dbmeta.MySQL, dump.Policy{})
	if err != nil || len(same.Untranslated) != 0 {
		t.Errorf("the same dialect gave %v and %v, want a plan with nothing untranslated", same.Untranslated, err)
	}
}

func TestACastReadsAColumnAsAnotherKind(t *testing.T) {
	t.Parallel()
	s := mysqlSnapshot()
	s.Schemas[0].Tables[0].Columns = append(s.Schemas[0].Tables[0].Columns, func() dump.Column {
		typ, _ := dump.ParseType(dbmeta.MySQL, "char(36)")
		return dump.Column{Name: "ref", Type: typ}
	}())
	p := dump.Policy{Default: dump.Allow{Text: true}, Casts: []dump.Cast{{Column: "%.ref", Type: "char(36)", Kind: dump.UUID}}}
	plan, err := dump.MakePlan(s, dbmeta.PostgreSQL, p)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range plan.Columns {
		if c.Name == "shop.item.ref" && (c.SQL != "uuid" || c.Level != dump.Exact) {
			t.Errorf("the cast column is %q at %s, want uuid at exact", c.SQL, c.Level)
		}
	}
}

func TestABooleanByConventionGetsACheck(t *testing.T) {
	t.Parallel()
	plan, err := dump.MakePlan(mysqlSnapshot(), dbmeta.PostgreSQL, dump.Policy{Default: dump.Allow{Text: true}})
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range plan.Columns {
		switch c.Name {
		case "shop.item.ok":
			if !strings.Contains(c.Check, "`ok` NOT IN (0, 1)") || !strings.Contains(c.Check, "`item`") {
				t.Errorf("the check of ok is %q", c.Check)
			}
		case "shop.item.id":
			if c.Check != "" {
				t.Errorf("the check of id is %q, want none", c.Check)
			}
		}
	}
}

func TestADumpToTheSameDialectKeepsTheTypeAsRead(t *testing.T) {
	t.Parallel()
	plan, err := dump.MakePlan(mysqlSnapshot(), dbmeta.MySQL, dump.Policy{})
	if err != nil {
		t.Fatalf("the same dialect needs no level: %v", err)
	}
	for _, c := range plan.Columns {
		if c.Level != dump.Exact || c.SQL != c.Source.Raw {
			t.Errorf("%s is %q at %s, want %q at exact", c.Name, c.SQL, c.Level, c.Source.Raw)
		}
	}
}
