// Package spannerdsn turns the URL of a Spanner credential file into the DSN
// of go-sql-spanner, for the tests.
//
// The credential files that dbsetup wrote hold the form that dburl v0.50.0
// reads, which the driver of dbimp takes as it is. The tests still open
// go-sql-spanner, because the driver of dbimp and go-sql-spanner both register
// the name spanner, so no one binary holds both, and Spanner Omni speaks gRPC
// only, which the driver of dbimp does not. See D229.
package spannerdsn

import (
	"net/url"
	"strings"
)

// ErrInvalid is the error for a URL that is not the form of a credential
// file, such as a path that does not name a project, an instance and a database.
// It names no part of the URL, because the URL can hold a path to a key.
const ErrInvalid Error = "the Spanner URL is not valid"

// Error is an error.
type Error string

// Error satisfies the error interface.
func (err Error) Error() string {
	return string(err)
}

// FromURL returns the DSN of go-sql-spanner for a URL of the form
// spanner://host:port/project/instance/database?credential_file=/path/key.json.
// The host and the port are optional, and the key file becomes the property
// credentials. A URL that holds no key file gives a DSN with none, which is what
// Spanner Omni and the emulator need.
func FromURL(raw string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Scheme != "spanner" {
		return "", ErrInvalid
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		return "", ErrInvalid
	}
	var b strings.Builder
	if u.Host != "" {
		b.WriteString(u.Host + "/")
	}
	b.WriteString("projects/" + parts[0] + "/instances/" + parts[1] + "/databases/" + parts[2])
	if key := u.Query().Get("credential_file"); key != "" {
		b.WriteString(";credentials=" + key)
	}
	return b.String(), nil
}
