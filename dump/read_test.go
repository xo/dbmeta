package dump

import "testing"

func TestStripCastsTakesOffTheCastsOfTheTypeOfTheColumn(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct{ in, typ, want string }{
		{`CAST((a + b) AS INTEGER)`, "INTEGER", `(a + b)`},
		{`CAST(CAST((budget / x) AS DECIMAL(14, 2)) AS DECIMAL(14, 2))`, "DECIMAL(14,2)", `(budget / x)`},
		{`CAST(a AS INTEGER) + CAST(b AS INTEGER)`, "INTEGER", `CAST(a AS INTEGER) + CAST(b AS INTEGER)`},
		{`CAST(a AS BIGINT)`, "INTEGER", `CAST(a AS BIGINT)`},
	} {
		if got := stripCasts(tt.in, tt.typ); got != tt.want {
			t.Errorf("stripCasts(%q, %q) = %q, want %q", tt.in, tt.typ, got, tt.want)
		}
	}
}
