package dbmeta

import (
	"database/sql"
	"testing"
)

func TestHasNameIgnoresCaseAndSize(t *testing.T) {
	t.Parallel()
	names := []string{"VARBINARY", "BLOB"}
	for name, want := range map[string]bool{
		"varbinary": true, "VARBINARY(8)": true, "Blob": true, "TEXT": false, "": false, "BINARY": false,
	} {
		if got := hasName(name, names); got != want {
			t.Errorf("hasName(%q) = %t, want %t", name, got, want)
		}
	}
}

// TestFormatBinaryOfADialectWithNoModel holds that a dialect that records
// nothing says false, so that a client leaves the value alone.
func TestFormatBinaryOfADialectWithNoModel(t *testing.T) {
	t.Parallel()
	if s, ok := Dialect("no-such-dialect").FormatBinary(&sql.ColumnType{}, []byte{1}); ok || s != "" {
		t.Errorf("got %q and %t for a dialect with no model", s, ok)
	}
}
