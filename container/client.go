package container

import (
	"net/url"
	"strconv"
	"strings"
)

// Connection is where a native client connects, read from the URL of one
// principal of a server. It holds the parts that a client takes as separate
// options, so that each product writes its own form once and no caller parses
// a URL.
type Connection struct {
	// Host is the host name or address, such as 127.0.0.1.
	Host string
	// Port is the port in the URL.
	Port int
	// First is the first host port that the server is published on. It is
	// the port the URL names for most products. It is not for a product whose
	// URL names the second port, such as ClickHouse and the Spanner emulator,
	// where the native client uses the first one. See [Server.SecondPort].
	First int
	// User is the name the principal logs in with, and is empty when the URL
	// has none.
	User string
	// Password is the password in the URL, and is empty when the URL has
	// none. It is a secret.
	Password string
	// Database is the path of the URL without its first slash.
	Database string
	// Query is the query of the URL.
	Query url.Values
	// Scheme is the scheme of the URL.
	Scheme string
}

// Error is an error.
type Error string

// Error satisfies the error interface.
func (err Error) Error() string { return string(err) }

// ErrBadURL is the error for a connection URL that cannot be read. It names no
// part of the URL, because a hosted URL holds a secret.
const ErrBadURL Error = "the connection URL is not valid"

// ParseConnection reads a URL that a [Server] or a [Principal] gives, and
// first is the first host port of the server. It returns an error when the
// text is not a URL.
func ParseConnection(raw string, first int) (Connection, error) {
	u, err := url.Parse(raw)
	if err != nil {
		// The error of url.Parse holds the whole URL, and a hosted URL holds a
		// secret, so the error is not wrapped.
		return Connection{}, ErrBadURL
	}
	c := Connection{
		Host:     u.Hostname(),
		First:    first,
		Database: strings.TrimPrefix(u.Path, "/"),
		Query:    u.Query(),
		Scheme:   u.Scheme,
	}
	if p := u.Port(); p != "" {
		if c.Port, err = strconv.Atoi(p); err != nil {
			return Connection{}, ErrBadURL
		}
	}
	if c.First == 0 {
		c.First = c.Port
	}
	if u.User != nil {
		c.User = u.User.Username()
		c.Password, _ = u.User.Password()
	}
	return c, nil
}

// Call is one run of a native client: the command, its arguments and the
// environment variables that it needs. It holds a secret and must never be
// printed as it is. [Call.Masked] is the form to print.
type Call struct {
	// Command is the name of the client, which the caller looks up on the
	// path of the host.
	Command string
	// Args are the arguments of the command.
	Args []string
	// Env is the environment variables of the call as NAME=value. The
	// caller adds them to its own environment.
	Env []string

	// secret is the password. It is empty when the principal has none.
	secret string
}

// InArgs reports whether the password is in the arguments, where a process
// list shows it. A client that has an environment variable or a file for the
// password never makes this true.
func (c Call) InArgs() bool {
	if c.secret == "" {
		return false
	}
	for _, a := range c.Args {
		if strings.Contains(a, c.secret) {
			return true
		}
	}
	return false
}

// Masked returns the arguments with the password replaced by xxxxx.
func (c Call) Masked() []string {
	out := make([]string, len(c.Args))
	for i, a := range c.Args {
		out[i] = a
		if c.secret != "" {
			out[i] = strings.ReplaceAll(a, c.secret, "xxxxx")
		}
	}
	return out
}

// EnvNames returns the names of the environment variables, without values.
func (c Call) EnvNames() []string {
	out := make([]string, len(c.Env))
	for i, e := range c.Env {
		out[i], _, _ = strings.Cut(e, "=")
	}
	return out
}

// Client is the native command line client of a product, which is the client
// the vendor ships, such as psql for PostgreSQL. It is data. It names the
// command and builds the arguments, and it runs nothing: the caller looks the
// command up on its own path and runs it.
//
// A product with no such client says why in Why, and a test fails when a
// product has neither. See D236.
type Client struct {
	// Command is the name of the client on the path of the host. It is empty
	// when the product has no native client.
	Command string
	// Why says why there is no client, when Command is empty.
	Why string

	build func(Connection) (args, env []string)
}

// Call builds the call of this client for conn. It returns the zero Call
// when the product has no client.
//
// The password goes in an environment variable where the client reads one,
// and in the arguments only where it does not. [Call.InArgs] says which.
func (c Client) Call(conn Connection) Call {
	if c.Command == "" || c.build == nil {
		return Call{}
	}
	args, env := c.build(conn)
	return Call{Command: c.Command, Args: args, Env: env, secret: conn.Password}
}

// NoClient is a product with no native client.
func NoClient(why string) Client { return Client{Why: why} }

// opt is a flag and its value, and nothing when the value is empty.
func opt(name, value string) []string {
	if value == "" {
		return nil
	}
	return []string{name, value}
}

// withEnv is NAME=value, and nothing when the value is empty.
func withEnv(name, value string) []string {
	if value == "" {
		return nil
	}
	return []string{name + "=" + value}
}

func itoa(n int) string { return strconv.Itoa(n) }

// hostPort is host:port, which is how most clients take an address.
func hostPort(host string, n int) string { return host + ":" + itoa(n) }

// PsqlClient is psql, for PostgreSQL and every product that speaks its
// protocol. The password is PGPASSWORD.
func PsqlClient() Client { return psqlClient("") }

// PsqlTLSClient is psql for a hosted service that refuses a connection without
// TLS, such as Amazon Redshift. It asks for TLS unless the URL names a mode.
func PsqlTLSClient() Client { return psqlClient("require") }

// psqlClient builds psql, with sslmode as the default mode when the URL names
// none, and no default when it is empty.
func psqlClient(sslmode string) Client {
	return Client{Command: "psql", build: func(c Connection) ([]string, []string) {
		args := []string{"--host", c.Host, "--port", itoa(c.Port)}
		args = append(args, opt("--username", c.User)...)
		args = append(args, opt("--dbname", c.Database)...)
		env := withEnv("PGPASSWORD", c.Password)
		mode := c.Query.Get("sslmode")
		if mode == "" {
			mode = sslmode
		}
		return args, append(env, withEnv("PGSSLMODE", mode)...)
	}}
}

// MysqlClient is a client of the MySQL protocol. The command is mysql or
// mariadb, and both read the password from MYSQL_PWD.
func MysqlClient(command string) Client { return mysqlClient(command, false) }

// MysqlTLSClient is mysql for a hosted service that refuses a connection
// without TLS, such as PlanetScale.
func MysqlTLSClient() Client { return mysqlClient("mysql", true) }

func mysqlClient(command string, tls bool) Client {
	return Client{Command: command, build: func(c Connection) ([]string, []string) {
		args := []string{"--protocol", "tcp", "--host", c.Host, "--port", itoa(c.Port)}
		args = append(args, opt("--user", c.User)...)
		args = append(args, opt("--database", c.Database)...)
		if tls {
			args = append(args, "--ssl-mode=REQUIRED")
		}
		return args, withEnv("MYSQL_PWD", c.Password)
	}}
}

// sqlcmdClient is sqlcmd, which reads the password from SQLCMDPASSWORD. -C
// trusts the certificate of the server, which is a self signed one.
func sqlcmdClient() Client {
	return Client{Command: "sqlcmd", build: func(c Connection) ([]string, []string) {
		args := []string{"-S", c.Host + "," + itoa(c.Port), "-C"}
		args = append(args, opt("-U", c.User)...)
		db := c.Database
		if db == "" {
			db = c.Query.Get("database")
		}
		args = append(args, opt("-d", db)...)
		return args, withEnv("SQLCMDPASSWORD", c.Password)
	}}
}

// sqlplusClient is SQL*Plus. It takes the password only in the connect
// string, so the password is in the arguments.
func sqlplusClient() Client {
	return Client{Command: "sqlplus", build: func(c Connection) ([]string, []string) {
		login := c.User
		if c.Password != "" {
			// Double quotes keep a symbol in the password from ending it.
			login += `/"` + c.Password + `"`
		}
		return []string{login + "@//" + hostPort(c.Host, c.Port) + "/" + c.Database}, nil
	}}
}

// clickhouseClient is clickhouse-client on the native port, which is the first
// published port, and reads the password from CLICKHOUSE_PASSWORD.
func clickhouseClient() Client {
	return Client{Command: "clickhouse-client", build: func(c Connection) ([]string, []string) {
		args := []string{"--host", c.Host, "--port", itoa(c.First)}
		args = append(args, opt("--user", c.User)...)
		args = append(args, opt("--database", c.Database)...)
		return args, withEnv("CLICKHOUSE_PASSWORD", c.Password)
	}}
}

// cqlshClient is cqlsh. It reads the password from a credentials file, which
// dbrun does not write yet, so the password is in the arguments. See D236.
func cqlshClient() Client {
	return Client{Command: "cqlsh", build: func(c Connection) ([]string, []string) {
		args := []string{c.Host, itoa(c.Port)}
		args = append(args, opt("--username", c.User)...)
		return append(args, opt("--password", c.Password)...), nil
	}}
}

// mongoshClient is mongosh. The password is in the arguments, because the
// client reads it from no environment variable.
func mongoshClient() Client {
	return Client{Command: "mongosh", build: func(c Connection) ([]string, []string) {
		target := "mongodb://" + hostPort(c.Host, c.Port) + "/" + c.Database
		if a := c.Query.Get("authSource"); a != "" {
			target += "?authSource=" + url.QueryEscape(a)
		}
		args := []string{target}
		args = append(args, opt("--username", c.User)...)
		return append(args, opt("--password", c.Password)...), nil
	}}
}

// trinoClient is the command line client of Trino and of Presto. The two take
// the same options. Neither server has a password here.
func trinoClient(command string) Client {
	return Client{Command: command, build: func(c Connection) ([]string, []string) {
		args := []string{"--server", "http://" + hostPort(c.Host, c.Port)}
		args = append(args, opt("--user", c.User)...)
		catalog, schema, _ := strings.Cut(c.Database, "/")
		args = append(args, opt("--catalog", catalog)...)
		return append(args, opt("--schema", schema)...), nil
	}}
}

// firebirdClient is isql-fb, the name the Firebird tool has on Linux, because
// isql is also the name of the unixODBC tool. It reads ISC_USER and
// ISC_PASSWORD.
func firebirdClient() Client {
	return Client{Command: "isql-fb", build: func(c Connection) ([]string, []string) {
		env := withEnv("ISC_USER", c.User)
		env = append(env, withEnv("ISC_PASSWORD", c.Password)...)
		return []string{c.Host + "/" + itoa(c.Port) + ":/" + c.Database}, env
	}}
}

// hdbsqlClient is hdbsql. It reads the password from no environment variable,
// and its password store needs a step that dbrun does not take, so the
// password is in the arguments.
func hdbsqlClient() Client {
	return Client{Command: "hdbsql", build: func(c Connection) ([]string, []string) {
		args := []string{"-n", hostPort(c.Host, c.Port)}
		args = append(args, opt("-u", c.User)...)
		return append(args, opt("-p", c.Password)...), nil
	}}
}

// beelineClient is beeline. The Hive here takes no password (auth is NONE), so
// NoClient is given.
func beelineClient() Client {
	return Client{Command: "beeline", build: func(c Connection) ([]string, []string) {
		args := []string{"-u", "jdbc:hive2://" + hostPort(c.Host, c.Port) + "/" + c.Database}
		return append(args, opt("-n", c.User)...), nil
	}}
}

// vsqlClient is vsql, which reads the password from VSQL_PASSWORD.
func vsqlClient() Client {
	return Client{Command: "vsql", build: func(c Connection) ([]string, []string) {
		args := []string{"-h", c.Host, "-p", itoa(c.Port)}
		args = append(args, opt("-U", c.User)...)
		args = append(args, opt("-d", c.Database)...)
		return args, withEnv("VSQL_PASSWORD", c.Password)
	}}
}

// surrealClient is surreal sql, which reads SURREAL_USER and SURREAL_PASS.
func surrealClient() Client {
	return Client{Command: "surreal", build: func(c Connection) ([]string, []string) {
		args := []string{"sql", "--endpoint", "ws://" + hostPort(c.Host, c.Port), "--pretty"}
		ns, db, _ := strings.Cut(c.Database, "/")
		args = append(args, opt("--namespace", ns)...)
		args = append(args, opt("--database", db)...)
		env := withEnv("SURREAL_USER", c.User)
		return args, append(env, withEnv("SURREAL_PASS", c.Password)...)
	}}
}

// arangoshClient is arangosh. The password is in the arguments, because the
// client reads it from no environment variable.
func arangoshClient() Client {
	return Client{Command: "arangosh", build: func(c Connection) ([]string, []string) {
		args := []string{"--server.endpoint", "tcp://" + hostPort(c.Host, c.Port)}
		args = append(args, opt("--server.username", c.User)...)
		args = append(args, opt("--server.password", c.Password)...)
		return append(args, opt("--server.database", c.Database)...), nil
	}}
}

// influxClient is the influx shell of InfluxDB 1, which reads INFLUX_USERNAME
// and INFLUX_PASSWORD.
func influxClient() Client {
	return Client{Command: "influx", build: func(c Connection) ([]string, []string) {
		args := []string{"-host", c.Host, "-port", itoa(c.Port)}
		args = append(args, opt("-database", c.Database)...)
		env := withEnv("INFLUX_USERNAME", c.User)
		return args, append(env, withEnv("INFLUX_PASSWORD", c.Password)...)
	}}
}

// rqliteClient is the rqlite shell. It takes the password only as user:password
// in an argument.
func rqliteClient() Client {
	return Client{Command: "rqlite", build: func(c Connection) ([]string, []string) {
		args := []string{"-H", c.Host, "-p", itoa(c.Port)}
		if c.User != "" {
			user := c.User
			if c.Password != "" {
				user += ":" + c.Password
			}
			args = append(args, "-u", user)
		}
		return args, nil
	}}
}

// cbqClient is the query shell of Couchbase. The password is in the arguments.
func cbqClient() Client {
	return Client{Command: "cbq", build: func(c Connection) ([]string, []string) {
		args := []string{"-e", "http://" + hostPort(c.Host, c.Port)}
		args = append(args, opt("-u", c.User)...)
		return append(args, opt("-p", c.Password)...), nil
	}}
}

// ksqlClient is the ksql shell. The password is in the arguments.
func ksqlClient() Client {
	return Client{Command: "ksql", build: func(c Connection) ([]string, []string) {
		args := opt("-u", c.User)
		args = append(args, opt("-p", c.Password)...)
		return append(args, "http://"+hostPort(c.Host, c.Port)), nil
	}}
}

// exaplusClient is EXAplus. The password is in the arguments, because the
// client reads it from no environment variable.
func exaplusClient() Client {
	return Client{Command: "exaplus", build: func(c Connection) ([]string, []string) {
		args := []string{"-c", hostPort(c.Host, c.Port)}
		args = append(args, opt("-u", c.User)...)
		args = append(args, opt("-p", c.Password)...)
		if c.Query.Get("validateservercertificate") == "false" {
			args = append(args, "-jdbcparam", "validateservercertificate=0")
		}
		return args, nil
	}}
}

// impalaClient is impala-shell, on the HiveServer2 port.
func impalaClient() Client {
	return Client{Command: "impala-shell", build: func(c Connection) ([]string, []string) {
		return []string{"--impalad", hostPort(c.Host, c.Port), "--protocol", "hs2"}, nil
	}}
}

// cockroachClient is cockroach sql. It reads the whole URL from COCKROACH_URL,
// so the password is never an argument.
func cockroachClient() Client {
	return Client{Command: "cockroach", build: func(c Connection) ([]string, []string) {
		u := url.URL{Scheme: "postgresql", Host: hostPort(c.Host, c.Port), Path: "/" + c.Database, RawQuery: c.Query.Encode()}
		if c.User != "" {
			u.User = url.UserPassword(c.User, c.Password)
		}
		return []string{"sql"}, withEnv("COCKROACH_URL", u.String())
	}}
}

// bendsqlClient is bendsql, which reads the whole DSN from BENDSQL_DSN.
func bendsqlClient() Client {
	return Client{Command: "bendsql", build: func(c Connection) ([]string, []string) {
		u := url.URL{Scheme: "databend", Host: hostPort(c.Host, c.Port), Path: "/" + c.Database, RawQuery: "sslmode=disable"}
		if c.User != "" {
			u.User = url.UserPassword(c.User, c.Password)
		}
		return nil, withEnv("BENDSQL_DSN", u.String())
	}}
}

// sqllineClient is sqlline over Avatica. The server takes no password.
func sqllineClient(command string, jdbc bool) Client {
	return Client{Command: command, build: func(c Connection) ([]string, []string) {
		address := "http://" + hostPort(c.Host, c.Port)
		if !jdbc {
			return []string{address}, nil
		}
		args := []string{"-u", "jdbc:avatica:remote:url=" + address + ";serialization=protobuf"}
		return append(args, opt("-n", c.User)...), nil
	}}
}

// spannerCLIClient is spanner-cli against the emulator, which it finds from
// SPANNER_EMULATOR_HOST. The URL path is the project, the instance and the
// database.
func spannerCLIClient() Client {
	return Client{Command: "spanner-cli", build: func(c Connection) ([]string, []string) {
		project, rest, _ := strings.Cut(c.Database, "/")
		instance, database, _ := strings.Cut(rest, "/")
		args := opt("-p", project)
		args = append(args, opt("-i", instance)...)
		args = append(args, opt("-d", database)...)
		return args, withEnv("SPANNER_EMULATOR_HOST", hostPort(c.Host, c.First))
	}}
}
