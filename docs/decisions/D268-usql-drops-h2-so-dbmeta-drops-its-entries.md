# D268. usql drops H2, so dbmeta drops its entries

Status: Amends D118.

usql dropped its h2 driver, and dburl v0.56.0 removes the scheme. The module of
the driver was last released in 2020. D118 gave H2 an entry so that dbrun started
a server for usql. Nothing reaches H2 now, and dbmeta has no H2 model.
The maintainers decided on 2026-11-05 that the entries go.

## Decision

The H2 entry in `container/`, its Containerfile in `test/cmd/dbrun/image` and
its mention in the documents are removed. dbrun no longer starts an H2 server.

The example of a dialect with no model used H2. It uses DynamoDB now.
