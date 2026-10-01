// Package fixture holds known good Vitess schemas.
//
// A fixture is a schema that contains one of everything the metadata queries
// read, so that a test asks a real server for metadata and gets an answer
// worth checking. Read the mysql fixture package for what a fixture is and
// what a caller can depend on. The types are that package's.
//
// Vitess builds most of the MySQL fixture as it is, so [Everything] is the
// MySQL fixture step by step, less the steps that vtgate refuses: CREATE
// FUNCTION, CREATE TRIGGER and CREATE SERVER are syntax errors to it. A
// procedure is accepted. It adds one step, a sequence, which in Vitess is a
// table with the comment vitess_sequence. The steps and the reasons were
// measured on 23.0.6 and 24.0.3, on 2026-09-30. See D135.
package fixture

import (
	"github.com/xo/dbmeta"
	myfixture "github.com/xo/dbmeta/models/mysql/fixture"
)

// left names each MySQL step that vtgate refuses, with the reason. The
// queries that read what the step builds are not answered on Vitess.
var left = map[string]string{
	"function":       "vtgate refuses CREATE FUNCTION",
	"trigger":        "vtgate refuses CREATE TRIGGER",
	"foreign server": "vtgate refuses CREATE SERVER",
}

// Everything is a schema containing one of every object kind the Vitess
// queries read, built from the MySQL fixture of the same name.
var Everything = func() myfixture.Fixture {
	my := myfixture.Everything
	f := myfixture.Fixture{Name: my.Name, Schema: my.Schema}
	for _, s := range my.Setup {
		if _, skip := left[s.Name]; !skip {
			f.Setup = append(f.Setup, s)
		}
	}
	// A sequence is a table with this comment. The VSchema entry that lets
	// vtgate take values from it is left out, because the metadata is the
	// table and a second run finds the entry there already.
	f.Setup = append(f.Setup, myfixture.Step{
		Name: "vitess sequence",
		Stmt: dbmeta.Always("CREATE TABLE dbmeta_fixture.counter (\n" +
			"	id integer PRIMARY KEY,\n" +
			"	next_id bigint,\n" +
			"	cache bigint\n" +
			") COMMENT 'vitess_sequence'"),
	})
	// The MySQL teardown drops a server, which vtgate refuses.
	for _, s := range my.Teardown {
		if s.Name != "drop server" {
			f.Teardown = append(f.Teardown, s)
		}
	}
	return f
}()

// Left says why [Everything] has no step of this name that the MySQL fixture
// has, and is empty when it has one.
func Left(step string) string { return left[step] }
