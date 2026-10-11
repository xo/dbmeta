//go:build omni

package main

import (
	// go-sql-spanner reads Spanner Omni over gRPC. The driver of dbimp reads
	// Cloud Spanner, in the default dbrun. Both register the name spanner, so
	// one dbrun holds one of them. See D231.
	_ "github.com/googleapis/go-sql-spanner"
)

// spannerOmniBuild says that this dbrun reads Spanner Omni.
func spannerOmniBuild() bool { return true }
