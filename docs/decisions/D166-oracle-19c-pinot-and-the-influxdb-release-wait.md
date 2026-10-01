# D166. Oracle 19c, Pinot and the InfluxDB release wait

Status: Amends D157.

## The decision

On 2026-10-02 Ken settled three items that were open after the dialects of
2026-10-01.

Oracle 19c is not measured further. D157 left it unmeasured on go-ora v3,
because its first start stayed at 36% of its database creation at the 4 GB
memory limit, and the backlog held a measurement of the memory it needs. Ken
chose not to spend the time, because 19c is an old release. It stays
Verified, and D59 holds its last measurement, on the same go-ora commit.

Apache Pinot gets no model for now. Its catalog is only in the Controller's
REST API, and a model needs a driver that answers metadata statements from
it. Ken chose not to do that work now. He will consider a generic SQL layer
that answers what a product does not, such as SHOW TABLES or DESCRIBE, but it
is not a priority.

usql reads the release of InfluxDB from `GET /ping`, as it does today. No
InfluxQL statement names the release, and InfluxDB 3 names only the release
of DataFusion (D152, D165). The same SQL layer can one day answer a
`SELECT version()`, and that is not a priority either.

The backlog holds the SQL layer as one item for both products.
