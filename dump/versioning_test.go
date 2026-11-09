package dump_test

import (
	"strings"
	"testing"

	"github.com/xo/dbmeta"
	"github.com/xo/dbmeta/dump"
	_ "github.com/xo/dbmeta/models/sqlserver"
)

// versionedSnapshot is a SQL Server schema with a system versioned table, its
// history table, and a table with a spatial column and a column that the server
// numbers.
func versionedSnapshot() *dump.Snapshot {
	col := func(name, raw, generated string) dump.Column {
		typ, err := dump.ParseType(dbmeta.SQLServer, raw)
		if err != nil {
			panic(err)
		}
		typ.Raw = raw
		return dump.Column{Name: name, Type: typ, Generated: generated}
	}
	period := []dump.Column{
		col("valid_from", "datetime2(6)", "row start"),
		col("valid_to", "datetime2(6)", "row end"),
	}
	return &dump.Snapshot{
		Version: dump.SnapshotVersion, Dialect: "sqlserver",
		Schemas: []dump.Schema{{
			Name: "hr",
			Tables: []dump.Table{
				{
					Name:       "price",
					Versioning: &dump.Versioning{Schema: "hr", Table: "price_history", Start: "valid_from", End: "valid_to"},
					Columns:    append([]dump.Column{col("id", "int", "")}, period...),
					Constraints: []dump.Constraint{{Name: "pk_price", Kind: "primary key", Columns: []string{"id"},
						Definition: "PRIMARY KEY (id)"}},
				},
				{
					Name:    "price_history",
					Columns: append([]dump.Column{col("id", "int", "")}, col("valid_from", "datetime2(6)", ""), col("valid_to", "datetime2(6)", "")),
				},
				{
					Name: "place",
					Columns: []dump.Column{
						col("id", "int", ""), col("spot", "geography", ""), col("node", "hierarchyid", ""),
						col("version", "timestamp", "rowversion"),
					},
					Constraints: []dump.Constraint{{Name: "pk_place", Kind: "primary key", Columns: []string{"id"},
						Definition: "PRIMARY KEY (id)"}},
				},
			},
		}},
	}
}

func TestASystemVersionedTableIsWrittenWithItsPeriodAndTurnedOnAfterTheRows(t *testing.T) {
	t.Parallel()
	var pre, data, post []string
	for _, st := range scriptText(t, versionedSnapshot(), dbmeta.SQLServer) {
		switch st.Stage {
		case dump.Pre:
			pre = append(pre, st.SQL)
		case dump.Data:
			data = append(data, st.SQL)
		case dump.Post:
			post = append(post, st.SQL)
		}
	}
	create := strings.Join(pre, "\n")
	for _, want := range []string{
		"[valid_from] datetime2(6) GENERATED ALWAYS AS ROW START NOT NULL",
		"[valid_to] datetime2(6) GENERATED ALWAYS AS ROW END NOT NULL",
		"PERIOD FOR SYSTEM_TIME ([valid_from], [valid_to])",
	} {
		if !strings.Contains(create, want) {
			t.Errorf("the script has no %q:\n%s", want, create)
		}
	}
	if strings.Contains(create, "SYSTEM_VERSIONING") {
		t.Errorf("a table that is created with versioning on refuses the rows of its history:\n%s", create)
	}
	if got := strings.Join(data, "\n"); !strings.Contains(got, "ALTER TABLE [hr].[price] DROP PERIOD FOR SYSTEM_TIME") {
		t.Errorf("the period is not dropped before the rows:\n%s", got)
	}
	got := strings.Join(post, "\n")
	add := strings.Index(got, "ALTER TABLE [hr].[price] ADD PERIOD FOR SYSTEM_TIME ([valid_from], [valid_to])")
	on := strings.Index(got, "ALTER TABLE [hr].[price] SET (SYSTEM_VERSIONING = ON (HISTORY_TABLE = [hr].[price_history]))")
	if add < 0 || on < add {
		t.Errorf("versioning is not turned on after the period is made again:\n%s", got)
	}
}

func TestAnotherProductGetsPlainTablesForAVersionedTable(t *testing.T) {
	t.Parallel()
	var created []string
	var noted bool
	snap := versionedSnapshot()
	// Another product has no type for the spatial column or the rowversion.
	snap.Schemas[0].Tables = snap.Schemas[0].Tables[:2]
	for _, st := range scriptText(t, snap, dbmeta.PostgreSQL) {
		if st.Kind == "table" {
			created = append(created, st.SQL)
			noted = noted || strings.Contains(st.Note, "keeps its history")
		}
		if strings.Contains(st.SQL, "PERIOD") || strings.Contains(st.SQL, "SYSTEM_VERSIONING") {
			t.Errorf("a product with no versioning got %s", st.SQL)
		}
	}
	if !noted {
		t.Error("the script does not say that the history is lost")
	}
	if !strings.Contains(strings.Join(created, "\n"), `"price_history"`) {
		t.Errorf("the history table is not written:\n%s", strings.Join(created, "\n"))
	}
}

func TestALoaderGivesAVersionedTableItsPeriodAndLeavesOutARowversion(t *testing.T) {
	t.Parallel()
	snap := versionedSnapshot()
	l, err := dump.NewLoader(snap, "hr", "price", dbmeta.SQLServer, dump.Policy{})
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(l.Columns(), ","); got != "id,valid_from,valid_to" {
		t.Errorf("the columns are %s, want the period columns with the rest", got)
	}
	l, err = dump.NewLoader(snap, "hr", "place", dbmeta.SQLServer, dump.Policy{})
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(l.Columns(), ","); got != "id,spot,node" {
		t.Errorf("the columns are %s, want the rowversion column left out", got)
	}
}

func TestALoaderCastsTheBinaryFormOfAClrType(t *testing.T) {
	t.Parallel()
	l, err := dump.NewLoader(versionedSnapshot(), "hr", "place", dbmeta.SQLServer, dump.Policy{})
	if err != nil {
		t.Fatal(err)
	}
	stmt, err := l.Insert([][]any{{int64(1), []byte{0xe6, 0x10, 0, 0, 1}, []byte{0x5b, 0x40}}, {int64(2), nil, nil}})
	if err != nil {
		t.Fatal(err)
	}
	want := "INSERT INTO [hr].[place] ([id], [spot], [node]) VALUES " +
		"(1, CAST(0xe610000001 AS geography), CAST(0x5b40 AS hierarchyid)), (2, NULL, NULL)"
	if stmt != want {
		t.Errorf("got\n%s\nwant\n%s", stmt, want)
	}
	want = "INSERT INTO [hr].[place] ([id], [spot], [node]) VALUES (@p1, CAST(@p2 AS geography), CAST(@p3 AS hierarchyid))"
	if got := l.InsertArgs(1); got != want {
		t.Errorf("InsertArgs gave\n%s\nwant\n%s", got, want)
	}
}

// typesSnapshot is a SQL Server schema with an alias type, a table type, a function
// that reads a table, and a table that uses the alias type.
func typesSnapshot() *dump.Snapshot {
	col := func(name, raw string) dump.Column {
		typ, err := dump.ParseType(dbmeta.SQLServer, raw)
		if err != nil {
			panic(err)
		}
		return dump.Column{Name: name, Type: typ}
	}
	return &dump.Snapshot{
		Version: dump.SnapshotVersion, Dialect: "sqlserver",
		Schemas: []dump.Schema{{
			Name:    "hr",
			Domains: []dump.Domain{{Name: "email", Type: "nvarchar(254)", NotNull: true}},
			TableTypes: []dump.Table{{
				Name:        "code_list",
				Columns:     []dump.Column{col("code", "char(4)")},
				Constraints: []dump.Constraint{{Name: "PK__x", Kind: "primary key", Columns: []string{"code"}}},
			}},
			Tables: []dump.Table{{
				Name:    "staff",
				Columns: []dump.Column{col("id", "int"), col("email", "[hr].[email]")},
			}},
			Routines: []dump.Routine{
				{Name: "staff_in", Kind: "function", Definition: "create function hr.staff_in (@codes hr.code_list readonly) returns table as return select * from hr.staff"},
			},
		}},
	}
}

func TestAnAliasTypeAndATableTypeComeBeforeTheTablesAndARoutineAfterThem(t *testing.T) {
	t.Parallel()
	var order []string
	for _, st := range scriptText(t, typesSnapshot(), dbmeta.SQLServer) {
		switch st.Kind {
		case "type", "table", "routine":
			order = append(order, st.Kind+" "+st.Name)
		}
		if st.Kind == "type" && st.Name == "hr.email" && st.SQL != "CREATE TYPE [hr].[email] FROM nvarchar(254) NOT NULL" {
			t.Errorf("the alias type is %s", st.SQL)
		}
		if st.Kind == "type" && st.Name == "hr.code_list" && !strings.Contains(st.SQL, "CREATE TYPE [hr].[code_list] AS TABLE (\n  [code] char(4) NOT NULL,\n  PRIMARY KEY ([code])\n)") {
			t.Errorf("the table type is %s", st.SQL)
		}
	}
	want := "type hr.email,type hr.code_list,table hr.staff,routine hr.staff_in"
	if got := strings.Join(order, ","); got != want {
		t.Errorf("the order is %s, want %s", got, want)
	}
}
