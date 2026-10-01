package container

import "net/url"

// Principal is one user that a test reaches a server as.
//
// Every server has its administrator, which is the user [Server.DSN] holds.
// A server whose setup creates an ordinary user lists that one too, so that a
// consumer tests as both without writing the ordinary user's connection
// string itself. Couchbase is the one today. See D102.
type Principal struct {
	// Role is "administrator" for the user of the server's own DSN, and
	// "user" for an ordinary user that the server's setup creates.
	Role string
	// User is the name the principal logs in with.
	User string

	dsn func(port int) string
	url func(port int) string
	api func(port int) string
}

// Principal roles.
const (
	// Administrator is the role of the user the server's DSN holds.
	Administrator = "administrator"
	// User is the role of an ordinary user the server's setup creates.
	User = "user"
)

// DSN returns the connection string for this principal on port at 127.0.0.1,
// in the form the Go driver takes.
func (p Principal) DSN(port int) string { return p.dsn(port) }

// API returns the address of the HTTP API for this principal on port, and is
// empty where the server sets none. See [Server.API].
func (p Principal) API(port int) string {
	return apiOf(p.api, p.dsn, port)
}

// URL returns the dburl style URL for this principal on port.
func (p Principal) URL(port int) string {
	if p.url != nil {
		return p.url(port)
	}
	return p.dsn(port)
}

// Principals returns every principal of the server, the administrator first.
func (s Server) Principals() []Principal {
	admin := Principal{Role: Administrator, User: userOf(s.URL(0)), dsn: s.dsn, url: s.url, api: s.api}
	return append([]Principal{admin}, s.users...)
}

// userOf reads the user name from a URL, and is empty when it has none.
func userOf(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.User == nil {
		return ""
	}
	return u.User.Username()
}
