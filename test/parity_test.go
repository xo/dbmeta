package test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/xo/dbmeta"
	arfixture "github.com/xo/dbmeta/models/arangodb/fixture"
	avfixture "github.com/xo/dbmeta/models/avatica/fixture"
	cafixture "github.com/xo/dbmeta/models/cassandra/fixture"
	chfixture "github.com/xo/dbmeta/models/clickhouse/fixture"
	cbfixture "github.com/xo/dbmeta/models/couchbase/fixture"
	crfixture "github.com/xo/dbmeta/models/cratedb/fixture"
	dbfixture "github.com/xo/dbmeta/models/databend/fixture"
	dlfixture "github.com/xo/dbmeta/models/drill/fixture"
	drfixture "github.com/xo/dbmeta/models/druid/fixture"
	esfixture "github.com/xo/dbmeta/models/elasticsearch/fixture"
	exfixture "github.com/xo/dbmeta/models/exasol/fixture"
	fbfixture "github.com/xo/dbmeta/models/firebird/fixture"
	hafixture "github.com/xo/dbmeta/models/hana/fixture"
	hvfixture "github.com/xo/dbmeta/models/hive/fixture"
	iqfixture "github.com/xo/dbmeta/models/influxql/fixture"
	lsfixture "github.com/xo/dbmeta/models/libsql/fixture"
	myfixture "github.com/xo/dbmeta/models/mysql/fixture"
	njfixture "github.com/xo/dbmeta/models/neo4j/fixture"
	osfixture "github.com/xo/dbmeta/models/opensearch/fixture"
	orfixture "github.com/xo/dbmeta/models/oracle/fixture"
	pgfixture "github.com/xo/dbmeta/models/postgres/fixture"
	prfixture "github.com/xo/dbmeta/models/presto/fixture"
	qdfixture "github.com/xo/dbmeta/models/questdb/fixture"
	rsfixture "github.com/xo/dbmeta/models/redshift/fixture"
	rqfixture "github.com/xo/dbmeta/models/rqlite/fixture"
	ssfixture "github.com/xo/dbmeta/models/singlestore/fixture"
	sffixture "github.com/xo/dbmeta/models/snowflake/fixture"
	slfixture "github.com/xo/dbmeta/models/solr/fixture"
	msfixture "github.com/xo/dbmeta/models/sqlserver/fixture"
	srfixture "github.com/xo/dbmeta/models/surrealdb/fixture"
	tdfixture "github.com/xo/dbmeta/models/tidb/fixture"
	trfixture "github.com/xo/dbmeta/models/trino/fixture"
	vefixture "github.com/xo/dbmeta/models/vertica/fixture"
	ydfixture "github.com/xo/dbmeta/models/ydb/fixture"
)

// The privilege parity test, which D61 requires of every dialect.
//
// Every query is asked twice of the same objects: once as the administrator
// the other tests use, and once as an ordinary principal. A query that
// answers differently is reporting a fact about the connection rather than
// about the database, and a consumer has to be told which ones do that.
//
// Oracle is why this exists. Its ALL_ views show the caller only what the
// caller can see, which was known, but nothing had measured whether the same
// thing happens elsewhere. It does: PostgreSQL filters pg_stats and
// pg_settings by role, and information_schema filters everywhere.
//
// # A principal is not one thing
//
// SQL Server has three and they are not interchangeable. A sysadmin. A server
// login mapped to a database user, which is the ordinary model. And a
// contained database user, which has a password in the database itself and no
// login at the server, and which needs CONTAINMENT set to PARTIAL.
//
// Oracle has the same three from 12c. SYSTEM is the administrator. A common
// user exists in the container database and in every pluggable database at
// once, which is what a server login is. A local user authenticates against
// one pluggable database and has nothing above it, which is what a contained
// database user is.
//
// PostgreSQL has no containment, because a role belongs to the cluster and
// not to a database. The nearest three are the superuser, the owner of the
// objects, and a role holding only grants.
//
// MySQL and MariaDB have no containment either. A user is a name and a host
// at server level and a database is only a grant scope, so there are two.
//
// Cassandra has no containment either, and for the same reason as MySQL: a
// role belongs to the cluster and a keyspace is only a grant scope.
//
// SQLite and DuckDB have no user, no role and no grant, so there is nothing
// to compare and this test cannot cover them.
//
// # Why the expectation is checked in
//
// A difference is not a failure. A principal with no privilege on another
// schema has no business seeing it. The file records which queries differ so
// that a change in the set is what fails, the same way conformance.txt works.

// parityGolden is the checked in expectation.
const parityGolden = "testdata/parity.txt"

// parityHeader is written at the top of that file.
const parityHeader = `# What each query answers for a principal that is not the administrator.
#
# Written by go test -update. A line is a query that answered differently for
# the principal named in the section. An empty section means every query gave
# the administrator's answer, which is what a principal owning the objects
# ought to get.
#
# A section is <database>/<scene>/<principal>. The scene is the database the
# comparison ran in, because a contained user needs one of its own.
`

// parityPassword is what every principal is created with. SQL Server enforces
// complexity, so it has an uppercase letter, a digit and a symbol.
const parityPassword = "P4ssw0rd!x"

// parityTarget is one database, its scenes, and the principals in each.
type parityTarget struct {
	// dialect is what the model is registered under, and it names the section
	// in the golden file. There is no separate name field: the two were the
	// same string in every target.
	dialect dbmeta.Dialect
	// driver is the database/sql driver, which is not always the dialect.
	// PostgreSQL is read through pgx, and the two coincide for Cassandra,
	// whose dialect is cassandra because that is the name the driver registers.
	driver string
	env    string
	// open returns the administrator connection, or skips.
	open func(*testing.T) *sql.DB
	// build runs the fixture on a connection and returns the meta.
	build func(*testing.T, *sql.DB) *dbmeta.Meta
	// schema is where the fixture built its objects.
	schema string
	scenes []parityScene
}

// parityScene is one database the comparison runs in.
//
// Most products need only the database the administrator already connects to.
// A SQL Server contained user needs one of its own, because containment
// cannot be set on master.
type parityScene struct {
	name string
	// prepare makes the database and returns the DSN to reach it. A nil
	// prepare means the administrator's own DSN.
	prepare func(t *testing.T, admin *sql.DB, adminDSN string) string
	// min is the release the scene needs, zero when every release has it.
	// A server older than it is skipped rather than failed, the same way a
	// fixture step the server is too old for is skipped. A kind of principal
	// that a release does not have is not a gap in coverage.
	min        dbmeta.Version
	principals []parityPrincipal
}

// parityPrincipal is one non-administrator user.
type parityPrincipal struct {
	name string
	// make creates the principal in the scene and returns its DSN.
	make func(t *testing.T, scene *sql.DB, sceneDSN, schema string) string
}

func parityTargets() []parityTarget {
	return []parityTarget{
		{
			dialect: dbmeta.PostgreSQL, driver: "pgx", env: "DBMETA_POSTGRES",
			open: open, build: setup, schema: pgfixture.Everything.Schema,
			scenes: []parityScene{{
				name: "same",
				principals: []parityPrincipal{
					{name: "owner", make: makePostgresOwner},
					{name: "grantee", make: makePostgresGrantee},
				},
			}},
		},
		{
			// CockroachDB has no containment either. The owner and the
			// grantee are PostgreSQL's two (D123).
			dialect: dbmeta.CockroachDB, driver: "pgx", env: "DBMETA_COCKROACHDB",
			open: openCockroachDB, build: setupCockroachDB, schema: pgFamilies[1].fixture.Schema,
			scenes: []parityScene{{
				name: "same",
				principals: []parityPrincipal{
					{name: "owner", make: makeCockroachDBOwner},
					{name: "grantee", make: makeCockroachDBGrantee},
				},
			}},
		},
		{
			dialect: dbmeta.MySQL, driver: "mysql", env: "DBMETA_MYSQL",
			open: openMySQL, build: setupMySQL, schema: myfixture.Everything.Schema,
			scenes: []parityScene{{
				name:       "same",
				principals: []parityPrincipal{{name: "grantee", make: makeMySQLGrantee}},
			}},
		},
		{
			dialect: dbmeta.TiDB, driver: "mysql", env: "DBMETA_TIDB",
			open: openTiDB, build: setupTiDB, schema: tdfixture.Everything.Schema,
			scenes: []parityScene{{
				// TiDB has no containment either, and the same two
				// lesser principals as MySQL: one with every privilege on
				// the schema, and one that can only read it.
				name: "same",
				principals: []parityPrincipal{
					{name: "grantee", make: makeTiDBGrantee},
					{name: "reader", make: makeTiDBReader},
				},
			}},
		},
		{
			dialect: dbmeta.MemSQL, driver: "mysql", env: "DBMETA_MEMSQL",
			open: openSingleStore, build: setupSingleStore, schema: ssfixture.Everything.Schema,
			scenes: []parityScene{{
				// SingleStore has no containment, and takes MySQL's
				// grants, so the principals are TiDB's.
				name: "same",
				principals: []parityPrincipal{
					{name: "grantee", make: makeTiDBGrantee},
					{name: "reader", make: makeTiDBReader},
				},
			}},
		},
		{
			dialect: dbmeta.SQLServer, driver: "sqlserver", env: "DBMETA_SQLSERVER",
			open: openSQLServer, build: setupSQLServer, schema: msfixture.Everything.Schema,
			scenes: []parityScene{
				{
					name:       "same",
					principals: []parityPrincipal{{name: "login", make: makeSQLServerLogin}},
				},
				{
					// A contained database arrived in SQL Server 2012, which
					// is 11.0. 2008 R2 has no such thing: sp_configure has no
					// 'contained database authentication' option and refuses
					// the name outright.
					name:       "contained",
					min:        dbmeta.V(11),
					prepare:    prepareSQLServerContained,
					principals: []parityPrincipal{{name: "contained", make: makeSQLServerContained}},
				},
			},
		},
		{
			dialect: dbmeta.Presto, driver: "trino", env: "DBMETA_PRESTO",
			open: openPresto, build: setupPresto, schema: prfixture.Everything.Schema,
			scenes: []parityScene{{
				// The same shape as Trino. Presto has no containment and no
				// users, and its memory connector implements no roles at all,
				// so a principal is whatever the client says it is.
				name:       "same",
				principals: []parityPrincipal{{name: "other", make: makePrestoPrincipal}},
			}},
		},
		{
			dialect: dbmeta.Trino, driver: "trino", env: "DBMETA_TRINO",
			open: openTrino, build: setupTrino, schema: trfixture.Everything.Schema,
			scenes: []parityScene{{
				// Trino has no containment and no users. A catalog is a grant
				// scope for a connector that implements roles, the memory
				// connector implements none, and a principal is whatever the
				// client says it is.
				name:       "same",
				principals: []parityPrincipal{{name: "other", make: makeTrinoPrincipal}},
			}},
		},
		{
			dialect: dbmeta.Cassandra, driver: "cassandra", env: "DBMETA_CASSANDRA",
			open: openCassandra, build: setupCassandra,
			schema: cafixture.Everything.Schema,
			scenes: []parityScene{{
				// Cassandra has no containment. A role belongs to the
				// cluster and a keyspace is only a grant scope, so there is
				// the superuser and there is everybody else.
				name: "same",
				principals: []parityPrincipal{
					{name: "grantee", make: makeCassandraGrantee},
					{name: "user", make: makeCassandraUser},
				},
			}},
		},
		{
			dialect: dbmeta.Couchbase, driver: "couchbase", env: "DBMETA_COUCHBASE",
			open: openCouchbase, build: setupCouchbase,
			schema: cbfixture.Everything.Schema,
			scenes: []parityScene{{
				// Couchbase has no containment. A user belongs to the
				// cluster and holds roles on buckets, scopes and
				// collections. SQL++ cannot create a user, so the lesser
				// principal is the ordinary user the dbrun setup makes,
				// with select, insert, update and delete on the bucket and
				// the system catalog role.
				name:       "same",
				principals: []parityPrincipal{{name: "user", make: makeCouchbaseUser}},
			}},
		},
		{
			dialect: dbmeta.Neo4j, driver: "neo4j", env: "DBMETA_NEO4J",
			open: openNeo4j, build: setupNeo4j,
			schema: njfixture.Everything.Schema,
			scenes: []parityScene{{
				// Neo4j has no containment. A user belongs to the server and
				// holds roles, and a role holds privileges on graphs. The
				// lesser principal is the ordinary user the dbrun setup
				// makes, with the role publisher, which reads and writes and
				// cannot manage the server. The fixture also gives it the
				// role dbmeta_reader.
				name:       "same",
				principals: []parityPrincipal{{name: "user", make: makeNeo4jUser}},
			}},
		},
		{
			dialect: dbmeta.ArangoDB, driver: "arangodb", env: "DBMETA_ARANGODB",
			open: openArangoDB, build: setupArangoDB,
			schema: arfixture.Everything.Schema,
			scenes: []parityScene{{
				// ArangoDB has no containment. A user belongs to the
				// server, and a database or a collection is only a grant
				// scope. The user the dbrun setup makes has read and write
				// on the database, and the reader can only read it and has
				// no access to one collection.
				name: "same",
				principals: []parityPrincipal{
					{name: "user", make: makeArangoDBUser},
					{name: "reader", make: makeArangoDBReader},
				},
			}},
		},
		{
			dialect: dbmeta.Drill, driver: "drill", env: "DBMETA_DRILL",
			open: openDrill, build: setupDrill,
			schema: dlfixture.Everything.Schema,
			scenes: []parityScene{{
				// Drill has no containment and no grant that the catalog
				// reads. The administrator is the one user who can change an
				// option or a storage plugin, and the ordinary user of the
				// entry can query and cannot. Drill filters nothing in the
				// catalog by user.
				name: "same",
				principals: []parityPrincipal{
					{name: "user", make: makeDrillUser},
				},
			}},
		},
		{
			dialect: dbmeta.Avatica, driver: "avatica", env: "DBMETA_AVATICA",
			open: openAvatica, build: setupAvatica,
			schema: avfixture.Everything.Schema,
			scenes: []parityScene{{
				// HSQLDB has no containment. A user belongs to the database.
				// The user the dbrun entry makes can read one table in another
				// schema and nothing else, so HSQLDB hides the rest from
				// INFORMATION_SCHEMA. The grantee is a user holding the role
				// dbmeta_reader, which can read the fixture table author, run
				// a procedure and use a sequence.
				name: "same",
				principals: []parityPrincipal{
					{name: "user", make: makeAvaticaUser},
					{name: "grantee", make: makeAvaticaGrantee},
				},
			}},
		},
		{
			dialect: dbmeta.Druid, driver: "druid", env: "DBMETA_DRUID",
			open: openDruid, build: setupDruid,
			schema: drfixture.Everything.Schema,
			scenes: []parityScene{{
				// Druid has no containment. A user belongs to the cluster,
				// and a role grants a permission on a resource. The user the
				// dbrun setup makes can READ every datasource. The reader can
				// READ the datasource author alone, and Druid hides the
				// others from INFORMATION_SCHEMA. Neither can read sys.servers
				// or sys.server_properties, so neither can read the version.
				name: "same",
				principals: []parityPrincipal{
					{name: "user", make: makeDruidUser},
					{name: "reader", make: makeDruidReader},
				},
			}},
		},
		{
			dialect: dbmeta.Elasticsearch, driver: "elasticsearch", env: "DBMETA_ELASTICSEARCH",
			open: openElasticsearch, build: setupElasticsearch,
			schema: esfixture.Everything.Schema,
			scenes: []parityScene{{
				// Elasticsearch has no containment. A user belongs to the
				// cluster, and a role grants a privilege on index names. The
				// user the dbrun entry makes can read the indices whose names
				// start with dbmeta, and has no cluster privilege. The reader
				// can read the index dbmeta_author alone, and Elasticsearch
				// leaves every other index out of its SQL catalog. SELECT
				// version() gives the user HTTP 403, and
				// TestElasticsearchVersionRefusedToAnOrdinaryUser asserts it.
				name: "same",
				principals: []parityPrincipal{
					{name: "user", make: makeElasticsearchUser},
					{name: "reader", make: makeElasticsearchReader},
				},
			}},
		},
		{
			dialect: dbmeta.OpenSearch, driver: "opensearch", env: "DBMETA_OPENSEARCH",
			open: openOpenSearch, build: setupOpenSearch,
			schema: osfixture.Everything.Schema,
			scenes: []parityScene{{
				// OpenSearch has no containment. A user belongs to the
				// cluster, and a role grants a permission on index names. The
				// user the dbrun entry makes can read the indices whose names
				// start with dbmeta and can list every index. The reader can
				// read, list and describe the index dbmeta_author alone, which
				// SHOW TABLES refuses. The lister can list every index and
				// describe none. SELECT version() gives the user HTTP 403 on
				// 2.19.6 and the release on 3.9.0, and
				// TestOpenSearchVersionForAnOrdinaryUser asserts both.
				name: "same",
				principals: []parityPrincipal{
					{name: "user", make: makeOpenSearchUser},
					{name: "reader", make: makeOpenSearchReader},
					{name: "lister", make: makeOpenSearchLister},
				},
			}},
		},
		{
			dialect: dbmeta.Solr, driver: "solr", env: "DBMETA_SOLR",
			open: openSolr, build: setupSolr,
			schema: slfixture.Everything.Schema,
			scenes: []parityScene{{
				// Solr has no containment. A user belongs to the cluster, and
				// security.json gives it roles, and a permission names the
				// roles that can reach a path. The user the dbrun setup makes
				// has the role search, which can read a collection and run
				// SQL on it. It cannot read the Collections API, which is not
				// a statement of the model. SELECT version() gives it HTTP
				// 403, and TestSolrVersionRefusedToAnOrdinaryUser asserts it.
				name:       "same",
				principals: []parityPrincipal{{name: "user", make: makeSolrUser}},
			}},
		},
		{
			dialect: dbmeta.SurrealDB, driver: "surrealdb", env: "DBMETA_SURREALDB",
			open: openSurrealDB, build: setupSurrealDB,
			schema: srfixture.Everything.Schema,
			scenes: []parityScene{{
				// SurrealDB defines a system user on the root, on a
				// namespace or on a database, and gives it one of three
				// roles there: OWNER, EDITOR or VIEWER. The administrator is
				// the OWNER on the root. The user the dbrun setup makes is
				// an EDITOR on the database, and the test makes a VIEWER on
				// the database and an EDITOR on the namespace. A record user
				// signs in through a DEFINE ACCESS rather than as a system
				// user, and the driver signs in system users only (dbimp
				// D51), so it is not here.
				name: "same",
				principals: []parityPrincipal{
					{name: "user", make: makeSurrealDBUser},
					{name: "viewer", make: makeSurrealDBViewer},
					{name: "namespace", make: makeSurrealDBNamespaceUser},
				},
			}},
		},
		{
			dialect: dbmeta.ClickHouse, driver: "clickhouse", env: "DBMETA_CLICKHOUSE",
			open: openClickHouse, build: setupClickHouse,
			schema: chfixture.Everything.Schema,
			scenes: []parityScene{{
				// ClickHouse has no containment. A user belongs to the server
				// and a database is only a grant scope.
				name:       "same",
				principals: []parityPrincipal{{name: "grantee", make: makeClickHouseGrantee}},
			}},
		},
		{
			dialect: dbmeta.YDB, driver: "ydb", env: "DBMETA_YDB",
			open: openYDB, build: setupYDB, schema: ydfixture.Everything.Schema,
			scenes: []parityScene{{
				// YDB has no containment. A user belongs to the cluster and a
				// directory is only a grant scope. The lesser principal is
				// the user the dbrun setup makes, who can read and describe
				// the directory that holds the fixture.
				name:       "same",
				principals: []parityPrincipal{{name: "user", make: makeYDBUser}},
			}},
		},
		{
			dialect: dbmeta.CrateDB, driver: "pgx", env: "DBMETA_CRATEDB",
			open: openCrateDB, build: setupCrateDB, schema: crfixture.Everything.Schema,
			scenes: []parityScene{{
				// CrateDB has no containment, because a user belongs to the
				// cluster, and it has no owner either. The nearest to an
				// owner is a user holding every privilege on the schema.
				name: "same",
				principals: []parityPrincipal{
					{name: "all", make: makeCrateDBHolder("ALL PRIVILEGES")},
					{name: "grantee", make: makeCrateDBHolder("DQL")},
				},
			}},
		},
		{
			dialect: dbmeta.Databend, driver: "databend", env: "DBMETA_DATABEND",
			open: openDatabend, build: setupDatabend, schema: dbfixture.Everything.Schema,
			scenes: []parityScene{{
				// Databend has no containment, so root, a user whose role
				// holds every privilege on the fixture's database, and one
				// whose role can only read it.
				name: "same",
				principals: []parityPrincipal{
					{name: "grantee", make: makeDatabendHolder("ALL")},
					{name: "reader", make: makeDatabendHolder("SELECT")},
				},
			}},
		},
		{
			dialect: dbmeta.Rqlite, driver: "rqlite", env: "DBMETA_RQLITE",
			open: openRqlite, build: setupRqlite, schema: rqfixture.Everything.Schema,
			scenes: []parityScene{{
				// rqlite reads its users from a file and has no grant on a
				// table. The entry declares one user besides the
				// administrator, who can query and execute and nothing else.
				name:       "same",
				principals: []parityPrincipal{{name: "user", make: makeRqliteUser}},
			}},
		},
		{
			dialect: dbmeta.LibSQL, driver: "libsql", env: "DBMETA_LIBSQL",
			open: openLibSQL, build: setupLibSQL, schema: lsfixture.Everything.Schema,
			scenes: []parityScene{{
				// sqld has no grant on a table. A token either writes or
				// only reads, and the entry declares one user besides the
				// administrator, whose token only reads (D153).
				name:       "same",
				principals: []parityPrincipal{{name: "user", make: makeLibSQLUser}},
			}},
		},
		{
			dialect: dbmeta.InfluxQL, driver: "influxdb", env: "DBMETA_INFLUXQL",
			open: openInfluxQL, build: setupInfluxQL, schema: iqfixture.Everything.Schema,
			scenes: []parityScene{{
				// InfluxDB has no containment. A user belongs to the server,
				// and a database is only a grant scope, so there is the
				// administrator and there is a user that can read one
				// database. The entry makes it on InfluxDB 1 and 2, and
				// InfluxDB 3 Core has none (D165).
				name:       "same",
				principals: []parityPrincipal{{name: "user", make: makeInfluxQLUser}},
			}},
		},
		{
			dialect: dbmeta.QuestDB, driver: "pgx", env: "DBMETA_QUESTDB",
			open: openQuestDB, build: setupQuestDB, schema: qdfixture.Everything.Schema,
			scenes: []parityScene{{
				// The open source edition has no roles and no grants. Its
				// PostgreSQL interface has one more user, which can only
				// read, and the entry turns it on.
				name:       "same",
				principals: []parityPrincipal{{name: "read-only", make: makeQuestDBReader}},
			}},
		},
		{
			dialect: dbmeta.Hive, driver: "hive", env: "DBMETA_HIVE",
			open: openHive, build: setupHive, schema: hvfixture.Everything.Schema,
			scenes: []parityScene{{
				// Hive has roles and no users. The image configures no
				// authorization, so a client states a principal and the
				// server takes it, which is the same shape as Trino and
				// Presto. There is nothing to create and nothing to
				// grant, so this target is expected to find no
				// difference at all, and D61 wants that written down
				// either way.
				name:       "same",
				principals: []parityPrincipal{{name: "other", make: makeHivePrincipal}},
			}},
		},
		{
			dialect: dbmeta.HANA, driver: "hdb", env: "DBMETA_HDB",
			open: openHANA, build: setupHANA, schema: hafixture.Everything.Schema,
			scenes: []parityScene{{
				// A HANA connection reaches one tenant database and a user
				// belongs to that tenant, so there is the administrator and
				// there is everybody else. The system database has users of
				// its own and nothing here connects to it, which is the same
				// shape as Oracle's CDB$ROOT.
				name:       "same",
				principals: []parityPrincipal{{name: "grantee", make: makeHANAGrantee}},
			}},
		},
		{
			dialect: dbmeta.Firebird, driver: "firebirdsql", env: "DBMETA_FIREBIRDSQL",
			open: openFirebird, build: setupFirebird, schema: fbfixture.Everything.Schema,
			scenes: []parityScene{{
				// Firebird has no containment, and it cannot: a user lives in
				// the server's security database and a role lives in the
				// database, so a principal that can log in is always the
				// server's. That is two kinds and not three, where SQL Server
				// and Oracle have three.
				name:       "same",
				principals: []parityPrincipal{{name: "grantee", make: makeFirebirdGrantee}},
			}},
		},
		{
			dialect: dbmeta.Exasol, driver: "exasol", env: "DBMETA_EXASOL",
			open: openExasol, build: setupExasol, schema: exfixture.Everything.Schema,
			scenes: []parityScene{{
				// Exasol has no containment. A user belongs to the database
				// and a schema is only a grant scope, so there is SYS, the
				// owner of a schema and a grantee, the same three kinds
				// PostgreSQL has.
				name: "same",
				principals: []parityPrincipal{
					{name: "owner", make: makeExasolOwner},
					{name: "grantee", make: makeExasolGrantee},
				},
			}},
		},
		{
			dialect: dbmeta.Vertica, driver: "vertica", env: "DBMETA_VERTICA",
			open: openVertica, build: setupVertica, schema: vefixture.Everything.Schema,
			scenes: []parityScene{{
				// Vertica has no containment. A user belongs to the database
				// and a schema is only a grant scope, so there is the
				// administrator, the owner of a schema and a grantee. The
				// administrator here is dbmeta, which holds PSEUDOSUPERUSER,
				// because dbadmin has no password and cannot be given one
				// from outside the image.
				name: "same",
				principals: []parityPrincipal{
					{name: "owner", make: makeVerticaOwner},
					{name: "grantee", make: makeVerticaGrantee},
				},
			}},
		},
		{
			// Redshift has no containment. A user belongs to the database,
			// so there is the administrator, the owner of a schema, a
			// grantee and a user with no grant (D182).
			dialect: dbmeta.Redshift, driver: "pgx", env: "DBMETA_REDSHIFT",
			open: openRedshift, build: setupRedshift, schema: rsfixture.Everything.Schema,
			scenes: []parityScene{{
				name: "same",
				principals: []parityPrincipal{
					{name: "owner", make: makeRedshiftOwner},
					{name: "grantee", make: makeRedshiftGrantee},
					{name: "stranger", make: makeRedshiftStranger},
				},
			}},
		},
		{
			// Snowflake has no containment. A user belongs to the account
			// and holds roles, and a role holds the grants. A role that
			// holds USAGE on the schema and SELECT on one table, a role that
			// can use the database and nothing in it, and a role with no
			// grant at all, stand for the lesser principals (D193).
			dialect: dbmeta.Snowflake, driver: "snowflake", env: "DBMETA_SNOWFLAKE",
			open: openSnowflake, build: setupSnowflake, schema: sffixture.Everything.Schema,
			scenes: []parityScene{{
				name: "same",
				principals: []parityPrincipal{
					{name: "grantee", make: makeSnowflakeGrantee},
					{name: "visitor", make: makeSnowflakeVisitor},
					{name: "stranger", make: makeSnowflakeStranger},
				},
			}},
		},
		{
			dialect: dbmeta.Oracle, driver: "oracle", env: "DBMETA_ORACLE",
			open: openOracle, build: setupOracle, schema: orfixture.Everything.Schema,
			scenes: []parityScene{{
				// A common user cannot be made from inside a pluggable
				// database, and nothing connects to CDB$ROOT. That target is
				// recorded as missing in container/oracle.go and this is the
				// second place that needs it.
				name:       "same",
				principals: []parityPrincipal{{name: "local", make: makeOracleLocal}},
			}},
		},
	}
}

// parityExempt names the dialects that have no parity target, with the reason
// each one has none.
//
// A dialect belongs here only when the product has no second principal to be.
// Anything else is a gap, and the point of this map is that closing it has to
// be a deliberate line of code rather than an omission nobody sees.
var parityExempt = map[dbmeta.Dialect]string{
	dbmeta.SQLite3: "a file on disk. It has no user, so there is no second principal to be",
	dbmeta.DuckDB:  "an embedded library. It has no user either",
	dbmeta.GizmoSQL: "the core has one user, admin, and its engine is DuckDB, which has no" +
		" user. The other roles need an identity provider or an enterprise license." +
		" See D118 and D187",
	dbmeta.Impala: "the image configures no authentication, so every user is the same" +
		" principal and nothing can be refused to one. See D146",
	dbmeta.Vitess: "vttestserver starts vtcombo with no authentication and no table rules," +
		" so every user is the same principal and nothing can be refused to one. See D135",
	dbmeta.InfluxDB: "InfluxDB 3 Core has one kind of token, the administrator's, so there is" +
		" no principal with fewer rights to be. See D152",
	// Registered by informationschema_test.go so the shared model can run
	// against a PostgreSQL server. It is not a product and has no server of
	// its own, and the PostgreSQL target measures the same host.
	isDialect: "a test registration of the shared model over PostgreSQL, not a product",
}

// TestEveryDialectIsMeasuredForParity fails when a dialect has neither a
// parity target nor a recorded reason for having none.
//
// D61 makes parity part of finishing a dialect, and this is what holds it to
// that. TestPrivilegeParity cannot: it skips a target whose server is not
// running, and it says nothing at all about a target that was never written.
// Without this test, a dialect added without one passes every test in the
// repository.
func TestEveryDialectIsMeasuredForParity(t *testing.T) {
	t.Parallel()
	measured := make(map[dbmeta.Dialect]bool)
	for _, target := range parityTargets() {
		measured[target.dialect] = true
	}
	for _, d := range dbmeta.Dialects() {
		switch why, exempt := parityExempt[d]; {
		case measured[d] && exempt:
			t.Errorf("%s has a parity target and is also listed as exempt."+
				" Remove it from parityExempt.", d)
		case measured[d]:
		case exempt:
			t.Logf("%s has no parity target: %s", d, why)
		default:
			t.Errorf("%s has no parity target and no reason for having none."+
				" Add one to parityTargets, or add the reason to parityExempt."+
				" See D61.", d)
		}
	}
}

// TestEveryEmbeddedModelSaysSo checks the two directions of
// [dbmeta.Info.Embedded].
//
// A model that is a library must declare it, because dbrun builds its target
// list from the flag and a model that does not declare it does not appear in
// that list. And a model that declares it must be exempt from parity, because
// the reason it is exempt is the reason it is embedded: there is no second
// user to be.
func TestEveryEmbeddedModelSaysSo(t *testing.T) {
	t.Parallel()
	// The libraries, pinned. Without the pin, a refactor that drops the flag
	// removes them from dbrun silently.
	for _, d := range []dbmeta.Dialect{dbmeta.SQLite3, dbmeta.DuckDB} {
		if !d.Embedded() {
			t.Errorf("%s is a library and does not declare Embedded."+
				" dbrun builds its list from that flag. See docs/DIALECT.md.", d)
		}
	}
	for _, d := range dbmeta.Dialects() {
		if !d.Embedded() {
			continue
		}
		if _, exempt := parityExempt[d]; !exempt {
			t.Errorf("%s declares Embedded and is not in parityExempt."+
				" A library has no second user, which is the one reason an"+
				" exemption is allowed. See D61.", d)
		}
	}
}

// TestPrivilegeParity asks every query as the administrator and as each
// principal, and records the queries that answer differently.
//
// Run with -update to rewrite the expectation. Read the diff: a query that
// starts differing has begun depending on who is asking.
func TestPrivilegeParity(t *testing.T) {
	want := readGoldenAt(t, parityGolden)
	var ran int
	for _, target := range parityTargets() {
		t.Run(string(target.dialect), func(t *testing.T) {
			admin := target.open(t)
			adminDSN := dsnOf(t, target.env)
			for _, scene := range target.scenes {
				t.Run(scene.name, func(t *testing.T) {
					// The floor is read from the administrator connection
					// rather than from the meta, because the meta comes from
					// building the fixture in the scene and the scene is what
					// the old server cannot make.
					if !scene.min.IsZero() {
						versions, err := target.dialect.Version(t.Context(), admin)
						if err != nil {
							t.Fatalf("reading the version: %v", err)
						}
						if got := versions.Main(); !got.AtLeast(scene.min) {
							t.Skipf("%s needs %s and the server is %s",
								scene.name, scene.min, got)
						}
					}
					sceneDSN := adminDSN
					sceneDB := admin
					if scene.prepare != nil {
						sceneDSN = scene.prepare(t, admin, adminDSN)
						sceneDB = openAt(t, target.driver, sceneDSN)
					}
					m := target.build(t, sceneDB)
					for _, who := range scene.principals {
						t.Run(who.name, func(t *testing.T) {
							dsn := who.make(t, sceneDB, sceneDSN, target.schema)
							db := openAt(t, target.driver, dsn)
							// The administrator is asked again after the
							// principal exists. Making one changes the
							// catalog: an owner changes who owns the schema
							// and a grantee adds a grant, and a baseline read
							// before that reports the change as a difference
							// in privilege, which it is not.
							baseline := parityRun(t, sceneDB, m, target.schema)
							got := parityReport(baseline, parityRun(t, db, m, target.schema))
							asked := make(map[string]bool, len(baseline))
							for name := range baseline {
								asked[name] = true
							}
							base := parityName(target.dialect, m) +
								"/" + scene.name + "/" + who.name
							section := parityRelease(base, m, want)
							ran++
							if *update {
								writeGoldenAt(t, parityGolden, parityHeader, section, got)
								return
							}
							expected, ok := want[section]
							if !ok {
								t.Fatalf("no expectation for %s in %s."+
									" Run go test -update and read the diff.",
									section, parityGolden)
							}
							compareParity(t, section, expected, got, asked)
						})
					}
				})
			}
		})
	}
	if ran == 0 {
		t.Skip("no database was reachable")
	}
}

// parityFlavors names the dialects that more than one product speaks, and
// the product keys to look for.
//
// Only these are looked up. Cassandra records a version under cql and another
// under protocol, and neither is a product: a read of every key here files
// its answers under cassandra. ScyllaDB is a product, so its key is listed, and
// a Cassandra server, which reports no such key, stays under cassandra.
var parityFlavors = map[dbmeta.Dialect][]string{
	dbmeta.MySQL:     {"mariadb", "mysql"},
	dbmeta.Cassandra: {"scylla"},
}

// parityRelease picks the section this server is recorded under.
//
// Most answers hold for every release of a product and share one section. Some
// do not, and PostgreSQL 12 is why. It grants public SELECT on six columns of
// pg_subscription and not on subsynccommit, which the subscriptions query
// reads as "synchronous", so an ordinary role is refused the whole query. 13
// widened the grant to every column except subconninfo, so the same role is
// served there. A superuser reads it on both, so the query is right and the
// file was wrong to claim one answer covers every release.
//
// MariaDB 10 is the second. information_schema.ROUTINES reports
// routine_definition as NULL to a user that cannot read the routine's source,
// and the functions query selects it as "source", so the grantee sees the
// same rows with an absent definition. MariaDB 11.3 made SHOW CREATE ROUTINE
// a grantable privilege that GRANT ALL PRIVILEGES on a database carries, so
// 13.0 serves the definition to the same principal. 10.6 has no such
// privilege to grant.
//
// CockroachDB is the third. From 26.3 the databases query reports a size, and
// a principal that cannot connect to a database reads "no access" where the
// administrator reads its size. 24.3 and 26.2 have no pg_size_pretty, so the
// size is NULL for everybody and the answers agree.
//
// So a section can be written as product@major, and that one wins for a server
// reporting that major. Everything else falls back to the shared section. The
// override exists only where a release really differs, which keeps the file
// readable and puts the difference where a reader trips over it.
//
// The major alone is the key. A product whose answers differ between two
// releases of one major needs more, and none here does.
func parityRelease(base string, m *dbmeta.Meta, want map[string][]string) string {
	product, rest, _ := strings.Cut(base, "/")
	// A flavor's release is under its own key, and the main version can be
	// another product's number. ScyllaDB reports Cassandra 3.0.8 as its main
	// version, so a section for ScyllaDB 2026 is found by the scylla key.
	main := m.Version().Main()
	if m.Version().Has(product) {
		main = m.Version().Get(product)
	}
	if main.Unknown || len(main.Parts) == 0 {
		return base
	}
	// The major alone is not always a release line. ClickHouse versions by
	// calendar, so 25.3 and 25.8 are both "25" and they do not answer the
	// same: the privilege check on system.named_collections arrived between
	// them. SAP HANA is the same shape, where every release is 2.
	//
	// So the narrower name wins where the file has one. A product whose
	// major does identify a line keeps using it and nothing changes for
	// PostgreSQL, Oracle or SQL Server.
	for _, name := range releaseNames(product, rest, main.Parts) {
		if _, ok := want[name]; ok {
			return name
		}
	}
	return base
}

// releaseNames are the section names for a version, narrowest first.
func releaseNames(product, rest string, parts []uint32) []string {
	major := strconv.FormatUint(uint64(parts[0]), 10)
	names := make([]string, 0, 2)
	if len(parts) > 1 {
		minor := strconv.FormatUint(uint64(parts[1]), 10)
		names = append(names, product+"@"+major+"."+minor+"/"+rest)
	}
	return append(names, product+"@"+major+"/"+rest)
}

// parityName is the database the section is recorded under.
//
// It is the product rather than the dialect. MariaDB and MySQL share a
// dialect and do not share the tables these queries are refused on: mysql.proc
// was removed in MySQL 8.0 and MariaDB still has it, so one file cannot hold
// one answer for both.
func parityName(dialect dbmeta.Dialect, m *dbmeta.Meta) string {
	for _, key := range parityFlavors[dialect] {
		if m.Version().Has(key) {
			return key
		}
	}
	return string(dialect)
}

// openAt connects and closes at the end of the test.
func openAt(t *testing.T, driver, dsn string) *sql.DB {
	t.Helper()
	db, err := sql.Open(driver, dsn)
	if err != nil {
		t.Fatalf("opening %s: %v", driver, err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.PingContext(t.Context()); err != nil {
		t.Fatalf("connecting as the principal: %v", err)
	}
	return db
}

// parityAnswer is what one query returned.
type parityAnswer struct {
	rows int
	// body is every row as text, so a value the server blanked out is a
	// difference and not only a missing row.
	body string
	// err is what the server said, for a query the principal cannot run.
	err string
}

// parityRun asks every supported query and records the answer.
func parityRun(t *testing.T, db *sql.DB, m *dbmeta.Meta, schema string) map[string]parityAnswer {
	t.Helper()
	ctx := t.Context()
	out := map[string]parityAnswer{}
	for _, q := range dbmeta.Queries() {
		if q.Support(m) != dbmeta.Supported {
			continue
		}
		args, err := parityArgs(q, m, schema)
		if err != nil {
			continue
		}
		query, vals, err := q.Build(m, args)
		if walk, ok := parityWalks[q.Name()]; ok && errors.Is(err, dbmeta.ErrSeveralStatements) {
			// A walk has no one statement, so it is asked through its
			// iterator. InfluxQL is the case (D159).
			out[q.Name()] = walk(ctx, m, db, args)
			continue
		}
		if err != nil {
			continue
		}
		out[q.Name()] = parityAsk(ctx, db, query, vals)
	}
	return out
}

// parityArgs keeps only the filter values the query declares, because passing
// one it does not declare is ErrUnknownParam.
func parityArgs(q dbmeta.AnyQuery, m *dbmeta.Meta, schema string) (map[string]any, error) {
	params, err := q.Params(m)
	if err != nil {
		return nil, err
	}
	want := dbmeta.Args{Schema: schema}.Map()
	// Snowflake lists the schemas of one database, and other sessions keep
	// schemas of their own in it, such as the one dbimp measures in. A role
	// with no grant on one reads fewer rows than the administrator, which says
	// nothing about the fixture, so the Schemas query narrows to the fixture.
	if m.Dialect() == dbmeta.Snowflake && q.Name() == dbmeta.Schemas.Name() {
		want["name"] = schema
	}
	out := map[string]any{}
	for _, p := range params {
		if v, ok := want[p.Name]; ok {
			out[p.Name] = v
		}
	}
	return out, nil
}

// parityAsk runs one statement and returns its rows as text.
//
// It reads the result generically rather than through the typed iterator,
// because the comparison is the same for every query and a scan per object
// kind is a second copy of every model.
func parityAsk(ctx context.Context, db *sql.DB, query string, vals []any) parityAnswer {
	rows, err := db.QueryContext(ctx, query, vals...)
	if err != nil {
		return parityAnswer{err: firstLine(err.Error())}
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		return parityAnswer{err: firstLine(err.Error())}
	}
	var out []string
	for rows.Next() {
		cells := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range cells {
			ptrs[i] = &cells[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return parityAnswer{err: firstLine(err.Error())}
		}
		parts := make([]string, len(cells))
		for i, c := range cells {
			if b, ok := c.([]byte); ok {
				c = string(b)
			}
			parts[i] = fmt.Sprintf("%v", c)
		}
		out = append(out, strings.Join(parts, "\x1f"))
	}
	if err := rows.Err(); err != nil {
		return parityAnswer{err: firstLine(err.Error())}
	}
	// Sorted, because a query that orders by a name the principal cannot see
	// can return the same rows in another order.
	sort.Strings(out)
	return parityAnswer{rows: len(out), body: strings.Join(out, "\n")}
}

// firstLine keeps an error to one line and takes the host out of it.
//
// A server that prints a stack of context makes the expectation unreadable,
// and MySQL names the client in the message: "denied to user 'x'@'10.0.0.2'"
// records the address of whichever machine ran the test, which is never the
// same twice. The user is the part that matters and it stays.
func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	s = clientHost.ReplaceAllString(s, "@'client'")
	s = exasolSession.ReplaceAllString(s, "")
	s = ydbAddress.ReplaceAllString(s, "")
	s = ydbStack.ReplaceAllString(s, "")
	return grantColumns.ReplaceAllString(s, "$1 ON")
}

// ydbAddress matches the node ydb-go-sdk names in every error, such as
// ", address = 127.0.0.1:55119". The port is whichever one dbrun gave the
// server, so without this match the refusal never reads the same twice.
var ydbAddress = regexp.MustCompile(`, address = [^,)]+`)

// ydbStack matches the Go call stack ydb-go-sdk adds after an error, which
// begins with " at " and a function in backticks. It names lines of the
// driver's source, which move with every release of the driver.
var ydbStack = regexp.MustCompile(" at `.*$")

// exasolSession matches the session number Exasol ends every message with,
// such as (Session: 1877435836291743744). It is a new number on every
// connection, so without this match the refusal never reads the same twice.
var exasolSession = regexp.MustCompile(` \(Session: [0-9]+\)`)

// clientHost matches the host half of a MySQL user name.
var clientHost = regexp.MustCompile(`@'[^']*'`)

// grantColumns matches the column list ClickHouse puts in a refusal.
//
// 25.8 says "the grant SELECT(database, table, name) ON system.x" and 26.9
// says "the grant SELECT ON system.x" for the same query and the same user.
// The columns are the ones the statement happened to read, so the list moves
// whenever a query changes and differs between releases for no reason a
// reader cares about. The fact is that the query was refused on that table.
var grantColumns = regexp.MustCompile(`\b([A-Z]+)\([^)]*\) ON\b`)

// compareParity is compareReport, ignoring the expectation's lines for a query
// this server never answered.
//
// A query can be gated on a release. Cassandra's settings reads system_views,
// which arrived in 4.0, and PostgreSQL's publications arrived in 10. On an
// older server the query is not asked at all, so it can neither agree nor
// differ, and an expectation written from a newer one names it. Holding that
// against the older server makes the file release specific, which is the
// thing the format is built to avoid.
//
// A line for a query that was asked is compared as usual, in both directions.
func compareParity(t *testing.T, name string, want, got []string, asked map[string]bool) {
	t.Helper()
	kept := make([]string, 0, len(want))
	var skipped []string
	for _, l := range want {
		q, _, _ := strings.Cut(l, " ")
		if !asked[q] {
			skipped = append(skipped, q)
			continue
		}
		kept = append(kept, l)
	}
	if len(skipped) != 0 {
		t.Logf("%s: not asked on this release, so not compared: %s",
			name, strings.Join(skipped, ", "))
	}
	compareReport(t, name, kept, got)
}

// parityReport returns one line per query that answered differently.
func parityReport(admin, other map[string]parityAnswer) []string {
	names := make([]string, 0, len(admin))
	for name := range admin {
		names = append(names, name)
	}
	sort.Strings(names)
	var out []string
	for _, name := range names {
		a, p := admin[name], other[name]
		switch {
		case p.err != "" && a.err == "":
			out = append(out, fmt.Sprintf("%s refused: %s", name, p.err))
		case p.rows < a.rows:
			out = append(out, name+" fewer rows than the administrator")
		case p.rows > a.rows:
			out = append(out, name+" more rows than the administrator")
		case a.body != p.body:
			out = append(out, name+" the same rows with different values")
		}
	}
	return out
}
