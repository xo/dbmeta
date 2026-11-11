package dump

import (
	"testing"
	"time"
)

func TestAnArrayOfPostgreSQLBecomesAList(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]string{
		`{a,"b c",NULL}`:        `["a", "b c", NULL]`,
		`{{1,2},{3,4}}`:         `[["1", "2"], ["3", "4"]]`,
		`{}`:                    `[]`,
		`{"say \"hi\"","a\\b"}`: `["say \"hi\"", "a\\b"]`,
	} {
		got, ok := arrayList(in)
		if !ok || got != want {
			t.Errorf("arrayList(%s) is %q, and want %q", in, got, want)
		}
	}
	if _, ok := arrayList(`{a,b`); ok {
		t.Error("an array with no closing bracket is no array")
	}
}

func TestAnArrayIsAListOnlyForATargetThatReadsLists(t *testing.T) {
	t.Parallel()
	typ := Type{Kind: Array, Elem: &Type{Kind: String}}
	const text = `{a,"b c","d\"e"}`
	got, err := Convert(text, typ, typ)
	if err != nil || got != text {
		t.Errorf("Convert(%s) = %v, %v, want the text as it is", text, got, err)
	}
	got, err = convert(text, typ, true)
	if want := `["a", "b c", "d\"e"]`; err != nil || got != want {
		t.Errorf("convert(%s) for a list = %v, %v, want %s", text, got, err, want)
	}
}

func TestAnIntervalOfADriverIsWrittenAsWords(t *testing.T) {
	t.Parallel()
	type interval struct {
		Days   int32
		Months int32
		Micros int64
	}
	iv := Type{Kind: Interval}
	for _, tt := range []struct {
		in   any
		want string
	}{
		{interval{Days: 3, Months: 14, Micros: -1500000}, "14 months 3 days -1500000 microseconds"},
		{-5 * time.Hour, "0 months 0 days -18000000000 microseconds"},
	} {
		got, err := Convert(tt.in, iv, iv)
		if err != nil || got != tt.want {
			t.Errorf("Convert(%v) = %v, %v, want %q", tt.in, got, err, tt.want)
		}
	}
}
