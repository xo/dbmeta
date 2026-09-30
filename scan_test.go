package dbmeta

import (
	"database/sql"
	"errors"
	"testing"
)

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

// TestNumberAsBool checks that a bool, a number and the text of either read
// as a bool, and that NULL and anything else are refused.
func TestNumberAsBool(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		in   any
		want bool
	}{
		{true, true},
		{false, false},
		{int64(1), true},
		{int64(0), false},
		{float64(1), true},
		{float64(0), false},
		{"1", true},
		{[]byte("0"), false},
		{"true", true},
	} {
		b := !c.want
		if err := NumberAsBool(&b).Scan(c.in); err != nil {
			t.Errorf("%v: %v", c.in, err)
			continue
		}
		if b != c.want {
			t.Errorf("%v: got %v, want %v", c.in, b, c.want)
		}
	}
	for _, in := range []any{nil, "yes please", struct{}{}} {
		var b bool
		if err := NumberAsBool(&b).Scan(in); !errors.Is(err, ErrInvalidBool) {
			t.Errorf("%v: got %v, want ErrInvalidBool", in, err)
		}
	}
}

// TestNullNumberAsBool checks that NULL reads as absent and a number as a
// bool.
func TestNullNumberAsBool(t *testing.T) {
	t.Parallel()
	b := sql.Null[bool]{V: true, Valid: true}
	if err := NullNumberAsBool(&b).Scan(nil); err != nil || b.Valid {
		t.Errorf("NULL: got %+v, %v", b, err)
	}
	if err := NullNumberAsBool(&b).Scan(float64(1)); err != nil || !b.Valid || !b.V {
		t.Errorf("1: got %+v, %v", b, err)
	}
}
