package test

import (
	"strings"
	"testing"

	"github.com/xo/dburl"
)

// hostedDriverDSN returns what sql.Open takes for the URL of a credential file.
// The URL holds a secret, so a fault names no part of it. See D117.
func hostedDriverDSN(t *testing.T, url string) string {
	t.Helper()
	u, err := dburl.Parse(strings.TrimSpace(url))
	if err != nil || u.DSN == "" {
		t.Fatal("the credential file does not hold a URL that dburl reads")
	}
	return u.DSN
}
