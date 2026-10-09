# Decisions

Every decision this project made is a file in this folder, one per decision,
named by its number and its title. This table is the index. Find the number
here, then open the file.

Each file opens with its status. "Decided" means Ken chose it. "Proposed"
means an agent or a peer session suggested it and Ken has not confirmed it.
"Open" means nobody has chosen yet. A decision that changes an earlier one
says so in its status, as "Amends D50", and the earlier one says it back, as
"Amended by D111". Read the status before the decision.
75 of them amend or replace an earlier one, and a decision read without its
amendment is worse than no decision.

A new decision gets the next number and a file of its own. Add its row here.
`TestTheDecisionIndexIsComplete` fails when a decision has no row or a row is
wrong, and it prints the row to add. D111 moved the decisions here from one
file.

| # | Decision | Status |
| --- | --- | --- |
| [D1](D001-the-module-centralizes-database-metadata.md) | The module centralizes database metadata | Decided |
| [D2](D002-models-are-one-package-per-driver-under-models.md) | Models are one package per driver under `models/<driver>` | Amended by D71 |
| [D3](D003-the-root-package-is-the-driver-agnostic-api.md) | The root package is the driver agnostic API | Decided |
| [D4](D004-keep-the-object-coverage-drop-the-reader-naming.md) | Keep the object coverage, drop the Reader naming | Decided |
| [D5](D005-dbmeta-only-reads.md) | dbmeta only reads | Amended by D56 |
| [D6](D006-fix-the-null-scan-defect-once-and-never-hide-a.md) | Fix the NULL scan defect once, and never hide a NULL | Decided, amended in place, D6a superseded by D71 |
| [D7](D007-use-the-standard-library-third-party-packages.md) | Use the standard library. Third party packages are a last resort | Decided |
| [D8](D008-version-differences-are-generated-data-not.md) | Version differences are generated data, not packages | Decided |
| [D9](D009-there-are-two-platonic-models-postgresql-is-the.md) | There are two platonic models. PostgreSQL is the primary one | Decided |
| [D10](D010-take-the-initial-design-from-dbtpl-and-its.md) | Take the initial design from dbtpl and its models directory | Decided |
| [D11](D011-dbtpl-is-pinned-as-a-tool-in-the-generation.md) | dbtpl is pinned as a tool, in the generation module | Amended by D26, superseded by D71 |
| [D12](D012-work-against-live-databases-running-in.md) | Work against live databases running in containers | Amended by D70 and D71 |
| [D13](D013-build-the-models-before-the-root-package.md) | Build the models before the root package | Decided |
| [D14](D014-a-driver-is-a-family-not-a-product.md) | A driver is a family, not a product | Decided |
| [D15](D015-ci-runs-on-github-actions-on-ubuntu-latest-only.md) | CI runs on GitHub Actions, on ubuntu-latest only | Decided |
| [D16](D016-user-facing-text-follows-the-simple-english.md) | User facing text follows the simple English rules | Decided |
| [D17](D017-every-metadata-read-takes-a-context.md) | Every metadata read takes a context | Decided |
| [D18](D018-the-whole-package-is-idiomatic-go.md) | The whole package is idiomatic Go | Decided |
| [D19](D019-do-not-repeat-dburl.md) | Do not repeat dburl | Half decided, half overtaken by the code, amended by D125 |
| [D20](D020-postgresql-goes-back-to-9-6-every-other-database.md) | PostgreSQL goes back to 9.6. Every other database starts at the maintained floor | Decided |
| [D21](D021-drop-a-server-version-on-a-rule-not-on-a.md) | Drop a server version on a rule, not on a judgment | Decided |
| [D22](D022-test-every-supported-major-not-a-sample-of-them.md) | Test every supported major, not a sample of them | Superseded by D24 |
| [D23](D023-do-not-build-dbtest-first-let-dbmeta-pull-it.md) | Do not build dbtest first. Let dbmeta pull it into existence | Decided |
| [D24](D024-ci-tests-the-latest-version-only-the-matrix-runs.md) | CI tests the latest version only. The matrix runs locally | Supersedes D22, superseded by D42 |
| [D25](D025-test-on-amd64-only-no-build-tags-and-no-platform.md) | Test on amd64 only. No build tags and no platform gates | Decided |
| [D26](D026-no-database-driver-in-the-dbmeta-module.md) | No database driver in the dbmeta module | Amends D11, amended by D48 |
| [D27](D027-split-the-work-in-two-a-nested-test-module-here.md) | Split the work in two: a nested test module here, a shared harness in dbtest | Decided |
| [D28](D028-root-tests-use-a-fake-driver-replaying-captured.md) | Root tests use a fake driver replaying captured data | Decided |
| [D29](D029-pure-go-only-no-single-package-imports-every.md) | Pure Go only. No single package imports every driver | Amended by D48 |
| [D30](D030-dbtpl-is-not-used-to-generate-dbmeta.md) | dbtpl is not used to generate dbmeta | Amended by D71 |
| [D31](D031-models-register-from-internal-one-file-each.md) | Models register from internal, one file each, gated by build tags | Decided |
| [D32](D032-errors-are-constants-of-a-string-type.md) | Errors are constants of a string type | Decided |
| [D33](D033-results-stream-the-package-does-not-materialize.md) | Results stream. The package does not materialize them | Decided |
| [D34](D034-report-capabilities-and-return-a-typed-error.md) | Report capabilities, and return a typed error when asked anyway | Decided |
| [D35](D035-duckdb-is-out-of-the-initial-testing-set.md) | DuckDB is out of the initial testing set | Superseded by D48 |
| [D36](D036-the-client-drives-dbmeta-decides-nothing-about.md) | The client drives. dbmeta decides nothing about the connection | Decided |
| [D37](D037-a-version-is-a-list-of-numbers-with-a-name-and.md) | A version is a list of numbers with a name, and there can be several | Decided |
| [D38](D038-dbmeta-holds-the-version-query-and-will-run-it.md) | dbmeta holds the version query, and will run it on request | Amended in place |
| [D39](D039-queries-are-listed-described-and-rendered-for.md) | Queries are listed, described, and rendered for the client to run | Decided |
| [D40](D040-three-support-tiers-and-a-trigger-that-can.md) | Three support tiers, and a trigger that can remove a version | Amended by D119 |
| [D41](D041-every-model-ships-its-fixtures-beside-its.md) | Every model ships its fixtures beside its queries | Decided |
| [D42](D042-four-releases-per-push-every-release-nightly.md) | Four releases per push, every release nightly | Supersedes D24, amended by D69 |
| [D43](D043-ask-several-models-before-a-dialect-is-declared.md) | Ask several models before a dialect is declared finished | Decided |
| [D44](D044-a-version-key-names-the-product-a-number-alone.md) | A version key names the product. A number alone never does | Decided |
| [D45](D045-a-query-may-answer-partially-once-and-must-say.md) | A query can answer partially, once, and must say so | Amended by D161 |
| [D46](D046-five-object-kinds-are-missing-and-two-consumers.md) | Five object kinds are missing, and two consumers say which | Decided |
| [D47](D047-dbmeta-supplies-the-data-the-consumer-decides.md) | dbmeta supplies the data. The consumer decides what to show | Amended by D146 |
| [D48](D048-cgo-is-allowed-in-the-test-module-and-nowhere.md) | cgo is allowed in the test module, and nowhere else | Amends D26 and D29, supersedes D35 |
| [D49](D049-one-method-on-the-interface-and-a-not-null-is.md) | One method on the interface, and a NOT NULL is not a constraint row | Amended by D121 |
| [D50](D050-documentation-lives-in-docs-and-the-decision-log.md) | Documentation lives in docs, and the decision log stays one file | Amended by D110 and D111 |
| [D51](D051-there-is-no-alias-for-a-nullable-type.md) | There is no alias for a nullable type | Decided |
| [D52](D052-a-test-driver-is-the-one-usql-uses-or-it-is-the.md) | A test driver is the one usql uses, or it is the wrong driver | Amended by D59 and D154 |
| [D53](D053-one-canonical-expectation-checked-in-that-every.md) | One canonical expectation, checked in, that every database must meet | Decided |
| [D54](D054-sql-server-covers-every-release-that-ships-a.md) | SQL Server covers every release that ships a Linux container | Amended by D63 |
| [D55](D055-the-current-user-moves-here-changing-a-password.md) | The current user moves here. Changing a password does not | Decided |
| [D56](D056-the-password-statement-is-built-here-and-run-by.md) | The password statement is built here and run by the caller | Amends D5, amended by D127 |
| [D57](D057-a-windows-machine-is-how-a-pre-2017-sql-server.md) | A Windows machine is how a pre 2017 SQL Server gets tested, and it is Verified | Decided |
| [D58](D058-one-gitignore-in-the-repository-root.md) | One .gitignore, in the repository root | Decided |
| [D59](D059-oracle-is-tested-with-go-ora-v2-until-v3-tags.md) | Oracle is tested with go-ora v2 until v3 tags its fix | Amends D52, amended by D136 and D157 |
| [D60](D060-the-oracle-model-reads-all-views-and-there-is-no.md) | The Oracle model reads ALL_ views, and there is no DBA_ variant | Decided |
| [D61](D061-every-dialect-is-measured-against-every.md) | Every dialect is measured against every principal the product has | Amended in place |
| [D62](D062-cql-cannot-compute-so-the-cassandra-model.md) | CQL cannot compute, so the Cassandra model computes in Scan | Amended by D93 and D200 |
| [D63](D063-support-says-when-a-release-is-too-old.md) | Support says when a release is too old | Amends D54 |
| [D64](D064-the-verified-tier-is-checked-against-the.md) | The Verified tier is checked against the document | Decided |
| [D65](D065-a-windows-machine-rearms-its-evaluation-before.md) | A Windows machine rearms its evaluation before it expires | Decided |
| [D66](D066-the-order-the-remaining-dialects-are-written-in.md) | The order the remaining dialects are written in | Amended by D67, D77, D88, D91, D94 and D129 |
| [D67](D067-impala-cannot-be-a-dbmeta-model-and-clickhouse.md) | Impala cannot be a dbmeta model, and ClickHouse goes first | Amends D66, amended by D146 |
| [D68](D068-every-container-is-started-by-the-runner-and.md) | Every container is started by the runner and named product-release | Amended by D70 and D124 |
| [D69](D069-the-workflow-builds-its-matrix-from-the-go-list.md) | The workflow builds its matrix from the Go list | Amends D42 |
| [D70](D070-the-runner-is-a-go-command-called-dbrun.md) | The runner is a Go command called dbrun | Amends D68 and D12 |
| [D71](D071-nothing-here-is-generated-the-models-are-written.md) | Nothing here is generated. The models are written | Amends D2, D12 and D30, supersedes D11 and D6a |
| [D72](D072-trino-reads-system-jdbc-and-a-catalog-is-a-real.md) | Trino reads system.jdbc, and a catalog is a real level | Decided |
| [D73](D073-presto-is-its-own-dialect-and-not-a-flavor-of.md) | Presto is its own dialect, and not a flavor of Trino | Decided |
| [D74](D074-firebird-has-no-schemas-and-none-is-invented.md) | Firebird has no schemas, and none is invented | Decided |
| [D75](D075-every-container-is-bounded-and-four-run-at-once.md) | Every container is bounded, and four run at once | Amended by D98 and D108 |
| [D76](D076-sap-hana-reads-sys-and-answers-more-than.md) | SAP HANA reads SYS, and answers more than anything but PostgreSQL | Decided |
| [D77](D077-exasol-will-not-run-here-and-hive-goes-ahead-of.md) | Exasol will not run here, and Hive goes ahead of it | Amends D66, amended by D84 |
| [D78](D078-hive-reads-sys-and-is-a-model.md) | Hive reads sys, and is a model | Decided |
| [D79](D079-a-dialect-that-cannot-bind-renders-its-values.md) | A dialect that cannot bind renders its values | Decided |
| [D80](D080-the-driver-registry-is-dburl-s-and-reading-it-is.md) | The driver registry is dburl's, and reading it is not importing it | Amended by D125 and D154 |
| [D81](D081-the-cassandra-dialect-is-cql.md) | The Cassandra dialect is cql | Decided, amended by D196 |
| [D82](D082-ci-compiles-once-and-every-job-runs-the-binary.md) | CI compiles once and every job runs the binary | Decided |
| [D83](D083-a-server-is-ready-when-it-can-run-a-query-and.md) | A server is ready when it can run a query, and keeps being able to | Decided |
| [D84](D084-exasol-runs-after-all-on-the-nano-image.md) | Exasol runs after all, on the nano image | Amends D77 |
| [D85](D085-exasol-is-the-nano-containers-and-one-frozen.md) | Exasol is the nano containers and one frozen virtual machine | Decided |
| [D86](D086-a-machine-is-one-list-with-a-spec-for-how-it-is.md) | A machine is one list, with a spec for how it is built | Decided |
| [D87](D087-exasol-is-a-model-read-from-the-exa-all-views.md) | Exasol is a model, read from the EXA_ALL views | Decided |
| [D88](D088-vertica-is-a-model-on-four-community-images.md) | Vertica is a model, on four community images | Amends D66, amended by D100 |
| [D89](D089-agent-skills-are-committed-as-copies-and-one.md) | Agent skills are committed as copies, and one command installs them | Amended by D110 |
| [D90](D090-a-database-qualifies-when-it-is-free-to-run-for.md) | A database qualifies when it is free to run for development and testing | Decided |
| [D91](D091-scylladb-is-a-flavor-of-the-cassandra-model.md) | ScyllaDB is a flavor of the Cassandra model | Amends D66, amended by D92 |
| [D92](D092-a-second-version-statement-reads-the-scylladb.md) | A second version statement reads the ScyllaDB release | Amends D91 |
| [D93](D093-the-cql-tests-use-github-com-xo-cql-which.md) | The cql tests use github.com/xo/cql, which reports a NULL | Amends D62, amended by D196 |
| [D94](D094-couchbase-runs-under-dbrun-and-its-model-waits.md) | Couchbase runs under dbrun, and its model waits for the n1ql rewrite | Amends D66, amended by D95, D96 and D104 |
| [D95](D095-the-couchbase-model-waits-for-the-dbimp-driver.md) | The Couchbase model waits for the dbimp driver | Amends D94, amended by D101 and D104 |
| [D96](D096-couchbase-gets-an-ordinary-user-and-starts-again.md) | Couchbase gets an ordinary user, and starts again after a stop | Amends D94, amended by D104 |
| [D97](D097-dbrun-is-documented-for-its-users-in-dbrun-and.md) | dbrun is documented for its users, in DBRUN and CONTAINERS | Decided |
| [D98](D098-a-server-has-an-owner-and-dbrun-acts-only-on-the.md) | A server has an owner, and dbrun acts only on the caller's own | Amends D75, amended by D102, D108 and D115 |
| [D99](D099-dburl-names-the-product-that-a-scheme-drives.md) | dburl names the product that a scheme drives | Amended by D125 |
| [D100](D100-the-vertica-images-live-in-usql-vertica-and-the.md) | The Vertica images live in usql/vertica, and the older ones wait for admintools | Amends D88 |
| [D101](D101-couchbase-is-the-dialect-couchbase-read-through.md) | Couchbase is the dialect couchbase, read through the dbimp driver | Amends D95 |
| [D102](D102-dbrun-prints-every-principal-of-a-server.md) | dbrun prints every principal of a server | Amends D98 |
| [D103](D103-surrealdb-runs-under-dbrun-for-the-dbimp-driver.md) | SurrealDB runs under dbrun, for the dbimp driver | Amended by D119 |
| [D104](D104-the-couchbase-model-reads-7-6-and-later.md) | The Couchbase model reads 7.6 and later | Amends D94, D95 and D96 |
| [D105](D105-dbrun-runs-a-setup-again-after-a-failure-and.md) | dbrun runs a setup again after a failure, and prints the log of a server that never answered | Amended by D107 |
| [D106](D106-neo4j-enterprise-runs-under-dbrun-under-the.md) | Neo4j Enterprise runs under dbrun, under the evaluation agreement | Amended by D109 and D119 |
| [D107](D107-the-hive-setup-runs-from-a-copy-that-is-safe-to.md) | The Hive setup runs from a copy that is safe to run twice | Amends D105 |
| [D108](D108-eight-servers-run-at-once-and-a-stopped.md) | Eight servers run at once, and a stopped container belongs to nobody | Amends D75 and D98 |
| [D109](D109-neo4j-is-the-dialect-neo4j-and-its-url-names-the.md) | Neo4j is the dialect neo4j, and its URL names the database | Amends D106, amended by D112 |
| [D110](D110-every-xo-repository-is-set-up-for-agents-alike.md) | Every xo repository is set up for coding agents the same way | Amends D50 and D89 |
| [D111](D111-a-large-project-keeps-one-file-per-decision.md) | A large project keeps one file per decision | Amends D50 |
| [D112](D112-eight-servers-run-under-dbrun-for-dbimp.md) | Eight more servers run under dbrun for dbimp's drivers | Amends D109, amended by D114, D119, D123, D134 and D153 |
| [D113](D113-three-avatica-servers-run-under-dbrun.md) | Three Avatica servers run under dbrun for dbimp's driver | Amended by D119 and D155 |
| [D114](D114-a-server-can-answer-more-than-one-dialect.md) | A server can answer more than one dialect | Amends D112, amended by D119 |
| [D115](D115-a-server-shows-its-owners-name-and-status-a-shows.md) | A server shows its owner's name, and status -a shows the stopped ones | Amends D98 |
| [D116](D116-dbrun-knows-the-embedded-databases-before-their-models.md) | dbrun knows the embedded databases before their models | Amended by D119 and D142 |
| [D117](D117-a-hosted-service-appears-in-dbrun-when-its-credential-does.md) | A hosted service appears in dbrun when its credential does | Amended by D125 |
| [D118](D118-every-database-usql-or-dbimp-reaches-gets-an-entry.md) | Every database that usql or dbimp reaches gets an entry | Amended by D123, D141 and D145 |
| [D119](D119-a-release-no-model-reads-is-staged.md) | A release that no model reads is Staged | Amends D40, D103, D106, D112, D113, D114 and D116, amended by D120 and D142 |
| [D120](D120-a-staged-release-keeps-its-cadence.md) | A Staged release keeps its cadence | Amends D119 |
| [D121](D121-the-interface-is-named-queryer.md) | The interface is named Queryer | Amends D49 |
| [D122](D122-dbrun-calls-the-runner-as-few-times-as-it-can.md) | dbrun calls the runner as few times as it can | Decided |
| [D123](D123-cockroachdb-and-cratedb-have-dialects-of-their-own.md) | CockroachDB and CrateDB have dialects of their own | Amends D112 and D118 |
| [D124](D124-a-server-can-publish-a-second-port.md) | A server can publish a second port | Amends D68 |
| [D125](D125-each-wire-compatible-product-has-a-dialect-of-its-own.md) | Each wire compatible product has a dialect of its own | Amends D19, D80, D99 and D117 |
| [D126](D126-a-version-query-waits-for-the-product-s-model.md) | A version query waits for the product's model | Decided |
| [D127](D127-quoting-rules-live-in-dbmeta.md) | Quoting rules live in dbmeta | Amends D56 |
| [D128](D128-cockroachdb-lists-crdb-internal.md) | CockroachDB lists crdb_internal | Decided |
| [D129](D129-an-embedded-product-waits-for-a-model.md) | An embedded product waits for a model | Amends D66, amended by D142 |
| [D130](D130-db2-is-out-of-scope.md) | Db2 is out of scope | Decided |
| [D131](D131-cratedb-answers-no-text-search-kind.md) | CrateDB answers no text search kind | Decided |
| [D132](D132-netezza-is-out-of-scope.md) | Netezza is out of scope | Decided |
| [D133](D133-tidb-shares-the-mysql-model.md) | TiDB shares the mysql model | Decided |
| [D134](D134-tdengine-is-removed.md) | TDengine is removed | Amends D112 |
| [D135](D135-vitess-shares-the-mysql-model.md) | Vitess shares the mysql model and names a schema by its keyspace | Decided |
| [D136](D136-oracle-binds-a-flag-as-a-number.md) | Oracle binds a flag as a number, and is tested on go-ora v3 too | Amends D59, amended by D157 |
| [D137](D137-a-child-kind-takes-parent-for-its-owner.md) | A child kind takes parent for its owner and name for itself | Decided |
| [D138](D138-tables-takes-types.md) | Tables takes types, bound as one string | Decided |
| [D139](D139-a-column-has-a-collation.md) | A column has a collation | Decided |
| [D140](D140-databend-reads-its-system-database.md) | Databend reads its system database | Decided |
| [D141](D141-singlestore-shares-the-mysql-model.md) | SingleStore shares the mysql model, and runs with no license | Amends D118 |
| [D142](D142-ql-is-removed.md) | ql is removed | Amends D116, D119 and D129 |
| [D143](D143-a-dialect-says-how-its-sql-is-written.md) | A dialect says how its SQL is written | Decided |
| [D144](D144-snowflake-and-redshift-are-written-before-they-run.md) | Snowflake and Redshift are written before they run | Decided, amended by D182 and D190 |
| [D145](D145-impala-runs-in-one-container.md) | Impala runs in one container | Amends D118 |
| [D146](D146-a-query-can-walk-several-statements.md) | A query can walk several statements | Amends D47 and D67, amended by D159 and D175 |
| [D147](D147-the-fields-psql-prints-that-usql-needed.md) | The fields psql prints that usql needed | Decided, amended by D201 |
| [D148](D148-rqlite-shares-the-sqlite3-model.md) | rqlite shares the sqlite3 model | Amended by D151 |
| [D149](D149-oracle-and-singlestore-answer-extended-statistics.md) | Oracle and SingleStore answer extended statistics | Decided |
| [D150](D150-oracle-11g-reads-two-views-slowly.md) | Oracle 11g reads two views slowly | Decided |
| [D151](D151-rqlite-tests-use-dbimps-driver.md) | The rqlite tests use dbimp's driver | Amends D148 |
| [D152](D152-influxdb-3-reads-datafusions-information-schema.md) | InfluxDB 3 reads DataFusion's information_schema | Amended by D170 |
| [D153](D153-libsql-has-an-ordinary-user-through-a-jwt.md) | libSQL has an ordinary user through a JWT | Amends D112, amended by D160 |
| [D154](D154-a-test-driver-is-the-one-dburl-names.md) | A test driver is the one dburl names | Amends D52 and D80, amended by D157 |
| [D155](D155-the-avatica-servers-speak-json.md) | The Avatica servers speak JSON | Amends D113 |
| [D156](D156-a-test-checks-the-simple-english-rules-a-machine-can.md) | A test checks the simple English rules that a machine can check | Decided |
| [D157](D157-oracle-is-tested-on-go-ora-v3-at-the-fixing-commit.md) | Oracle is tested on go-ora v3 at the commit that fixes it | Amends D59, D136 and D154, amended by D166 |
| [D158](D158-h2-and-voltdb-wait.md) | H2 and VoltDB wait | Decided, amended by D188 |
| [D159](D159-influxql-can-walk-show-statements.md) | InfluxQL can walk SHOW statements | Amends D146, amended by D175 |
| [D160](D160-libsql-shares-the-sqlite3-model.md) | libSQL shares the sqlite3 model | Amends D153, amended by D167 |
| [D162](D162-neo4j-maps-a-database-to-a-schema-and-a-label-to-a.md) | Neo4j maps a database to a schema and a label to a table | Decided |
| [D161](D161-ydb-reads-its-sys-views-and-a-directory-is-a-schema.md) | YDB reads its .sys views, and a directory is a schema | Amends D45 |
| [D163](D163-arangodb-maps-a-collection-onto-a-table.md) | ArangoDB maps a collection onto a table | Amended by D168 |
| [D165](D165-influxql-maps-a-database-to-a-schema-and-a.md) | InfluxQL maps a database to a schema and a measurement to a table | Decided |
| [D164](D164-surrealdb-maps-a-namespace-to-a-catalog-and-a-field-to-a.md) | SurrealDB maps a namespace to a catalog and a defined field to a column | Amended by D168 and D192 |
| [D166](D166-oracle-19c-pinot-and-the-influxdb-release-wait.md) | Oracle 19c, Pinot and the InfluxDB release wait | Amends D157 |
| [D167](D167-the-dsn-is-what-sql-open-takes-and-api-is-the-http-address.md) | The DSN is what sql.Open takes, and api is the HTTP address | Amends D160 |
| [D168](D168-ken-reviews-the-six-dialects-of-2026-10-01.md) | Ken reviews the six dialects of 2026-10-01 | Amends D163 and D164 |
| [D169](D169-each-pattern-and-open-move-from-usql.md) | Each, Pattern and Open move from usql | Decided |
| [D170](D170-influxdb-3-answers-triggers-from-the-processing-engine.md) | InfluxDB 3 answers Triggers from the processing engine | Amends D152 |
| [D171](D171-druid-maps-a-datasource-to-a-table-and-reads-information-schema.md) | Druid maps a datasource to a table and reads INFORMATION_SCHEMA | Decided |
| [D172](D172-the-trino-and-presto-tests-use-dbimps-driver.md) | The Trino and Presto tests use dbimp's driver | Decided |
| [D173](D173-the-odbc-fallback-belongs-to-the-client.md) | The ODBC fallback belongs to the client | Decided |
| [D174](D174-the-clickhouse-tests-use-dbimps-driver.md) | The ClickHouse tests use dbimp's driver | Decided |
| [D175](D175-elasticsearch-and-opensearch-can-walk-show-statements.md) | Elasticsearch and OpenSearch can walk SHOW statements | Amends D146 and D159 |
| [D176](D176-the-first-search-dialects-follow-the-survey-answers.md) | The search dialects follow Ken's answers to the survey | Amended by D183 and D192 |
| [D178](D178-drill-maps-a-workspace-to-a-schema-and-needs-the-metastore-for-file-tables.md) | Drill maps a workspace to a schema and needs the Metastore for file tables | Decided |
| [D177](D177-elasticsearch-maps-an-index-to-a-table-and-walks-sys-and-show.md) | Elasticsearch maps an index to a table and walks SYS and SHOW | Decided, amended by D191 |
| [D179](D179-solr-maps-a-collection-to-a-table-under-a-fixed-schema.md) | Solr maps a collection to a table under a fixed schema | Decided, amended by D191 |
| [D180](D180-voltdb-has-no-catalog-a-statement-can-reach.md) | VoltDB has no catalog that a statement can reach | Decided |
| [D181](D181-opensearch-maps-an-index-to-a-table-and-walks-show-and-describe.md) | OpenSearch maps an index to a table and walks SHOW and DESCRIBE | Decided, amended by D189 and D191 |
| [D182](D182-redshift-ran-against-redshift-serverless.md) | Redshift ran against Redshift Serverless | Amends D144, amended by D204 |
| [D183](D183-aliases-are-tables-and-dbimp-supplies-the-version.md) | Aliases are tables where SQL cannot tell them, and dbimp supplies the version | Amends D176, amended by D191 |
| [D184](D184-dynamodb-has-no-catalog-a-statement-can-reach.md) | DynamoDB has no catalog that a statement can reach | Decided |
| [D185](D185-pinot-has-no-catalog-a-statement-can-reach.md) | Apache Pinot has no catalog that a statement can reach | Decided |
| [D186](D186-avatica-reads-the-hsqldb-catalog-and-phoenix-has-no-model.md) | Avatica reads the HSQLDB catalog, and Phoenix has no model | Decided |
| [D187](D187-gizmosql-shares-the-duckdb-model-and-the-tests-open-the-session.md) | GizmoSQL shares the DuckDB model and the tests open the session | Decided |
| [D188](D188-no-dialect-for-h2-voltdb-chai-or-csvq.md) | No dialect for H2, VoltDB, chai or csvq | Amends D158, amended by D194 |
| [D189](D189-dbimp-reads-the-opensearch-2-19-6-describe-row.md) | dbimp reads the OpenSearch 2.19.6 DESCRIBE row | Amends D181 |
| [D190](D190-snowflake-ran-against-a-trial-account.md) | Snowflake ran against a trial account | Amends D144, amended by D193 and D203 |
| [D191](D191-elasticsearch-solr-and-opensearch-read-the-release-with-select-version.md) | Elasticsearch, Solr and OpenSearch read the release with SELECT version() | Amends D177, D179, D181 and D183, amended by D192 |
| [D192](D192-the-ordinary-user-reads-the-release-and-surrealdb-reads-it-with-select-version.md) | The ordinary user reads the release, and SurrealDB reads it with SELECT version() | Amends D191, D176 and D164 |
| [D193](D193-snowflake-parity-conformance-and-password-were-measured.md) | Snowflake parity, conformance and the password statement were measured | Amends D190, amended by D203 |
| [D194](D194-no-models-for-hosted-services-beyond-redshift-and-snowflake.md) | No models for hosted services beyond Redshift and Snowflake | Amends D188 |
| [D195](D195-cassandra-and-scylladb-have-a-url-dsn-and-an-ordinary-user.md) | Cassandra and ScyllaDB have a URL DSN and an ordinary user | Decided, amended by D196 |
| [D196](D196-the-cassandra-dialect-is-cassandra.md) | The Cassandra dialect is cassandra | Amends D81, D93 and D195 |
| [D197](D197-three-fields-can-be-null-in-the-postgresql-catalog.md) | Three fields can be NULL in the PostgreSQL catalog | Decided |
| [D198](D198-describe-fields-for-relations-indexes-columns-and-functions.md) | Describe fields for relations, indexes, columns and functions | Decided |
| [D199](D199-describe-sections-for-partitions-inheritance-policies-and-rules.md) | Describe sections for partitions, inheritance, policies and rules | Decided, amended by D201 |
| [D200](D200-a-binding-can-keep-rows-for-a-product-that-cannot-filter.md) | A binding can keep rows for a product that cannot filter | Amends D62, amended by D202 |
| [D201](D201-the-third-group-of-describe-data-for-usql.md) | The third group of describe data for usql | Amends D147 and D199 |
| [D202](D202-cassandra-matches-names-ignoring-case-and-the-models-share-two-helpers.md) | Cassandra matches names ignoring case, and the models share two helpers | Amends D200 |
| [D203](D203-snowflake-reads-the-columns-of-a-key-through-the-pipe-operator.md) | Snowflake reads the columns of a key through the pipe operator | Amends D190 and D193 |
| [D204](D204-redshift-answers-the-grants-and-the-roles-and-takes-every-password.md) | Redshift answers the grants and the roles, and takes every password | Amends D182 |
| [D205](D205-the-mysql-family-fills-the-new-fields-of-d198-to-d203.md) | The MySQL family fills the new fields of D198 to D203 | Decided |
| [D206](D206-sql-server-and-oracle-fill-the-describe-fields.md) | SQL Server and Oracle fill the describe fields | Decided |
| [D207](D207-snowflake-redshift-cratedb-and-questdb-fill-the-describe-fields-they-have-a-source-for.md) | Snowflake, Redshift, CrateDB and QuestDB fill the describe fields they have a source for | Decided |
