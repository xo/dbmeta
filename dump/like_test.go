package dump

import "testing"

func TestLike(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		pattern, s string
		want       bool
	}{
		{"abc", "abc", true},
		{"abc", "abd", false},
		{"a%", "abc", true},
		{"a%", "a", true},
		{"%c", "abc", true},
		{"%b%", "abc", true},
		{"%", "", true},
		{"a_c", "abc", true},
		{"a_c", "ac", false},
		{`a\%c`, "a%c", true},
		{`a\%c`, "abc", false},
		{`a\_c`, "a_c", true},
		{"%_x", "ax", true},
		{"%_x", "x", false},
		{"é%", "été", true},
	} {
		if got := like(tt.pattern, tt.s); got != tt.want {
			t.Errorf("like(%q, %q) = %v, want %v", tt.pattern, tt.s, got, tt.want)
		}
	}
}
