//go:build !omni

package main

import (
	// The driver of dbimp reads Cloud Spanner over REST. Spanner Omni speaks gRPC
	// only, and go-sql-spanner reads it in a dbrun built with -tags omni. Both
	// register the name spanner, so one dbrun holds one of them. See D231.
	_ "github.com/xo/dbimp/spanner"
)

// spannerOmniBuild says that this dbrun reads Spanner Omni. It does not.
func spannerOmniBuild() bool { return false }
