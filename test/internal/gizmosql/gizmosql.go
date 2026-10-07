// Package gizmosql opens a session on a GizmoSQL server for the tests.
//
// GizmoSQL gives a session at the Flight SQL handshake and refuses every call
// that does not carry it, with "No session ID in request context". The
// flightsql driver that dburl names never makes the handshake. It sends the
// user and the password on each call, so a connection opens and every
// statement fails. usql's driver for GizmoSQL is in its bad group for this
// reason, and D118 records the fault. The fix belongs to the driver.
//
// The driver does take a token in the DSN. So [DSN] makes the handshake
// itself, with the same Arrow Flight client that the driver uses, and returns
// the DSN with the token the server issued. Nothing else about the driver
// changes. See D187.
package gizmosql

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/apache/arrow-go/v18/arrow/flight/flightsql"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

// DSN makes the handshake that the flightsql driver does not make, and
// returns dsn with the session token in place of the user and the password.
// The DSN has the form of the one dbrun prints: flightsql://user:password@host:port.
func DSN(ctx context.Context, dsn string) (string, error) {
	u, err := url.Parse(dsn)
	if err != nil {
		return "", fmt.Errorf("parsing the dsn: %w", err)
	}
	password, _ := u.User.Password()
	client, err := flightsql.NewClient(u.Host, nil, nil,
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return "", fmt.Errorf("connecting to %s: %w", u.Host, err)
	}
	defer client.Close()
	authed, err := client.Client.AuthenticateBasicToken(ctx, u.User.Username(), password)
	if err != nil {
		return "", fmt.Errorf("making the handshake with %s: %w", u.Host, err)
	}
	md, _ := metadata.FromOutgoingContext(authed)
	values := md.Get("authorization")
	if len(values) == 0 {
		return "", errors.New("reading the token: the server issued none")
	}
	q := url.Values{"token": {strings.TrimPrefix(values[0], "Bearer ")}}
	return (&url.URL{Scheme: u.Scheme, Host: u.Host, RawQuery: q.Encode()}).String(), nil
}
