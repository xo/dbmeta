package main

import "testing"

// TestAnAutonomousDatabaseIsNeverTheNewestOracle holds that a bare oracle
// names the newest ordinary release. The release of an Autonomous Database
// image is the number of the image, which sorts above every database.
func TestAnAutonomousDatabaseIsNeverTheNewestOracle(t *testing.T) {
	t.Parallel()
	got := newestOf([]target{
		{Name: "oracle-26ai", Release: "23.26.3"},
		{Name: "oracle-adb-19c", Release: "26.9.4.2"},
	})
	if got.Name != "oracle-26ai" {
		t.Errorf("the newest Oracle is %s, and want oracle-26ai", got.Name)
	}
}
