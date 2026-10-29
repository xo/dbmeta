package test

import (
	"os"
	"testing"
)

// TestMain drops the Spanner, BigQuery, Athena, Databricks, Cosmos DB and MaxCompute fixtures after the last test. Each
// takes minutes to build, so the tests share one, and nothing else in this
// package needs a main of its own.
func TestMain(m *testing.M) {
	code := m.Run()
	shutdownSpanner()
	shutdownBigQuery()
	shutdownAthena()
	shutdownDatabricks()
	shutdownCosmos()
	shutdownMaxCompute()
	shutdownTablestore()
	os.Exit(code)
}
