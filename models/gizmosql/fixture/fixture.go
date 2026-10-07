// Package fixture holds a known good GizmoSQL schema.
//
// GizmoSQL runs DuckDB, and it builds the DuckDB fixture as it is. [Everything]
// is that fixture, with the same steps, the same schema and the same types.
// GizmoSQL adds no object that a query reads, so there is nothing to add. The
// one object of its own, the _gizmosql_system database, is not built here,
// because the server makes it.
package fixture

import dkfixture "github.com/xo/dbmeta/models/duckdb/fixture"

// Everything is a schema holding one of every object the GizmoSQL queries
// read. It is the DuckDB fixture.
var Everything = dkfixture.Everything
