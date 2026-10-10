package test

import (
	"os"
	"testing"
)

// TestMain drops the Spanner, BigQuery, Athena and Databricks fixtures after the last test. Each
// takes minutes to build, so the tests share one, and nothing else in this
// package needs a main of its own.
func TestMain(m *testing.M) {
	code := m.Run()
	shutdownSpanner()
	shutdownBigQuery()
	shutdownAthena()
	shutdownDatabricks()
	shutdownCosmos()
	os.Exit(code)
}
