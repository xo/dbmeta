// Package fixture holds known good PostgreSQL schemas.
//
// A fixture is a schema that contains one of everything the metadata queries
// read, so a test can ask a real server for metadata and get an answer worth
// checking. `dbmeta` uses them for its own integration tests, and they are
// exported so that other projects can use them too. A code generator such as
// `dbtpl` can generate against a schema that is known to exercise every object
// kind, without writing one of its own.
//
// A fixture is SQL text and nothing else. It needs no database driver to
// define, only to execute, so this package stays inside the root module and
// D26 holds.
//
// # Versions
//
// A fixture varies by server version for the same reason a query does. Some
// syntax arrives in a later release, and some objects do not exist at all on
// an older one. `CREATE TRIGGER ... EXECUTE FUNCTION` is release 11 syntax and
// `EXECUTE PROCEDURE` is what works below it. Writing the deprecated form
// everywhere would mean the newest release is tested with syntax nobody
// writes, so each step carries alternatives exactly as a query piece does.
//
// A step with no alternative for the server is skipped rather than refused.
// That is the one place a fixture differs from a query: asking for
// publications on release 9.6 is an error, but creating one there is simply
// something the fixture does not do. [Result.Skipped] records it.
//
// # What you may depend on
//
// A fixture is additive. A later release of `dbmeta` may add an object to one,
// and will not rename or remove what is already there. Code written against a
// fixture keeps working when this package grows.
//
// Adding an object is not free for a caller that counts rows, so a caller that
// needs an exact set should name the objects it reads rather than reading
// everything in the schema.
//
// To change a schema in a way that is not additive, add a new fixture beside
// the old one rather than editing it.
package fixture

import (
	"errors"

	"github.com/xo/dbmeta"
)

// Step is one statement of a fixture, with its alternatives by version.
type Step struct {
	// Name says what the step creates, for a caller reporting what it ran or
	// skipped.
	Name string
	// Stmt is the statement, which may differ by server version.
	Stmt dbmeta.Stmt
}

// Result is what a step resolved to for one server.
type Result struct {
	// Name is the step's name.
	Name string
	// SQL is the statement to run. It is empty when the step was skipped.
	SQL string
	// Skipped reports that the server is too old for the step, so there is
	// nothing to run. The object the step would have created does not exist on
	// that release, and the query that reads it is refused there too.
	Skipped bool
	// Reason says why the step was skipped.
	Reason string
}

// Fixture is a schema, with the statements that build it and drop it.
type Fixture struct {
	// Name identifies the fixture.
	Name string
	// Schema is the schema the fixture creates. Everything it builds lives
	// here, so a caller can read one schema and see the whole fixture.
	Schema string
	// Setup builds the schema, in order.
	Setup []Step
	// Teardown drops it.
	Teardown []Step
}

// ResolveSetup returns the statements that build the fixture on a server at
// this version, in order, with the skipped steps marked.
func (f Fixture) ResolveSetup(versions dbmeta.VersionSet) ([]Result, error) {
	return resolve(f.Setup, versions)
}

// ResolveTeardown returns the statements that drop the fixture.
func (f Fixture) ResolveTeardown(versions dbmeta.VersionSet) ([]Result, error) {
	return resolve(f.Teardown, versions)
}

// resolve turns steps into results, marking the ones the server is too old
// for. It reads the same error the query path returns, so there is one rule
// for when a version does not apply and two ways of reacting to it.
func resolve(steps []Step, versions dbmeta.VersionSet) ([]Result, error) {
	out := make([]Result, 0, len(steps))
	for _, step := range steps {
		sqlstr, err := step.Stmt.SQL(versions)
		switch {
		case errors.Is(err, dbmeta.ErrVersionTooOld):
			out = append(out, Result{
				Name:    step.Name,
				Skipped: true,
				Reason:  "the server is older than this step needs",
			})
		case err != nil:
			return nil, err
		default:
			out = append(out, Result{Name: step.Name, SQL: sqlstr})
		}
	}
	return out, nil
}

// at builds a step whose statement is the same on every server.
func at(name, sqlstr string) Step {
	return Step{Name: name, Stmt: dbmeta.Always(sqlstr)}
}

// from builds a step that needs a server at min or newer, and is skipped
// below it.
func from(name string, min dbmeta.Version, sqlstr string) Step {
	return Step{Name: name, Stmt: dbmeta.Stmt{{{Min: min, SQL: sqlstr}}}}
}

// choose builds a step whose syntax changed, taking the alternatives in any
// order.
func choose(name string, alts ...dbmeta.Fragment) Step {
	return Step{Name: name, Stmt: dbmeta.Stmt{dbmeta.Choice(alts)}}
}
