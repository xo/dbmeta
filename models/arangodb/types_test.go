package arangodb

import (
	"testing"

	"github.com/xo/dbmeta/dump"
)

func TestTheTypesOfArangoDBAreRead(t *testing.T) {
	t.Parallel()
	for text, want := range map[string]dump.Kind{
		"string":          dump.String,
		"integer":         dump.Integer,
		"number":          dump.Float,
		"boolean":         dump.Boolean,
		"object":          dump.JSON,
		"string, integer": dump.Other,
		"":                dump.Other,
	} {
		got, err := parseType(text)
		if err != nil || got.Kind != want {
			t.Errorf("parseType(%q) is %v, %v, and want %s", text, got.Kind, err, want)
		}
	}
}

func TestAnAQLStatementReadsEveryDocument(t *testing.T) {
	t.Parallel()
	got := selectAll("`book`", []string{"`a`", "`b`"})
	if want := "FOR d IN `book` RETURN {`a`: d.`a`, `b`: d.`b`}"; got != want {
		t.Errorf("selectAll is %q, and want %q", got, want)
	}
}
