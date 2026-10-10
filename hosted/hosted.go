// Package hosted names the hosted database services that dbrun can reach,
// as Go data.
//
// A hosted service has no server to start. Snowflake, BigQuery and the rest
// run somewhere else, and a person who has an account reaches one with a
// connection string that holds a secret. This package holds what is not a
// secret: the name of each service, its dialect, the form of its connection
// string and what its driver reads by itself. It holds no secret, reads no
// file and runs nothing, and it must not. dbrun resolves the connection
// string at run time, from the places D117 names, and a service appears in
// dbrun only while its connection string resolves.
//
// A service that has a local emulator also has a container entry, named in
// Emulator, and that entry is what a person without an account uses.
package hosted

import (
	"github.com/xo/dbmeta"
	"github.com/xo/dbmeta/container"
)

// Service is one hosted database service.
type Service struct {
	// Name is the name dbrun shows, and the name every credential source
	// uses: the variable DBMETA_<NAME>_DSN, the file credentials/<name> and
	// the helper dbmeta-credential-<name>.
	Name string
	// Product is the name the vendor gives the service.
	Product string
	// Dialect is the dbmeta dialect, which is dburl's. A service can share
	// one with a server product, as Neon shares postgres and PlanetScale
	// shares mysql. Redshift speaks PostgreSQL's protocol and has a dialect
	// of its own from dburl v0.36.0, because its catalog is its own (D125).
	Dialect dbmeta.Dialect
	// Tier is how thoroughly the service is tested. CI never runs a hosted
	// service, because it costs money and needs a secret. A service that a
	// model reads is Verified, because a person with an account runs the
	// tests against it. A service that no model reads is Staged, as a
	// container entry is. See D117 and D119.
	Tier container.Tier
	// Form is the shape of the connection string, for a person, with each
	// part a person fills in written in angle brackets.
	Form string
	// Native says what the driver reads by itself, beside the connection
	// string, such as a key file named by GOOGLE_APPLICATION_CREDENTIALS.
	// The connection string can then hold no secret at all.
	Native string
	// Emulator is the product name of the container entry that emulates
	// the service, and empty when there is none.
	Emulator string
}

// All is every hosted service dbrun knows, in the order dbrun lists them.
//
// Fauna and the MongoDB Atlas Data API are not here. Fauna shut down on
// 2025-05-30, and MongoDB removed the Data API on 2025-09-30. SingleStore is
// not here either. Its development image runs locally, and it has a container
// entry. See D117 and D141.
func All() []Service {
	out := []Service{
		{
			Name: "athena", Product: "Amazon Athena", Dialect: dbmeta.Athena,
			Form: "athena://<key id>:<secret>@athena.<region>.amazonaws.com/<database>" +
				"?workgroup=<workgroup>&output=s3://<bucket>/<path>/",
		},
		{
			Name: "bigquery", Product: "Google BigQuery", Dialect: dbmeta.BigQuery,
			Form:     "bigquery://<project>/<location>/<dataset>?credential_file=<path of the key file>",
			Emulator: "bigquery",
		},
		{
			Name: "cosmos", Product: "Azure Cosmos DB", Dialect: dbmeta.Cosmos,
			Form:     "cosmos://<user>:<account key, percent encoded>@<account>.documents.azure.com/<database>",
			Emulator: "cosmos",
		},
		{
			Name: "databricks", Product: "Databricks", Dialect: dbmeta.Databricks,
			Form: "databricks://token:<personal access token>@<workspace>.cloud.databricks.com" +
				"/<warehouse id>?catalog=<catalog>&schema=<schema>",
		},
		{
			Name: "dynamodb", Product: "Amazon DynamoDB", Dialect: dbmeta.DynamoDB,
			Form:     "dynamodb://<key id>:<secret>@<region>",
			Native:   "AWS_REGION, AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY",
			Emulator: "dynamodb",
		},
		{
			Name: "maxcompute", Product: "Alibaba Cloud MaxCompute", Dialect: dbmeta.MaxCompute,
			Form:   "maxcompute://<key id>:<secret>@service.<region>.maxcompute.aliyun.com/api?project=<project>",
			Native: "ALIBABA_CLOUD_ACCESS_KEY_ID, ALIBABA_CLOUD_ACCESS_KEY_SECRET and ALIBABA_CLOUD_SECURITY_TOKEN",
		},
		{
			Name: "neon", Product: "Neon", Dialect: dbmeta.PostgreSQL,
			Form: "postgres://<user>:<password>@<endpoint>.neon.tech/<database>?sslmode=require",
		},
		{
			Name: "planetscale", Product: "PlanetScale", Dialect: dbmeta.MySQL,
			Form: "mysql://<user>:<password>@aws.connect.psdb.cloud/<database>?tls=true",
		},
		{
			Name: "redshift", Product: "Amazon Redshift", Dialect: dbmeta.Redshift,
			Form: "redshift://<user>:<password>@<cluster>.<region>.redshift.amazonaws.com:5439/<database>",
		},
		{
			Name: "snowflake", Product: "Snowflake", Dialect: dbmeta.Snowflake,
			Form: "snowflake://<user>@<account>/<database>?warehouse=<warehouse>&role=<role>" +
				"&authenticator=SNOWFLAKE_JWT&privateKey=<key>",
		},
		{
			Name: "spanner", Product: "Google Cloud Spanner", Dialect: dbmeta.Spanner,
			Form:     "spanner:///<project>/<instance>/<database>?credential_file=<path of the key file>",
			Emulator: "spanner",
		},
		{
			Name: "tablestore", Product: "Alibaba Cloud Tablestore", Dialect: dbmeta.Tablestore,
			Form: "ots://<key id>:<secret>@<instance>.<region>.ots.aliyuncs.com/<instance>",
		},
	}
	for i := range out {
		out[i].Tier = container.Staged
		switch out[i].Dialect {
		case dbmeta.PostgreSQL, dbmeta.MySQL, dbmeta.Snowflake, dbmeta.Redshift, dbmeta.Spanner, dbmeta.BigQuery, dbmeta.Athena, dbmeta.Databricks, dbmeta.Cosmos:
			out[i].Tier = container.Verified
		}
	}
	return out
}
