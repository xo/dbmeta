# D246. Oracle Autonomous Database Free is a Verified release

Status: Decided.

Ken asked for Oracle Autonomous Database Free so that a driver can test the TLS
path and the flags that Autonomous Database sets. The oracle session needs it
to test XMLType, which comes back NULL there.

The image is `container-registry.oracle.com/database/adb-free`. It needs no
login. It has two releases that matter. The tag `26.9.4.2` is a 19c database,
and `26.9.4.1-26ai` is 26ai. The number in the tag is the number of the image
and not of the database, so the release sorts above 26ai even though it is
older. The servers are named `oracle-adb-19c` and `oracle-adb-26ai`.

## Facts measured

- Both releases are Verified. The image needs more than a runner gives it, and
  the point of it is a driver test on a development machine.
- A bare `oracle` must name the newest ordinary release. `newestOf` in
  `test/cmd/dbrun/target.go` skips an Autonomous Database, because its release
  wins otherwise.
- The image refuses `container.Password` because it is 10 characters long. It
  needs 12 to 30 with an uppercase letter, a lowercase letter and a digit.
  The entry uses the password and `Adb1`.
- It starts without `SYS_ADMIN` and without `/dev/fuse`. The log reports
  `OFS: Failed to mount data` and an `ORA-01031` from the cloud plug job, and
  neither stops the database or the login.
- The user is `ADMIN`. The database is `MYATP` and the service of the low
  group is `myatp_low.adb.oraclecloud.com`.
- Port 1522 is TLS, with a self signed certificate in
  `/u01/app/oracle/wallets/tls_wallet`. Port 1521 is plain TCP. The aliases
  that end in `_tls` in the image's own `tnsnames.ora` name port 1521 as TLS,
  and that is wrong, so the readiness check uses the plain alias.
- The entry publishes 1522 first and 1521 second. dbrun publishes the second
  port on the first host port plus 1000.
- The DSN uses the options of the go-ora that the test module pins, which are
  `ssl=true` and `ssl verify=false`. A driver with other option names changes
  that query and nothing else.
- Starting ORDS is turned off with `START_ORDS=false`, because nothing here
  needs it.

## Not done

No model query has run against these containers, and there is no parity
target for `ADMIN`. The image is there for driver tests.
