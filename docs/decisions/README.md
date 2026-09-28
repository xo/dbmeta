# Decisions

Every decision this project made is a file in this folder, one per decision,
named by its number and its title. This table is the index. Find the number
here, then open the file.

Each file opens with its status. "Decided" means Ken chose it. "Proposed"
means an agent or a peer session suggested it and Ken has not confirmed it.
"Open" means nobody has chosen yet. A decision that changes an earlier one
says so in its status, as "Amends D50", and the earlier one says it back, as
"Amended by D111". Read the status before the decision.
37 of them amend or replace an earlier one, and a decision read without its
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
| [D6](D006-fix-the-null-scan-defect-once-and-never-hide-a.md) | Fix the NULL scan defect once, and never hide a NULL | Decided, amended in place |
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
| [D19](D019-do-not-repeat-dburl.md) | Do not repeat dburl | Half decided, half overtaken by the code |
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
| [D45](D045-a-query-may-answer-partially-once-and-must-say.md) | A query may answer partially, once, and must say so | Decided |
| [D46](D046-five-object-kinds-are-missing-and-two-consumers.md) | Five object kinds are missing, and two consumers say which | Decided |
| [D47](D047-dbmeta-supplies-the-data-the-consumer-decides.md) | dbmeta supplies the data. The consumer decides what to show | Decided |
| [D48](D048-cgo-is-allowed-in-the-test-module-and-nowhere.md) | cgo is allowed in the test module, and nowhere else | Amends D26, D29 and D35 |
| [D49](D049-one-method-on-the-interface-and-a-not-null-is.md) | One method on the interface, and a NOT NULL is not a constraint row | Amended by D121 |
| [D50](D050-documentation-lives-in-docs-and-the-decision-log.md) | Documentation lives in docs, and the decision log stays one file | Amended by D110 and D111 |
| [D51](D051-there-is-no-alias-for-a-nullable-type.md) | There is no alias for a nullable type | Decided |
| [D52](D052-a-test-driver-is-the-one-usql-uses-or-it-is-the.md) | A test driver is the one usql uses, or it is the wrong driver | Amended by D59 |
| [D53](D053-one-canonical-expectation-checked-in-that-every.md) | One canonical expectation, checked in, that every database must meet | Decided |
| [D54](D054-sql-server-covers-every-release-that-ships-a.md) | SQL Server covers every release that ships a Linux container | Amended by D63 |
| [D55](D055-the-current-user-moves-here-changing-a-password.md) | The current user moves here. Changing a password does not | Decided |
| [D56](D056-the-password-statement-is-built-here-and-run-by.md) | The password statement is built here and run by the caller | Amends D5 |
| [D57](D057-a-windows-machine-is-how-a-pre-2017-sql-server.md) | A Windows machine is how a pre 2017 SQL Server gets tested, and it is Verified | Decided |
| [D58](D058-one-gitignore-in-the-repository-root.md) | One .gitignore, in the repository root | Decided |
| [D59](D059-oracle-is-tested-with-go-ora-v2-until-v3-tags.md) | Oracle is tested with go-ora v2 until v3 tags its fix | Amends D52 |
| [D60](D060-the-oracle-model-reads-all-views-and-there-is-no.md) | The Oracle model reads ALL_ views, and there is no DBA_ variant | Decided |
| [D61](D061-every-dialect-is-measured-against-every.md) | Every dialect is measured against every principal the product has | Amended in place |
| [D62](D062-cql-cannot-compute-so-the-cassandra-model.md) | CQL cannot compute, so the Cassandra model computes in Scan | Amended by D93 |
| [D63](D063-support-says-when-a-release-is-too-old.md) | Support says when a release is too old | Amends D54 |
| [D64](D064-the-verified-tier-is-checked-against-the.md) | The Verified tier is checked against the document | Decided |
| [D65](D065-a-windows-machine-rearms-its-evaluation-before.md) | A Windows machine rearms its evaluation before it expires | Decided |
| [D66](D066-the-order-the-remaining-dialects-are-written-in.md) | The order the remaining dialects are written in | Amended by D67, D77, D88, D91 and D94 |
| [D67](D067-impala-cannot-be-a-dbmeta-model-and-clickhouse.md) | Impala cannot be a dbmeta model, and ClickHouse goes first | Amends D66 |
| [D68](D068-every-container-is-started-by-the-runner-and.md) | Every container is started by the runner and named product-release | Amended by D70 |
| [D69](D069-the-workflow-builds-its-matrix-from-the-go-list.md) | The workflow builds its matrix from the Go list | Amends D42 |
| [D70](D070-the-runner-is-a-go-command-called-dbrun.md) | The runner is a Go command called dbrun | Amends D68 and D12 |
| [D71](D071-nothing-here-is-generated-the-models-are-written.md) | Nothing here is generated. The models are written | Amends D2, D12 and D30, supersedes D11 |
| [D72](D072-trino-reads-system-jdbc-and-a-catalog-is-a-real.md) | Trino reads system.jdbc, and a catalog is a real level | Decided |
| [D73](D073-presto-is-its-own-dialect-and-not-a-flavor-of.md) | Presto is its own dialect, and not a flavor of Trino | Decided |
| [D74](D074-firebird-has-no-schemas-and-none-is-invented.md) | Firebird has no schemas, and none is invented | Decided |
| [D75](D075-every-container-is-bounded-and-four-run-at-once.md) | Every container is bounded, and four run at once | Amended by D98 and D108 |
| [D76](D076-sap-hana-reads-sys-and-answers-more-than.md) | SAP HANA reads SYS, and answers more than anything but PostgreSQL | Decided |
| [D77](D077-exasol-will-not-run-here-and-hive-goes-ahead-of.md) | Exasol will not run here, and Hive goes ahead of it | Amends D66, amended by D84 |
| [D78](D078-hive-reads-sys-and-is-a-model.md) | Hive reads sys, and is a model | Decided |
| [D79](D079-a-dialect-that-cannot-bind-renders-its-values.md) | A dialect that cannot bind renders its values | Decided |
| [D80](D080-the-driver-registry-is-dburl-s-and-reading-it-is.md) | The driver registry is dburl's, and reading it is not importing it | Decided |
| [D81](D081-the-cassandra-dialect-is-cql.md) | The Cassandra dialect is cql | Decided |
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
| [D93](D093-the-cql-tests-use-github-com-xo-cql-which.md) | The cql tests use github.com/xo/cql, which reports a NULL | Amends D62 |
| [D94](D094-couchbase-runs-under-dbrun-and-its-model-waits.md) | Couchbase runs under dbrun, and its model waits for the n1ql rewrite | Amends D66, amended by D95, D96 and D104 |
| [D95](D095-the-couchbase-model-waits-for-the-dbimp-driver.md) | The Couchbase model waits for the dbimp driver | Amends D94, amended by D101 and D104 |
| [D96](D096-couchbase-gets-an-ordinary-user-and-starts-again.md) | Couchbase gets an ordinary user, and starts again after a stop | Amends D94, amended by D104 |
| [D97](D097-dbrun-is-documented-for-its-users-in-dbrun-and.md) | dbrun is documented for its users, in DBRUN and CONTAINERS | Decided |
| [D98](D098-a-server-has-an-owner-and-dbrun-acts-only-on-the.md) | A server has an owner, and dbrun acts only on the caller's own | Amends D75, amended by D102, D108 and D115 |
| [D99](D099-dburl-names-the-product-that-a-scheme-drives.md) | dburl names the product that a scheme drives | Decided |
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
| [D112](D112-eight-servers-run-under-dbrun-for-dbimp.md) | Eight more servers run under dbrun for dbimp's drivers | Amends D109, amended by D114, D119 and D123 |
| [D113](D113-three-avatica-servers-run-under-dbrun.md) | Three Avatica servers run under dbrun for dbimp's driver | Amended by D119 |
| [D114](D114-a-server-can-answer-more-than-one-dialect.md) | A server can answer more than one dialect | Amends D112, amended by D119 |
| [D115](D115-a-server-shows-its-owners-name-and-status-a-shows.md) | A server shows its owner's name, and status -a shows the stopped ones | Amends D98 |
| [D116](D116-dbrun-knows-the-embedded-databases-before-their-models.md) | dbrun knows the embedded databases before their models | Amended by D119 |
| [D117](D117-a-hosted-service-appears-in-dbrun-when-its-credential-does.md) | A hosted service appears in dbrun when its credential does | Decided |
| [D118](D118-every-database-usql-or-dbimp-reaches-gets-an-entry.md) | Every database that usql or dbimp reaches gets an entry | Amended by D123 |
| [D119](D119-a-release-no-model-reads-is-staged.md) | A release that no model reads is Staged | Amends D40, D103, D106, D112, D113, D114 and D116, amended by D120 |
| [D120](D120-a-staged-release-keeps-its-cadence.md) | A Staged release keeps its cadence | Amends D119 |
| [D121](D121-the-interface-is-named-queryer.md) | The interface is named Queryer | Amends D49 |
| [D122](D122-dbrun-calls-the-runner-as-few-times-as-it-can.md) | dbrun calls the runner as few times as it can | Decided |
| [D123](D123-cockroachdb-and-cratedb-have-dialects-of-their-own.md) | CockroachDB and CrateDB have dialects of their own | Amends D112 and D118 |
