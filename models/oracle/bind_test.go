package oracle_test

import (
	"testing"

	"github.com/xo/dbmeta"
	_ "github.com/xo/dbmeta/models/oracle"
)

// TestAFlagIsBoundAsANumber checks that no statement binds a Go bool, which
// go-ora/v3 refuses. usql found every query failing on its oracle scheme
// until the flag was bound as 1 or 0. See D136.
func TestAFlagIsBoundAsANumber(t *testing.T) {
	m, err := dbmeta.New(dbmeta.Oracle, dbmeta.VersionSet{})
	if err != nil {
		t.Fatalf("building the meta: %v", err)
	}
	for _, withSystem := range []bool{false, true} {
		for _, q := range dbmeta.Queries() {
			if q.Support(m) != dbmeta.Supported {
				continue
			}
			args := map[string]any{}
			params, err := q.Params(m)
			if err != nil {
				t.Fatalf("%s: reading the parameters: %v", q.Name(), err)
			}
			for _, p := range params {
				if p.Name == "with_system" {
					args[p.Name] = withSystem
				}
			}
			_, vals, err := q.Build(m, args)
			if err != nil {
				t.Fatalf("%s: building: %v", q.Name(), err)
			}
			for i, v := range vals {
				if _, ok := v.(bool); ok {
					t.Errorf("%s: value %d is the bool %v", q.Name(), i+1, v)
				}
			}
		}
	}
}
