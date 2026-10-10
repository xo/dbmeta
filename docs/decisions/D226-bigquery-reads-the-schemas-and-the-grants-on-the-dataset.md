# D226. BigQuery reads the schemas and the grants on the dataset

Status: Amends D220.

## The decision

D220 left out Schemas, CurrentSchema and Privileges, because the principals of
the test account had no permission on the project. dbsetup now grants
`roles/bigquery.metadataViewer` on the whole project to both principals, so
SCHEMATA and SCHEMATA_OPTIONS answer. The model now answers 18 of the 65.

## What was measured

On 2026-10-11, on the hosted service, in the location US.

1. SCHEMATA lists every dataset of the project, and a dataset of another test
   session is one of them. The model reads the one dataset of the connection,
   so the statement must keep one row. `@@dataset_id` is NULL on a session with
   a default dataset, so no statement can read its name. The tables of the
   dataset name it, because `INFORMATION_SCHEMA.TABLES` with no dataset in front
   reads only the default dataset. So the statement keeps the SCHEMATA row whose
   name is the name of a table of the dataset. A dataset with no table or view has
   no row.
2. SCHEMATA_OPTIONS holds the description, the location, the default table
   expiration and the maximum time travel hours of the dataset, one option to a
   row. The description is the comment, and the rest are in the new field
   `Schema.Options`, as name=value pairs, the way `Table.Options` holds them.
   The owner column of SCHEMATA is empty. Nothing measured a label.
3. OBJECT_PRIVILEGES answers a query that names one object, and the dataset is
   one. It returns one row for each IAM binding, with the role as the privilege
   and the principal as the grantee. A table of the fixture has no row, because
   its access comes from the dataset. So Privileges lists the grants on the
   dataset and no table, and a table that carries its own IAM binding is not
   read, because that needs one statement for each table (D47). The dataset form
   of the view and a query with IN or OR are refused, as D220 says.
4. The name of the view holds the region, and a statement cannot bind a name.
   So the statement builds the second statement as text with FORMAT, from the
   location in SCHEMATA and the dataset, and runs it with EXECUTE IMMEDIATE.
   The driver returns its rows. A dataset with no table gives the statement no
   text, so a second text that returns no row stands in.
5. ROW_ACCESS_POLICIES, DATA_POLICIES, CONNECTIONS and LINKED_DATASETS answer
   not found, in the region and in the dataset. Before the grant they answered
   403. So they are not views of this service, and `Policies` stays unanswered.
   TABLE_STORAGE answers that the project must enable it.

## Parity

The reader, a service account with the same project role, gets the answer of the
administrator to Schemas, CurrentSchema and Privileges. `TestPrivilegeParity`
shows no new line.

## The cost

Schemas and CurrentSchema read TABLES, SCHEMATA and SCHEMATA_OPTIONS, and
Privileges reads TABLES, SCHEMATA and OBJECT_PRIVILEGES, each at least 10 MB.
The cost grows with the number of views and not with the number of tables.
