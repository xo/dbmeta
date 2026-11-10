package dump

import "testing"

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
