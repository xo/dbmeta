package tablestore

import (
	"testing"

	"github.com/xo/dbmeta/dump"
)

func TestTheTypesOfTablestoreAreRead(t *testing.T) {
	t.Parallel()
	for text, want := range map[string]dump.Kind{
		"bigint(20)":    dump.Integer,
		"mediumtext":    dump.String,
		"varchar(1024)": dump.String,
		"double":        dump.Float,
		"blob":          dump.Binary,
		"datetime":      dump.LocalTimestamp,
		"geometry":      dump.Other,
	} {
		got, err := parseType(text)
		if err != nil || got.Kind != want {
			t.Errorf("parseType(%q) is %v, %v, and want %s", text, got.Kind, err, want)
		}
	}
	if got, _ := parseType("varchar(1024)"); got.Length != 1024 {
		t.Errorf("varchar(1024) has length %d", got.Length)
	}
}
