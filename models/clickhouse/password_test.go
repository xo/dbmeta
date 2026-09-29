package clickhouse

import (
	"testing"

	"github.com/xo/dbmeta"
)

// TestChangePassword checks the statement text. The escaping is proved
// against a real server in the test module, which logs in with each password
// it sets.
func TestChangePassword(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name string
		in   dbmeta.PasswordChange
		want string
	}{
		{"plain", dbmeta.PasswordChange{User: "bob", Password: "hunter2"},
			"ALTER USER `bob` IDENTIFIED BY 'hunter2'"},
		// A backslash always escapes in a ClickHouse string literal.
		{"a quote and a backslash", dbmeta.PasswordChange{User: "bob", Password: `a'b\`},
			"ALTER USER `bob` IDENTIFIED BY 'a''b\\\\'"},
		// And inside backticks, so a name that ends in a backslash would
		// escape its own closing backtick if the backslash were not doubled.
		{"a backslash in the name", dbmeta.PasswordChange{User: `bo\`, Password: "p"},
			"ALTER USER `bo\\\\` IDENTIFIED BY 'p'"},
		{"a backtick in the name", dbmeta.PasswordChange{User: "b`ob", Password: "p"},
			"ALTER USER `b``ob` IDENTIFIED BY 'p'"},
	} {
		got, err := dbmeta.ClickHouse.ChangePassword(c.in, dbmeta.Quoting{})
		if err != nil {
			t.Errorf("%s: %v", c.name, err)
			continue
		}
		if got != c.want {
			t.Errorf("%s: got %s, want %s", c.name, got, c.want)
		}
	}
}
