package dbmeta_test

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strings"

	"github.com/xo/dbmeta"
	_ "github.com/xo/dbmeta/all"
)

// Example shows what a client such as usql or dbtpl does: read the server
// version, list the tables in a schema, then read the columns of one table.
func Example() {
	ctx := context.Background()

	// The client opens the connection. dbmeta never does, and never imports a
	// driver. A real client would use dburl.Open here.
	db, err := sql.Open("examplefake", "postgres://localhost/example")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// The client picks the dialect. dbmeta does not detect it. A real client
	// takes it from the parsed URL, as dburl.URL.Driver.
	dialect := dbmeta.PostgreSQL

	// Read the version. dbmeta supplies the statement, runs it against the
	// connection the client passed, and parses the answer. A client that
	// wants the statement without running it asks for it instead, and a
	// client that wants to force a version skips this entirely.
	versions, err := dialect.Version(ctx, db)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(versions)

	// The dialect and the version together decide which fragment of each
	// query applies. A client that wants to force a version passes its own
	// here instead.
	m, err := dbmeta.New(dialect, versions)
	if err != nil {
		log.Fatal(err)
	}

	// List the tables.
	fmt.Println("\ntables:")
	for t, err := range dbmeta.Tables.All(ctx, m, db, dbmeta.Args{Schema: "public"}.Map()) {
		if err != nil {
			log.Fatal(err)
		}
		line := fmt.Sprintf("  %s.%s (%s)", t.Schema, t.Name, t.Type)
		// Comment is a sql.Null[string], so a table with no comment is absent rather than
		// empty. Reading .V prints empty for both, which is what a CLI wants.
		if t.Comment.Valid {
			line += " " + t.Comment.V
		}
		fmt.Println(line)
	}

	// Read the columns of one table.
	fmt.Println("\ncolumns of public.book:")
	args := dbmeta.Args{Schema: "public", Parent: "book"}.Map()
	for c, err := range dbmeta.Columns.All(ctx, m, db, args) {
		if err != nil {
			log.Fatal(err)
		}
		null := "not null"
		if c.Nullable {
			null = "null"
		}
		fmt.Printf("  %d %-10s %-8s %s\n", c.Ordinal, c.Name, c.DataType, null)
	}

	// Output:
	// PostgreSQL 16.2
	//
	// tables:
	//   public.author (table) people who write
	//   public.book (table)
	//   public.recent_book (view)
	//
	// columns of public.book:
	//   1 book_id    integer  not null
	//   2 title      text     not null
	//   3 published  date     null
	//   4 slug       text     null
}

// Example_sql shows a client taking the statement and running it itself, which
// is what usql does for a command that prints raw output.
func Example_sql() {
	m, err := dbmeta.New(dbmeta.PostgreSQL, versionSet("16.2"))
	if err != nil {
		log.Fatal(err)
	}
	query, args, err := dbmeta.Schemas.Build(m, dbmeta.Args{Name: "public"}.Map())
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(query)
	fmt.Println("args:", args)

	// Output:
	// SELECT current_database() AS "catalog"
	// , n.nspname AS "name"
	// , pg_catalog.pg_get_userbyid(n.nspowner) AS "owner"
	// , pg_catalog.obj_description(n.oid, 'pg_namespace') AS "comment"
	// FROM pg_catalog.pg_namespace n
	// WHERE ($1 OR (n.nspname !~ '^pg_' AND n.nspname <> 'information_schema'))
	// AND ($2 = '' OR n.nspname LIKE $3)
	// ORDER BY 2
	// args: [false public public]
}

// Example_oldServer shows the same query against a server too old for one of
// its columns. The column set does not change: the comment is padded with a
// literal instead, and the declared field says why.
func Example_oldServer() {
	sqlOf := func(ver string) string {
		m, err := dbmeta.New(dbmeta.PostgreSQL, versionSet(ver))
		if err != nil {
			log.Fatal(err)
		}
		query, _, err := dbmeta.Columns.Build(m, dbmeta.Args{Schema: "public"}.Map())
		if err != nil {
			log.Fatal(err)
		}
		return query
	}
	// attidentity arrived in release 11 and attgenerated in release 12, so an
	// older server selects a literal under the same name. The column set never
	// changes, which is what lets one scan function read every release.
	fmt.Println("10 pads identity: ", strings.Contains(sqlOf("10.23"), `, NULL AS "identity"`))
	fmt.Println("11 reads identity:", strings.Contains(sqlOf("11.22"), `a.attidentity`))
	fmt.Println("11 pads generated:", strings.Contains(sqlOf("11.22"), `, NULL AS "generated"`))
	fmt.Println("12 reads generated:", strings.Contains(sqlOf("12.18"), `a.attgenerated`))

	// a caller tells "absent at this version" from "genuinely null" by the
	// minimum the field declares
	m, err := dbmeta.New(dbmeta.PostgreSQL, versionSet("10.23"))
	if err != nil {
		log.Fatal(err)
	}
	fields, err := dbmeta.Columns.Fields(m)
	if err != nil {
		log.Fatal(err)
	}
	for _, f := range fields {
		if f.Name == "identity" {
			fmt.Println("identity present at 10.23:", m.Version().Main().AtLeast(f.Min))
		}
	}

	// Output:
	// 10 pads identity:  true
	// 11 reads identity: true
	// 11 pads generated: true
	// 12 reads generated: true
	// identity present at 10.23: false
}

// Example_support shows the four states a client must tell apart before it
// offers a command to a person.
func Example_support() {
	m, err := dbmeta.New(dbmeta.PostgreSQL, versionSet("16.2"))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("tables: ", dbmeta.Tables.Support(m))
	fmt.Println("indexes:", dbmeta.Indexes.Support(m))

	// an object this release does not have is a different answer: the product
	// has it and the server is too old, which an upgrade fixes. Support says
	// so on its own, and Build then returns ErrVersionTooOld. See D63.
	old := versionMeta("9.6.24")
	fmt.Println("publications on 9.6:", dbmeta.Publications.Support(old))
	if _, _, err := dbmeta.Publications.Build(old, nil); err != nil {
		fmt.Println("and building it:", err)
	}

	// a dialect no model was built for is a different answer entirely.
	// Trino is one usql speaks and dbmeta has no model for. It is next on
	// D66's list, so this example moves on when it arrives.
	if _, err := dbmeta.New("trino", dbmeta.VersionSet{}); err != nil {
		fmt.Println("trino:", err)
	}

	// Output:
	// tables:  supported
	// indexes: supported
	// publications on 9.6: version too old
	// and building it: version too old
	// trino: model not built
}

func versionMeta(ver string) *dbmeta.Meta {
	m, err := dbmeta.New(dbmeta.PostgreSQL, versionSet(ver))
	if err != nil {
		log.Fatal(err)
	}
	return m
}

func versionSet(s string) dbmeta.VersionSet {
	var v dbmeta.VersionSet
	v.Set("", dbmeta.ParseVersion(s))
	v.Display = "PostgreSQL " + s
	return v
}

// Example_versionByHand shows the two step form, for a client that wants the
// statement without running it. usql prints the statement in its trace output,
// so it needs this rather than [dbmeta.Dialect.Version].
func Example_versionByHand() {
	query, n, ok := dbmeta.PostgreSQL.VersionQuery()
	if !ok {
		log.Fatal("expected a version query")
	}
	fmt.Printf("%s (%d column)\n", query, n)

	// the client runs it however it likes, then hands the columns back
	versions, err := dbmeta.PostgreSQL.ParseVersion([]string{"16.2"})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(versions)

	// Output:
	// SHOW server_version (1 column)
	// PostgreSQL 16.2
}
