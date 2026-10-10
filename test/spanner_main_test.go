package test

import (
	"os"
	"testing"
)

// TestMain drops the Spanner fixture after the last test. The fixture takes
// minutes to build, so the tests share one, and nothing else in this package
// needs a main of its own.
func TestMain(m *testing.M) {
	code := m.Run()
	shutdownSpanner()
	os.Exit(code)
}
