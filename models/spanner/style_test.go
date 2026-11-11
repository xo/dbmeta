package spanner

import (
	"testing"

	"github.com/xo/dbmeta/dump"
)

func TestANumericIsReadAsText(t *testing.T) {
	t.Parallel()
	got, ok := selectNumeric(nil, "", dump.Column{Type: dump.Type{Kind: dump.Decimal}}, "t.`amount`")
	if !ok || got != "CAST(t.`amount` AS STRING)" {
		t.Errorf("a numeric is read as %q, %v", got, ok)
	}
	if _, ok := selectNumeric(nil, "", dump.Column{Type: dump.Type{Kind: dump.String}}, "x"); ok {
		t.Error("a string is read as it is")
	}
}
