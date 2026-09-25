package test

import (
	"os"
	"path/filepath"
	"testing"
)

// embeddedFile is where a library backed test keeps its database.
//
// dbrun names a file in the environment, the way it names a DSN for a server,
// and that file is kept between runs so a person can open it afterwards with
// `dbrun usql sqlite3`. A container is left running for the same reason.
//
// With no variable set the test makes its own in a temporary directory, so
// `go test ./...` on its own stays isolated and leaves nothing behind.
func embeddedFile(t *testing.T, env, name string) string {
	t.Helper()
	if path := os.Getenv(env); path != "" {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("making the directory for %s: %v", path, err)
		}
		return path
	}
	return filepath.Join(t.TempDir(), name)
}
