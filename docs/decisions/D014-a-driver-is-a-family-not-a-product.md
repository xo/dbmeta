# D14. A driver is a family, not a product

Status: Decided.

The `mysql` driver targets MariaDB, and its models must also work against
MySQL. The driver name is the family name. The reference database is the
product that the models are generated against.

This adds a second axis to D8. A model varies by version, and it varies by
flavor. Flavor means a product that speaks the same dialect and claims the same
driver, such as MariaDB and MySQL, or such as PostgreSQL and the databases that
copy its wire protocol.

The two axes are not the same and one does not contain the other. MariaDB 11
and MySQL 8 are two flavors at a similar age. MariaDB 10 and MariaDB 11 are one
flavor at two ages. Testing must cover both axes. See the testing plan.

Rule for agents: generate against the reference product, then test against
every flavor in the family. A query that only works on the reference product is
not finished.
