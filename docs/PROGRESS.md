# Progress

This file records where the work stands, so that a session that ends or
crashes can resume. Update it when a piece of work starts or ends. Work that
is known and not done goes in [`BACKLOG.md`](BACKLOG.md), and a decision goes
in [`decisions/`](decisions/README.md).

## Where the work stands

On 2026-10-02 `main` and `origin/main` are at 3a0ff0f, and CI passes on it.

Staged and not committed:

- D166, which records three choices Ken made on 2026-10-02. Oracle 19c is not
  measured further. Apache Pinot gets no model for now. usql reads the
  InfluxDB release from `GET /ping`.
- The backlog drops the item for the memory of Oracle 19c, and its Pinot item
  becomes one item for a generic SQL layer, which is not a priority.
- The decision counts for D166.
- This file.

## Waiting for Ken

- Review the mappings in D160 to D165. The six new dialects place a database
  in different ways. Neo4j and InfluxQL make a database the schema. SurrealDB
  makes its namespace the catalog and a database the schema. ArangoDB makes a
  database the catalog, with no schema level.
- D163 counts the JSON schema rule of an ArangoDB collection as a check
  constraint. If Ken calls that a stretch, ArangoDB answers 4 kinds, not 5.

## Done in this session

- The usql session knows that Ken chose `GET /ping` for the InfluxDB release
  (D166). usql now lets a driver's own version hook win over dbmeta's version
  for its banner, so `influxdb://` shows the release from `GET /ping` again.
  That change is in usql, not committed.
