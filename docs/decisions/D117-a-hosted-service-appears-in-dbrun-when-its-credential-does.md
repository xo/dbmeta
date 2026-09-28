# D117. A hosted service appears in dbrun when its credential does

Status: Decided.

Ken asked on 2026-09-28 for the hosted services that `usql` and dbimp
support to be reachable from `dbrun`: `dsn`, `usql`, `test` and the other
commands connect to the remote end, the secret comes from the environment, a
file, a credential helper or another way, and a service shows up only when its
credential is available. Gemini and DeepSeek were asked for a design, and they
agreed on its core. They differed on the tier, and this follows DeepSeek.

## What a hosted service is

The root package `hosted` names each service as data: its name, its product,
its dialect, the form of its connection string, what its driver reads by
itself, and the container that emulates it, if one does. It holds no secret,
reads no file and runs nothing, the way `container` starts nothing. `dbrun`
resolves the connection string at run time and makes a target of the kind
`hosted` from each one that resolves.

The services are Amazon Athena, Google BigQuery, Azure Cosmos DB, Databricks,
Amazon DynamoDB, Alibaba Cloud MaxCompute, Neon, PlanetScale, Amazon Redshift,
Snowflake, Google Cloud Spanner and Alibaba Cloud Tablestore. Fauna is not
there, because it shut down on 2025-05-30. The MongoDB Atlas Data API is not
there, because MongoDB removed it on 2025-09-30. SingleStore is not
there either. Its development image runs locally, and Ken chose on
2026-09-28 to give it no entry at all (D118).

## Where the connection string comes from

`dbrun` reads the first of these that has one:

1. The variable `DBMETA_<NAME>_DSN`, such as `DBMETA_SNOWFLAKE_DSN`.
2. The file `<name>` in `$XDG_CONFIG_HOME/dbmeta/credentials`. `dbrun`
   refuses a file that anybody but its owner can read, and says to run
   `chmod 600`.
3. The helper `dbmeta-credential-<name>` on the path, which prints the
   connection string, the way a git or docker credential helper does. What
   it writes to its error stream is never shown.

The connection string can hold no secret, when the driver reads one by
itself, such as a key file named by `GOOGLE_APPLICATION_CREDENTIALS`.
`hosted.Service.Native` says which. A place that has a connection string and
cannot be used, such as a file others can read, is reported once, and the
service stays absent.

A person provisions every credential. No agent enters one.

## What each command does

- `list`, `status` and every selector include a service only while its
  connection string resolves. `status` shows where it came from, as
  `(hosted, env DBMETA_NEON_DSN)`, and never the secret.
- `dsn` masks the secret: a password, a user name that is a key, as Cosmos DB
  has, and each query parameter whose name holds password, secret, token,
  key or credential. A connection string that is not a URL is masked whole.
  `dsn --reveal` prints it whole.
- `usql` writes the connection string to a usql configuration file that only
  its owner can read, as a named connection, and starts `usql --config` on
  the name, so that no process list shows the secret. The file is removed
  when usql ends.
- `test` sets `DBMETA_<NAME>` to the connection string in the environment of
  the tests. The variable is named for the service and not for its dialect,
  so that Redshift and Neon do not take `DBMETA_POSTGRES`.
- `version` connects through the driver of the dialect, where the test module
  has one.
- `start`, `stop`, `remove` and `logs` have nothing to act on.

`status` does not probe the service. A probe needs the service's driver, and
the drivers of Snowflake, BigQuery, the AWS SDK and the rest are not in the
test module. Adding them is a separate decision.

## The tier

No hosted service is in a tier CI runs. A person with an account runs it
before a release, and CI never does. D119 later split them by whether a model
reads them: Neon, PlanetScale and Redshift are Verified, and the rest are
Staged. Gemini proposed a Hosted tier and a flag that
lists only the services with credentials. DeepSeek said the tiers already
answer it, because CI builds its matrix from the Tested tier, which no hosted
service is in. A fork with no secrets therefore sees nothing new, and a
service can move to Nightly later if Ken gives CI a secret for it.

## What is tested

`test/cmd/dbrun/credential_test.go` tests the order of the three places, the
refusal of a file others can read, a helper that fails without showing what
it wrote, a service that appears only with a credential, and the masking. No
real credential is used. On 2026-09-28 a made-up Neon connection string in
`DBMETA_NEON_DSN` made `neon` appear in `list` and `status`, `dsn` printed it
masked, `dsn --reveal` printed it whole, and the Tested tier did not include
it.
