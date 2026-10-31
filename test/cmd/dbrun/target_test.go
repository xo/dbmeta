package main

import "testing"

// TestAVariantIsNeverTheNewestOracle holds that a bare oracle names the
// newest ordinary release. An Autonomous Database has the number of its image,
// which sorts above every database, and a server that requires encryption has
// the release of 26ai.
func TestAVariantIsNeverTheNewestOracle(t *testing.T) {
	t.Parallel()
	got := newestOf([]target{
		{Name: "oracle-26ai", Release: "23.26.3"},
		{Name: "oracle-adb-19c", Release: "26.9.4.2"},
		{Name: "oracle-nne-26ai", Release: "23.26.3"},
	})
	if got.Name != "oracle-26ai" {
		t.Errorf("the newest Oracle is %s, and want oracle-26ai", got.Name)
	}
}
