package dbmeta

import "testing"

// TestNullAsEmpty checks that NULL reads as the empty string and that every
// other value reads as its text.
func TestNullAsEmpty(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		in   any
		want string
	}{
		{nil, ""},
		{"a", "a"},
		{[]byte("b"), "b"},
		{int64(7), "7"},
	} {
		s := "unset"
		if err := NullAsEmpty(&s).Scan(c.in); err != nil {
			t.Errorf("%v: %v", c.in, err)
			continue
		}
		if s != c.want {
			t.Errorf("%v: got %q, want %q", c.in, s, c.want)
		}
	}
}
