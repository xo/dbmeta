// Package fixture holds known good TiDB schemas.
//
// A fixture is a schema that contains one of everything the metadata queries
// read, so that a test asks a real server for metadata and gets an answer
// worth checking. Read the mysql fixture package for what a fixture is and
// what a caller can depend on. The types are that package's.
//
// TiDB builds most of the MySQL fixture as it is, so [Everything] is the
// MySQL fixture step by step. It keeps each step that TiDB builds, leaves out
// each step that TiDB cannot build at all, and adds the steps for the objects
// TiDB has and the MySQL fixture does not build on MySQL: a sequence, a role,
// a user and the grants between them. The steps and the reasons were measured
// on 7.5.8 and 8.5.8, on 2026-09-29. See D133.
//
// TiDB accepts a CHECK constraint and enforces it only when
// tidb_enable_check_constraint is on, which it is not by default, and the
// mysql model lists a check only from MySQL 8.0.16, which TiDB does not claim.
// So the check step runs and no query reports it.
package fixture

import (
	"github.com/xo/dbmeta"
	myfixture "github.com/xo/dbmeta/models/mysql/fixture"
)

// left names each MySQL step that TiDB cannot build, with the reason. The
// queries that read what the step builds are not answered on TiDB.
var left = map[string]string{
	"function":                  "TiDB has no stored function",
	"procedure":                 "TiDB has no stored procedure",
	"procedure with parameters": "TiDB has no stored procedure",
	"trigger":                   "TiDB has no trigger",
	"foreign server":            "TiDB has no CREATE SERVER",
}

func at(name, query string) myfixture.Step {
	return myfixture.Step{Name: name, Stmt: dbmeta.Always(query)}
}

// Everything is a schema containing one of every object kind the TiDB
// queries read, built from the MySQL fixture of the same name.
var Everything = func() myfixture.Fixture {
	my := myfixture.Everything
	f := myfixture.Fixture{
		Name:   my.Name,
		Schema: my.Schema,
		// The MySQL teardown drops a server, which TiDB has not got. The role
		// and the user are outside the schema.
		Teardown: []myfixture.Step{
			at("drop schema", `DROP SCHEMA IF EXISTS `+my.Schema),
			at("drop user", `DROP USER IF EXISTS 'dbmeta_member'@'%'`),
			at("drop role", `DROP ROLE IF EXISTS 'dbmeta_reader'@'%'`),
		},
	}
	for _, s := range my.Setup {
		if _, skip := left[s.Name]; skip {
			continue
		}
		f.Setup = append(f.Setup, s)
	}
	f.Setup = append(f.Setup,
		at("sequence", `CREATE SEQUENCE `+my.Schema+`.counter START WITH 10 INCREMENT BY 2`),
		// A role with a privilege on the schema, granted to a user, so that
		// the roles, role grants and privileges queries have rows.
		at("role", `CREATE ROLE 'dbmeta_reader'@'%'`),
		at("role privilege", `GRANT SELECT ON `+my.Schema+`.* TO 'dbmeta_reader'@'%'`),
		at("user", `CREATE USER 'dbmeta_member'@'%' IDENTIFIED BY 'P4ssw0rd!x'`),
		at("role grant", `GRANT 'dbmeta_reader'@'%' TO 'dbmeta_member'@'%'`),
		at("table privilege", `GRANT SELECT ON `+my.Schema+`.book TO 'dbmeta_member'@'%'`),
	)
	return f
}()

// Left says why [Everything] has no step of this name that the MySQL fixture
// has, and is empty when it has one.
func Left(step string) string { return left[step] }
