// Package fixture holds a known good rqlite schema.
//
// rqlite runs SQLite, and it builds the SQLite fixture as it is, so
// [Everything] is the sqlite3 model's fixture. The types are that package's.
package fixture

import sqfixture "github.com/xo/dbmeta/models/sqlite3/fixture"

// Everything is a schema holding one of every object the rqlite queries read,
// which is the SQLite fixture.
var Everything = sqfixture.Everything
