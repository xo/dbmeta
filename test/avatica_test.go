package test

import (
	"context"
	"database/sql"
	"errors"
	"net/url"
	"os"
	"strings"
	"testing"

	_ "github.com/xo/dbimp/avatica"

	"github.com/xo/dbmeta"
	"github.com/xo/dbmeta/container"
	_ "github.com/xo/dbmeta/models/avatica"
	avfixture "github.com/xo/dbmeta/models/avatica/fixture"
)

// openAvatica returns a connection to the server named by DBMETA_AVATICA,
// which is the avatica:// URL that github.com/xo/dbimp/avatica takes, and
// which dburl's avatica scheme opens (D154, D186). The server is the
// standalone Avatica server in front of HSQLDB, and the URL names SA.
func openAvatica(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("DBMETA_AVATICA")
	if dsn == "" {
		t.Skip("set DBMETA_AVATICA to run against a real server")
	}
	return openAvaticaAs(t, dsn)
}

// openAvaticaAs opens a connection with the given URL.
func openAvaticaAs(t *testing.T, dsn string) *sql.DB {
	t.Helper()
	db, err := sql.Open("avatica", dsn)
	if err != nil {
		t.Fatalf("opening: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.PingContext(t.Context()); err != nil {
		t.Fatalf("connecting: %v", err)
	}
	return db
}

// setupAvatica builds the fixture as the administrator and returns the
// metadata for the server. The teardown runs first, because the server keeps
// its database in memory and a run that failed part way leaves the schema
// behind. HSQLDB has no DROP IF EXISTS, so an error from a teardown step is
// expected and ignored.
func setupAvatica(t *testing.T, db *sql.DB) *dbmeta.Meta {
	t.Helper()
	ctx := t.Context()
	versions, err := dbmeta.Avatica.Version(ctx, db)
	if err != nil {
		t.Fatalf("reading the version: %v", err)
	}
	down, err := avfixture.Everything.ResolveTeardown(versions)
	if err != nil {
		t.Fatalf("resolving the teardown: %v", err)
	}
	drop := func(c context.Context) {
		for _, step := range down {
			if !step.Skipped {
				//nolint:errcheck // HSQLDB has no DROP IF EXISTS
				db.ExecContext(c, step.Query)
			}
		}
	}
	drop(ctx)
	up, err := avfixture.Everything.ResolveSetup(versions)
	if err != nil {
		t.Fatalf("resolving the setup: %v", err)
	}
	for _, step := range up {
		if step.Skipped {
			continue
		}
		if _, err := db.ExecContext(ctx, step.Query); err != nil {
			t.Fatalf("setup %s: %v\n%s", step.Name, err, step.Query)
		}
	}
	t.Cleanup(func() { drop(context.WithoutCancel(ctx)) })
	m, err := dbmeta.New(dbmeta.Avatica, versions)
	if err != nil {
		t.Fatalf("building the metadata: %v", err)
	}
	return m
}

// TestAvaticaVersion reads the version and checks what the model makes of it.
// The release is the one of HSQLDB, because no catalog holds the release of
// Avatica.
func TestAvaticaVersion(t *testing.T) {
	db := openAvatica(t)
	versions, err := dbmeta.Avatica.Version(t.Context(), db)
	if err != nil {
		t.Fatalf("reading the version: %v", err)
	}
	main := versions.Main()
	if main.Unknown || len(main.Parts) < 3 || main.Parts[0] != 2 {
		t.Errorf("expected HSQLDB 2.x, got %v", versions)
	}
	if !strings.HasPrefix(versions.String(), "Avatica, HSQLDB 2") {
		t.Errorf("expected the display line to name Avatica and HSQLDB, got %q", versions)
	}
}

// TestAvaticaEveryQueryRuns runs every query the model answers and checks that
// the columns of each row are the declared fields, by name and in order.
func TestAvaticaEveryQueryRuns(t *testing.T) {
	db := openAvatica(t)
	m := setupAvatica(t, db)
	var ran int
	for _, q := range dbmeta.Queries() {
		if q.Support(m) != dbmeta.Supported {
			continue
		}
		query, vals, err := q.Build(m, nil)
		if err != nil {
			t.Errorf("%s: building: %v", q.Name(), err)
			continue
		}
		cols, err := columnsOf(t, db, query, vals)
		if err != nil {
			t.Errorf("%s: %v\n%s", q.Name(), err, query)
			continue
		}
		fields, err := q.Fields(m)
		if err != nil {
			t.Fatalf("%s: reading fields: %v", q.Name(), err)
		}
		names := make([]string, len(fields))
		for i, f := range fields {
			names[i] = f.Name
		}
		if len(cols) == 0 {
			t.Errorf("%s: no columns", q.Name())
		} else if !strings.EqualFold(strings.Join(cols, ","), strings.Join(names, ",")) {
			t.Errorf("%s: returns %v and declares %v", q.Name(), cols, names)
		}
		ran++
	}
	if ran != 24 {
		t.Errorf("%d queries ran, and the model answers 24: change this test with the model", ran)
	}
}

// TestAvaticaScansEveryQuery reads every query through its own Scan, with the
// system objects included.
func TestAvaticaScansEveryQuery(t *testing.T) {
	db := openAvatica(t)
	scanEveryQuery(t, setupAvatica(t, db), db)
}

// TestAvaticaFixtureObjects reads the fixture back through the typed API.
func TestAvaticaFixtureObjects(t *testing.T) {
	db := openAvatica(t)
	m := setupAvatica(t, db)
	ctx := t.Context()
	fx := avfixture.Everything
	args := dbmeta.Args{Schema: fx.Schema}.Map()

	tables := map[string]dbmeta.Table{}
	for v, err := range dbmeta.Tables.All(ctx, m, db, args) {
		if err != nil {
			t.Fatalf("reading tables: %v", err)
		}
		if v.Catalog != "PUBLIC" || v.Schema != fx.Schema {
			t.Errorf("table %s: catalog %q and schema %q, want PUBLIC and %q", v.Name, v.Catalog, v.Schema, fx.Schema)
		}
		tables[v.Name] = v
	}
	for name, want := range map[string]string{
		"AUTHOR": "memory table", "BOOK": "memory table", "RECENT": "view",
		"SCRATCH": "global temporary table",
	} {
		if tables[name].Type != want {
			t.Errorf("expected %s as %q with no padding, got %q", name, want, tables[name].Type)
		}
	}
	if c := tables["AUTHOR"].Comment; !c.Valid || c.V != "people who write" {
		t.Errorf("expected the table comment, got %+v", c)
	}
	// The fields of D210. The administrator owns the schema, a view has no
	// persistence, and the row count and the size are left out because
	// SYSTEM_TABLESTATS cost grows with the square of the tables.
	author := tables["AUTHOR"]
	if author.Persistence.V != "permanent" || !author.Owner.Valid {
		t.Errorf("expected permanent and an owner for AUTHOR, got %+v", author)
	}
	if v := tables["RECENT"]; v.Persistence.Valid {
		t.Errorf("expected a view to have no persistence, got %+v", v)
	}
	if v := tables["SCRATCH"]; v.Persistence.V != "temporary" {
		t.Errorf("expected SCRATCH as temporary, got %+v", v.Persistence)
	}
	if author.Size.Valid || author.Rows.Valid || author.Options.Valid {
		t.Errorf("expected no size, no rows and no options, got %+v", author)
	}
	if n := countRows(t, dbmeta.Tables.All(ctx, m, db, dbmeta.Args{Schema: "INFORMATION_SCHEMA"}.Map())); n != 0 {
		t.Errorf("expected no INFORMATION_SCHEMA table without with_system, got %d", n)
	}
	sys := map[string]string{}
	for v, err := range dbmeta.Tables.All(ctx, m, db, dbmeta.Args{Schema: "INFORMATION_SCHEMA", WithSystem: true}.Map()) {
		if err != nil {
			t.Fatalf("reading the system tables: %v", err)
		}
		sys[v.Name] = v.Type
	}
	if sys["SYSTEM_COMMENTS"] != "system table" {
		t.Errorf("expected SYSTEM_COMMENTS as a system table, got %q", sys["SYSTEM_COMMENTS"])
	}
	if n := countRows(t, dbmeta.Tables.All(ctx, m, db, dbmeta.Args{Schema: fx.Schema, Types: []string{"view"}}.Map())); n != 1 {
		t.Errorf("expected one view with the types filter, got %d", n)
	}

	cols := map[string]dbmeta.Column{}
	for v, err := range dbmeta.Columns.All(ctx, m, db, args) {
		if err != nil {
			t.Fatalf("reading columns: %v", err)
		}
		cols[v.Table+"."+v.Name] = v
	}
	for name, want := range map[string][2]string{
		"AUTHOR.NAME":          {"CHARACTER VARYING(128)", "SQL_TEXT"},
		"SHIPMENT.AMOUNT":      {"DECIMAL(12,2)", ""},
		"BOOK.PUBLISHED":       {"DATE", ""},
		"TICKET.TICKET_ID":     {"INTEGER", ""},
		"AUTHOR.AUTHOR_ID":     {"INTEGER", ""},
		"REGION.COUNTRY":       {"CHARACTER VARYING(64)", "SQL_TEXT"},
		"SHIPMENT.SHIPMENT_ID": {"INTEGER", ""},
	} {
		c, ok := cols[name]
		if !ok || c.DataType != want[0] || c.Collation.V != want[1] {
			t.Errorf("%s: want %v, got %+v", name, want, c)
		}
	}
	if c := cols["AUTHOR.SHADE"]; !c.Default.Valid || c.Default.V != "'plain'" {
		t.Errorf("expected the default 'plain' with its quotes, got %+v", c.Default)
	}
	if c := cols["TICKET.TICKET_ID"]; c.Identity.V != "by default" || !c.PrimaryKey {
		t.Errorf("expected an identity column by default and a key, got %+v", c)
	}
	if c := cols["TICKET.TWICE"]; c.Generated.V != "stored" || c.Identity.V != "" {
		t.Errorf("expected a stored generated column, got %+v", c)
	}
	if c := cols["AUTHOR.NAME"]; c.Comment.V != "the author name" || c.Nullable {
		t.Errorf("expected the column comment and NOT NULL, got %+v", c)
	}
	if c := cols["REGION.AREA"]; !c.PrimaryKey {
		t.Errorf("expected region.area in the key, got %+v", c)
	}

	idx := map[string]dbmeta.Index{}
	for v, err := range dbmeta.Indexes.All(ctx, m, db, args) {
		if err != nil {
			t.Fatalf("reading indexes: %v", err)
		}
		idx[v.Table+"."+strings.Split(v.Name, "_")[0]+"."+v.Name] = v
	}
	var pk, unique, plain int
	for _, v := range idx {
		switch {
		case v.Primary:
			pk++
		case v.Unique:
			unique++
		default:
			plain++
		}
	}
	if pk != 5 || unique != 1 || plain != 3 {
		t.Errorf("expected 5 primary, 1 unique and 3 plain indexes, got %d, %d and %d", pk, unique, plain)
	}

	kinds := map[string]string{}
	for v, err := range dbmeta.Constraints.All(ctx, m, db, args) {
		if err != nil {
			t.Fatalf("reading constraints: %v", err)
		}
		kinds[v.Name] = v.Type
		if v.Name == "BOOK_TITLE_CK" && (!v.Definition.Valid || !strings.Contains(v.Definition.V, "CHAR_LENGTH")) {
			t.Errorf("expected the CHECK text, got %+v", v.Definition)
		}
	}
	for name, want := range map[string]string{
		"AUTHOR_PK": "primary key", "BOOK_AUTHOR_FK": "foreign key",
		"BOOK_TITLE_UQ": "unique", "BOOK_TITLE_CK": "check",
	} {
		if kinds[name] != want {
			t.Errorf("constraint %s: want %q, got %q", name, want, kinds[name])
		}
	}

	seq, ok, err := dbmeta.First(dbmeta.Sequences.All(ctx, m, db, args))
	if err != nil || !ok || seq.Name != "DBMETA_COUNTER" || seq.Start.V != "5" || seq.Increment.V != "2" ||
		seq.Maximum.V != "1000" || !seq.Cycles.V {
		t.Errorf("expected the sequence, got %+v, %v, %v", seq, ok, err)
	}

	fns := map[string]dbmeta.Function{}
	for v, err := range dbmeta.Functions.All(ctx, m, db, args) {
		if err != nil {
			t.Fatalf("reading functions: %v", err)
		}
		fns[v.Name] = v
	}
	if f := fns["DOUBLED"]; f.Kind != "function" || f.Volatility != "immutable" || f.ResultType.V != "INTEGER" {
		t.Errorf("expected the function doubled, got %+v", f)
	}
	if f := fns["BOOK_COUNT"]; f.Kind != "procedure" || f.Volatility != "volatile" || f.ResultType.Valid {
		t.Errorf("expected the procedure book_count, got %+v", f)
	}
	if _, ok := fns["DBMETA_TOTAL"]; ok {
		t.Error("expected the aggregate to stay out of Functions")
	}
	agg, ok, err := dbmeta.First(dbmeta.Aggregates.All(ctx, m, db, args))
	if err != nil || !ok || agg.Name != "DBMETA_TOTAL" || agg.Kind != "aggregate" {
		t.Errorf("expected the aggregate, got %+v, %v, %v", agg, ok, err)
	}
	var modes []string
	for v, err := range dbmeta.RoutineParameters.All(ctx, m, db, dbmeta.Args{Schema: fx.Schema, Parent: "DBMETA_TOTAL"}.Map()) {
		if err != nil {
			t.Fatalf("reading routine parameters: %v", err)
		}
		modes = append(modes, v.Mode)
	}
	if strings.Join(modes, ",") != "in,in,inout,inout" {
		t.Errorf("expected the modes of the aggregate, got %v", modes)
	}

	typ, ok, err := dbmeta.First(dbmeta.Types.All(ctx, m, db, args))
	if err != nil || !ok || typ.Name != "DBMETA_MONEY" || typ.Kind != "distinct" || typ.Internal != "DECIMAL(12,2)" {
		t.Errorf("expected the distinct type, got %+v, %v, %v", typ, ok, err)
	}
	dom, ok, err := dbmeta.First(dbmeta.Domains.All(ctx, m, db, args))
	if err != nil || !ok || dom.Name != "DBMETA_RATING" || dom.Default.V != "0" || !strings.Contains(dom.Constraints, "VALUE") {
		t.Errorf("expected the domain, got %+v, %v, %v", dom, ok, err)
	}

	trg, ok, err := dbmeta.First(dbmeta.Triggers.All(ctx, m, db, args))
	if err != nil || !ok || trg.Name != "BOOK_BI" || trg.Table != "BOOK" || !strings.Contains(trg.Definition, "BEFORE INSERT") {
		t.Errorf("expected the trigger, got %+v, %v, %v", trg, ok, err)
	}

	view, ok, err := dbmeta.First(dbmeta.Views.All(ctx, m, db, args))
	if err != nil || !ok || view.Name != "RECENT" || view.CheckOption.V != "cascaded" || !view.Updatable.V ||
		view.Comment.V != "the newest books" {
		t.Errorf("expected the view, got %+v, %v, %v", view, ok, err)
	}

	// The fixture grants SELECT on author to dbmeta_reader, and a grant on two
	// columns to dbmeta_member.
	priv, ok, err := dbmeta.First(dbmeta.Privileges.All(ctx, m, db, dbmeta.Args{Schema: fx.Schema, Name: "AUTHOR"}.Map()))
	if err != nil || !ok || !strings.Contains(priv.Access.V, "DBMETA_READER=SELECT") ||
		!strings.Contains(priv.ColumnAccess.V, "NAME:DBMETA_MEMBER=SELECT") ||
		!strings.Contains(priv.ColumnAccess.V, "RATING:DBMETA_MEMBER=UPDATE") ||
		strings.Contains(priv.ColumnAccess.V, "DBMETA_READER") {
		t.Errorf("expected the grants on author, got %+v, %v, %v", priv, ok, err)
	}

	roles := map[string]dbmeta.Role{}
	for v, err := range dbmeta.Roles.All(ctx, m, db, nil) {
		if err != nil {
			t.Fatalf("reading roles: %v", err)
		}
		roles[v.Name] = v
	}
	if r := roles["SA"]; !r.Superuser || !r.CanLogin || r.MemberOf != "DBA" {
		t.Errorf("expected SA as a superuser holding DBA, got %+v", r)
	}
	if r := roles["DBMETA_READER"]; r.CanLogin || r.MemberOf != "DBMETA_MEMBER" {
		t.Errorf("expected dbmeta_reader as a role in dbmeta_member, got %+v", r)
	}
	grant, ok, err := dbmeta.First(dbmeta.RoleGrants.All(ctx, m, db, dbmeta.Args{Name: "DBMETA_READER"}.Map()))
	if err != nil || !ok || grant.MemberOf != "DBMETA_MEMBER" || grant.Admin {
		t.Errorf("expected the role grant, got %+v, %v, %v", grant, ok, err)
	}

	cm := map[string]string{}
	for v, err := range dbmeta.Comments.All(ctx, m, db, dbmeta.Args{}.Map()) {
		if err != nil {
			t.Fatalf("reading comments: %v", err)
		}
		cm[v.Type+" "+v.Name] = v.Comment
	}
	if cm["column AUTHOR.NAME"] != "the author name" || cm["table AUTHOR"] != "people who write" ||
		cm["view RECENT"] != "the newest books" {
		t.Errorf("expected the three comments, got %v", cm)
	}

	cs, ok, err := dbmeta.First(dbmeta.CurrentSchema.All(ctx, m, db, nil))
	if err != nil || !ok || cs.Name != "PUBLIC" || cs.Catalog != "PUBLIC" || cs.Owner != "DBA" {
		t.Errorf("expected the current schema PUBLIC owned by DBA, got %+v, %v, %v", cs, ok, err)
	}
	u, ok, err := dbmeta.First(dbmeta.CurrentUser.All(ctx, m, db, nil))
	if err != nil || !ok || u.Name != "SA" {
		t.Errorf("expected the current user SA, got %+v, %v, %v", u, ok, err)
	}
	d, ok, err := dbmeta.First(dbmeta.Databases.All(ctx, m, db, nil))
	if err != nil || !ok || d.Name != "PUBLIC" || d.Encoding != "UTF16" || d.Collate != "SQL_TEXT" {
		t.Errorf("expected the database PUBLIC, got %+v, %v, %v", d, ok, err)
	}
	set, ok, err := dbmeta.First(dbmeta.Settings.All(ctx, m, db, dbmeta.Args{Name: "sql.enforce_size"}.Map()))
	if err != nil || !ok || set.Value.V != "true" || set.Type.V != "Boolean" {
		t.Errorf("expected sql.enforce_size, got %+v, %v, %v", set, ok, err)
	}
}

// TestAvaticaOrdinaryUser checks what the user the dbrun entry makes sees. It
// can read one table, DBMETA.READABLE, and HSQLDB lists in INFORMATION_SCHEMA
// what the user has a privilege on. It sees every schema, owns none, and
// reads the version.
func TestAvaticaOrdinaryUser(t *testing.T) {
	admin := openAvatica(t)
	m := setupAvatica(t, admin)
	dsn := os.Getenv("DBMETA_AVATICA")
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	u.User = url.UserPassword(container.AvaticaUser, container.Password)
	db := openAvaticaAs(t, u.String())
	ctx := t.Context()

	if _, err := dbmeta.Avatica.Version(ctx, db); err != nil {
		t.Errorf("expected the ordinary user to read the version, got %v", err)
	}
	var names []string
	for v, err := range dbmeta.Tables.All(ctx, m, db, nil) {
		if err != nil {
			t.Fatalf("reading tables: %v", err)
		}
		names = append(names, v.Schema+"."+v.Name)
	}
	if strings.Join(names, ",") != "DBMETA.READABLE" {
		t.Errorf("expected DBMETA.READABLE alone, got %v", names)
	}
	schemas := map[string]string{}
	for v, err := range dbmeta.Schemas.All(ctx, m, db, nil) {
		if err != nil {
			t.Fatalf("reading schemas: %v", err)
		}
		schemas[v.Name] = v.Owner
	}
	if len(schemas) != 3 || schemas["DBMETA"] != "" || schemas["PUBLIC"] != "" {
		t.Errorf("expected three schemas and no owner for any, got %v", schemas)
	}
	roles := map[string]bool{}
	for v, err := range dbmeta.Roles.All(ctx, m, db, nil) {
		if err != nil {
			t.Fatalf("reading roles: %v", err)
		}
		roles[v.Name] = v.Superuser
	}
	if len(roles) != 1 || roles[container.AvaticaUser] {
		t.Errorf("expected the user alone and no superuser, got %v", roles)
	}
	cu, ok, err := dbmeta.First(dbmeta.CurrentUser.All(ctx, m, db, nil))
	if err != nil || !ok || cu.Name != container.AvaticaUser {
		t.Errorf("expected the current user to be the ordinary one, got %+v, %v, %v", cu, ok, err)
	}
}

// TestAvaticaLeavesOut checks the kinds the model does not answer. HSQLDB has
// no tablespace, access method, language, cast, operator or foreign object.
func TestAvaticaLeavesOut(t *testing.T) {
	m, err := dbmeta.New(dbmeta.Avatica, dbmeta.VersionSet{})
	if err != nil {
		t.Fatal(err)
	}
	for _, q := range []dbmeta.AnyQuery{
		dbmeta.Tablespaces, dbmeta.AccessMethods, dbmeta.Languages, dbmeta.Conversions, dbmeta.Casts,
		dbmeta.LargeObjects, dbmeta.EventTriggers, dbmeta.Operators, dbmeta.ForeignTables,
		dbmeta.PartitionedTables, dbmeta.EnumValues, dbmeta.ColumnStats, dbmeta.Extensions,
	} {
		if s := q.Support(m); s != dbmeta.NotSupported {
			t.Errorf("%s: %v, want NotSupported", q.Name(), s)
		}
	}
	if _, _, err := dbmeta.Casts.Build(m, nil); !errors.Is(err, dbmeta.ErrNotSupported) {
		t.Errorf("casts: expected ErrNotSupported, got %v", err)
	}
}
