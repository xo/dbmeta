package duckdb

import (
	"math"
	"testing"
)

func TestAFloatThatIsNotFiniteHasALiteral(t *testing.T) {
	t.Parallel()
	for v, want := range map[float64]string{
		math.NaN():   "'NaN'::DOUBLE",
		math.Inf(1):  "'Infinity'::DOUBLE",
		math.Inf(-1): "'-Infinity'::DOUBLE",
	} {
		got, ok := floatSpecial(v)
		if !ok || got != want {
			t.Errorf("floatSpecial(%v) is %q, and want %q", v, got, want)
		}
	}
}
