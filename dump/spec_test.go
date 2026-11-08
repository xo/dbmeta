package dump_test

import (
	"errors"
	"reflect"
	"testing"

	"github.com/xo/dbmeta/dump"
)

func TestParseSpec(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		text string
		want dump.Spec
	}{
		{"integer", dump.Spec{Name: "integer"}},
		{"Double  Precision", dump.Spec{Name: "double precision"}},
		{"numeric(10,2)", dump.Spec{Name: "numeric", Args: []int{10, 2}}},
		{"numeric( 10 , 2 )", dump.Spec{Name: "numeric", Args: []int{10, 2}}},
		{"time(3) with time zone", dump.Spec{Name: "time", Args: []int{3}, Rest: "with time zone"}},
		{"decimal(10,2) unsigned zerofill", dump.Spec{Name: "decimal", Args: []int{10, 2}, Rest: "unsigned zerofill"}},
		{"character varying(10)[]", dump.Spec{Name: "character varying", Args: []int{10}, Arrays: 1}},
		{"integer[][]", dump.Spec{Name: "integer", Arrays: 2}},
	} {
		got, err := dump.ParseSpec(tt.text)
		if err != nil {
			t.Errorf("ParseSpec(%q): %v", tt.text, err)
			continue
		}
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("ParseSpec(%q) = %+v, want %+v", tt.text, got, tt.want)
		}
	}
	for _, text := range []string{"numeric(10", "numeric(a)", "varchar(10,)"} {
		if _, err := dump.ParseSpec(text); !errors.Is(err, dump.ErrUnknownType) {
			t.Errorf("ParseSpec(%q) gave %v, want ErrUnknownType", text, err)
		}
	}
}

func TestSpecKey(t *testing.T) {
	t.Parallel()
	s, err := dump.ParseSpec("TIME(3) WITH TIME ZONE")
	if err != nil {
		t.Fatal(err)
	}
	if got := s.Key(); got != "time with time zone" {
		t.Errorf("Key = %q", got)
	}
}
