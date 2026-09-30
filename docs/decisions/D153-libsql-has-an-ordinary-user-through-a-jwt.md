# D153. libSQL has an ordinary user through a JWT

Status: Amends D112.

## The decision

D112 gave the libSQL entry one user, because sqld's basic authentication has
one, and a user with fewer rights needs a signed JWT. Ken decided on
2026-10-01 that the entry has an ordinary user that way, for dbimp's libSQL
and Turso driver (dbimp W23).

sqld starts with `SQLD_AUTH_JWT_KEY`, the base64url of a raw 32-byte Ed25519
public key, and without `SQLD_HTTP_AUTH`, because sqld takes basic
authentication and ignores the key when that is set. The key and two tokens
are fixed test values in `container/libsql.go`, the way the password is, and
the private key that signed them is kept nowhere. Each token has the header
`{"alg":"EdDSA","typ":"JWT"}` and no exp. `container.LibSQLAdminToken` has the
claim `{"a":"rw"}`, and `container.LibSQLUserToken` has `{"a":"ro"}`.

Each URL carries its token as the password and names its principal, `admin`
or `dbmeta_user`, as every key and token entry does (D102). sqld reads no
user name, and dbimp's driver sends only the password, as Bearer (dbimp D94).
The ready check sends the administrator's token.

## What was measured

On 0.24.33, on 2026-10-01, a SELECT worked with both tokens, CREATE TABLE
worked with the rw token, and the ro token was refused: "Current session
doesn't not have Write permission to namespace default". A token with a bad
signature and a request with no token were both refused, and both tokens
still worked after a stop and a start.
