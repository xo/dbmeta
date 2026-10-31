# D247. oracle-nne-26ai requires native network encryption

Status: Decided.

The oracle session needs a server that refuses a client with weak encryption,
to reproduce go-ora issue 729. A server whose `sqlnet.ora` requires
encryption answers a weak client with ORA-12269.

The release is `oracle-nne-26ai`. It is the image of `oracle-26ai`,
`gvenzl/oracle-free` 23.26.3, and its setup appends four lines to the
`sqlnet.ora` of the server:

- `SQLNET.ENCRYPTION_SERVER = REQUIRED`
- `SQLNET.ENCRYPTION_TYPES_SERVER = (AES256)`
- `SQLNET.CRYPTO_CHECKSUM_SERVER = REQUIRED`
- `SQLNET.CRYPTO_CHECKSUM_TYPES_SERVER = (SHA256)`

## Facts measured

- The setup runs after the server is ready, on every start, and adds the
  lines once. The server reads the file when a session connects, so it needs
  no restart.
- SQL*Plus inside the container reports the AES256 and SHA256 services in
  `V$SESSION_CONNECT_INFO`, so the requirement is in force.
- The go-ora that the test module pins connects to it and reads the version.
  The fault of issue 729 needs another client than that one.
- The release is Verified. CI never runs it.
- A bare `oracle` names `oracle-26ai`. `newestOf` in `test/cmd/dbrun/target.go`
  skips a name that contains `-adb-` or `-nne-`, because this release has the
  release number of 26ai and sorted after it.
- The name contains `-nne-`, so a CI job that runs every `oracle-*` release
  must exclude it, as it does `-adb-`.
- The new release moved the host port of every server after it. The two
  Autonomous Database containers were removed and started again, and they
  have new ports.
