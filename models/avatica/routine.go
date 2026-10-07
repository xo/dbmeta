package avatica

import (
	"database/sql"

	"github.com/xo/dbmeta"
)

func registerRoutines() {
	registerFunctions()
	registerRoutineParameters()
	registerTypes()
	registerDomains()
}

// aggregate is the test for an aggregate function. ROUTINES has one row for
// each routine and no column that says which kind it is, so the text of the
// definition says it. A routine written in Java has no definition.
const aggregate = `COALESCE(r.ROUTINE_DEFINITION, '') LIKE 'CREATE AGGREGATE FUNCTION%'`

// routineArgs lists the types of the parameters that a caller supplies.
const routineArgs = `(SELECT GROUP_CONCAT(` + `CASE WHEN p.DATA_TYPE = 'USER-DEFINED' THEN p.UDT_NAME ELSE p.DATA_TYPE END` +
	` ORDER BY p.ORDINAL_POSITION SEPARATOR ', ')` +
	` FROM INFORMATION_SCHEMA.PARAMETERS p WHERE p.SPECIFIC_SCHEMA = r.SPECIFIC_SCHEMA` +
	` AND p.SPECIFIC_NAME = r.SPECIFIC_NAME AND p.PARAMETER_MODE IN ('IN', 'INOUT'))`

// routineQuery is the statement for Functions and Aggregates, which differ in
// the test and the kind.
func routineQuery(kind, test string) dbmeta.Stmt {
	return dbmeta.Stmt{
		always(`SELECT r.ROUTINE_CATALOG AS "catalog"`),
		always(`, r.ROUTINE_SCHEMA AS "schema"`),
		always(`, r.ROUTINE_NAME AS "name"`),
		always(`, r.SPECIFIC_NAME AS "id"`),
		always(`, ` + kind + ` AS "kind"`),
		always(`, CASE WHEN r.DATA_TYPE IS NULL THEN NULL ELSE ` + sqlType("r", "TYPE_UDT_NAME") + ` END AS "result_type"`),
		always(`, ` + routineArgs + ` AS "arg_types"`),
		always(`, RTRIM(CASE WHEN r.IS_DETERMINISTIC = 'YES' THEN 'immutable' ELSE 'volatile' END) AS "volatility"`),
		always(`, '' AS "parallel"`),
		always(`, ` + text + ` AS "owner"`),
		always(`, LOWER(r.SECURITY_TYPE) AS "security"`),
		always(`, ` + text + ` AS "access"`),
		always(`, COALESCE(r.EXTERNAL_LANGUAGE, r.ROUTINE_BODY) AS "language"`),
		always(`, ` + text + ` AS "source"`),
		always(`, ` + text + ` AS "comment"`),
		always(`, r.ROUTINE_DEFINITION AS "definition"`),
		always(`FROM INFORMATION_SCHEMA.ROUTINES r`),
		always(`WHERE ` + test),
		always(`AND ` + notSystem("r.ROUTINE_SCHEMA")),
		always(`AND ` + like("r.ROUTINE_SCHEMA", "@schema")),
		always(`AND ` + like("r.ROUTINE_NAME", "@name")),
		always(`ORDER BY 2, 3, 4`),
	}
}

// routineFields describes the columns of Functions and Aggregates.
func routineFields(what string) []dbmeta.Field {
	return []dbmeta.Field{
		{Name: "catalog", Desc: catalogDesc},
		{Name: "schema"}, {Name: "name"},
		{Name: "id", Desc: "SPECIFIC_NAME, which is the name and a number that HSQLDB adds, so it names one routine where a name is shared by an overload"},
		{Name: "kind", Desc: what},
		{Name: "result_type", Desc: "the return type of a function, and absent for a procedure, which returns none"},
		{Name: "arg_types", Desc: "the types of the IN and INOUT parameters, comma separated and in declaration order. Absent when there are none"},
		{Name: "volatility", Desc: "immutable for a routine declared DETERMINISTIC, and volatile otherwise"},
		{Name: "parallel", Desc: "always empty: HSQLDB marks no parallel safety on a routine"},
		{Name: "owner", Desc: "always absent: ROUTINES has no owner, and the owner of the schema is in Schemas"},
		{Name: "security", Desc: "definer or invoker, from SECURITY_TYPE"},
		{Name: "access", Desc: "always absent: Privileges reads the grants"},
		{Name: "language", Desc: "JAVA for a routine written in Java, which is a library routine of SYSTEM_LOBS or a method, and SQL for the rest"},
		{Name: "source", Desc: "always absent: HSQLDB keeps the whole statement, which is definition"},
		{Name: "comment", Desc: "always absent: COMMENT ON takes no routine in HSQLDB 2.4.1"},
		{Name: "definition", Desc: "the CREATE text, which HSQLDB rewrites with every name in upper case. It is absent for a routine written in Java"},
	}
}

func scanRoutine(rows *sql.Rows) (dbmeta.Function, error) {
	var v dbmeta.Function
	err := rows.Scan(&v.Catalog, &v.Schema, &v.Name, &v.ID, &v.Kind, &v.ResultType, &v.ArgTypes,
		&v.Volatility, &v.Parallel, &v.Owner, &v.Security, &v.Access, &v.Language, &v.Source,
		&v.Comment, &v.Definition)
	return v, err
}

func registerFunctions() {
	dbmeta.Functions.Register(dbmeta.Avatica, &dbmeta.Binding[dbmeta.Function]{
		Stmt:   routineQuery(`LOWER(r.ROUTINE_TYPE)`, `NOT (`+aggregate+`)`),
		Fields: routineFields("function or procedure, from ROUTINE_TYPE"),
		Params: schemaAndName("routine"),
		Scan:   scanRoutine,
	})

	dbmeta.Aggregates.Register(dbmeta.Avatica, &dbmeta.Binding[dbmeta.Function]{
		Stmt:   routineQuery(`'aggregate'`, aggregate),
		Fields: routineFields("always aggregate"),
		Params: schemaAndName("aggregate"),
		Scan:   scanRoutine,
	})
}

func registerRoutineParameters() {
	dbmeta.RoutineParameters.Register(dbmeta.Avatica, &dbmeta.Binding[dbmeta.RoutineParameter]{
		Stmt: dbmeta.Stmt{
			always(`SELECT p.SPECIFIC_CATALOG AS "catalog"`),
			always(`, p.SPECIFIC_SCHEMA AS "schema"`),
			always(`, r.ROUTINE_NAME AS "routine"`),
			always(`, p.SPECIFIC_NAME AS "routine_id"`),
			always(`, p.PARAMETER_NAME AS "name"`),
			always(`, CAST(p.ORDINAL_POSITION AS BIGINT) AS "ordinal"`),
			always(`, LOWER(p.PARAMETER_MODE) AS "mode"`),
			always(`, ` + sqlType("p", "UDT_NAME") + ` AS "data_type"`),
			always(`, ` + text + ` AS "default"`),
			always(`FROM INFORMATION_SCHEMA.PARAMETERS p`),
			always(`JOIN INFORMATION_SCHEMA.ROUTINES r ON r.SPECIFIC_SCHEMA = p.SPECIFIC_SCHEMA AND r.SPECIFIC_NAME = p.SPECIFIC_NAME`),
			always(`WHERE ` + notSystem("p.SPECIFIC_SCHEMA")),
			always(`AND ` + like("p.SPECIFIC_SCHEMA", "@schema")),
			always(`AND ` + like("r.ROUTINE_NAME", "@parent")),
			always(`AND (CAST(@name AS VARCHAR(256)) = '' OR p.PARAMETER_NAME LIKE @name)`),
			always(`ORDER BY 2, 3, 4, 6`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: catalogDesc},
			{Name: "schema"}, {Name: "routine"},
			{Name: "routine_id", Desc: "SPECIFIC_NAME, which is the name and a number that HSQLDB adds, so it names one routine where a name is shared by an overload"},
			{Name: "name", Desc: "absent for a parameter that has no name"},
			{Name: "ordinal", Desc: "from ORDINAL_POSITION, which counts from one over every parameter"},
			{Name: "mode", Desc: "in, out or inout. The return value of a function is not a row here"},
			{Name: "data_type", Desc: "the type with its length, or its precision and scale. HSQLDB reports DOUBLE as DOUBLE PRECISION here"},
			{Name: "default", Desc: "always absent: HSQLDB has no default value for a parameter"},
		},
		Params: []dbmeta.Param{
			{Name: "schema", Desc: "schema name pattern, empty for every schema", Default: ""},
			{Name: "parent", Desc: "routine name pattern, empty for every routine", Default: ""},
			{Name: "name", Desc: "parameter name pattern, empty for every parameter", Default: ""},
			system,
		},
		Scan: func(rows *sql.Rows) (dbmeta.RoutineParameter, error) {
			var v dbmeta.RoutineParameter
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Routine, &v.RoutineID, &v.Name,
				&v.Ordinal, &v.Mode, &v.DataType, &v.Default)
			return v, err
		},
	})
}

func registerTypes() {
	// A distinct type is one a user made with CREATE TYPE. The types the
	// engine has are in SYSTEM_TYPEINFO, and they belong to no schema, so
	// with_system adds them.
	dbmeta.Types.Register(dbmeta.Avatica, &dbmeta.Binding[dbmeta.Type]{
		Stmt: dbmeta.Stmt{
			always(`SELECT u.USER_DEFINED_TYPE_CATALOG AS "catalog"`),
			always(`, u.USER_DEFINED_TYPE_SCHEMA AS "schema"`),
			always(`, u.USER_DEFINED_TYPE_NAME AS "name"`),
			always(`, ` + sqlType("u", "DATA_TYPE") + ` AS "internal"`),
			always(`, LOWER(u.USER_DEFINED_TYPE_CATEGORY) AS "kind"`),
			always(`, '' AS "elements"`),
			always(`, ` + text + ` AS "owner"`),
			always(`, ` + text + ` AS "access"`),
			always(`, ` + text + ` AS "comment"`),
			always(`, ` + text + ` AS "size"`),
			always(`FROM INFORMATION_SCHEMA.USER_DEFINED_TYPES u`),
			always(`WHERE ` + like("u.USER_DEFINED_TYPE_SCHEMA", "@schema")),
			always(`AND ` + like("u.USER_DEFINED_TYPE_NAME", "@name")),
			always(`UNION ALL`),
			always(`SELECT '', '', t.TYPE_NAME, CAST(t.DATA_TYPE AS VARCHAR(10)), 'base', ''`),
			always(`, ` + text + `, ` + text + `, ` + text + `, ` + text),
			always(`FROM INFORMATION_SCHEMA.SYSTEM_TYPEINFO t`),
			always(`WHERE CAST(@with_system AS BOOLEAN)`),
			always(`AND ` + like("''", "@schema")),
			always(`AND ` + like("t.TYPE_NAME", "@name")),
			always(`ORDER BY 2, 3`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: "PUBLIC for a user defined type, and empty for a type of the engine, which belongs to no catalog"},
			{Name: "schema", Desc: "empty for a type of the engine, which belongs to no schema"},
			{Name: "name"},
			{Name: "internal", Desc: "the type a distinct type is built on, such as DECIMAL(12,2). For a type of the engine, the JDBC type code"},
			{Name: "kind", Desc: "distinct for CREATE TYPE, which is the only kind HSQLDB 2.4.1 makes, and base for a type of the engine"},
			{Name: "elements", Desc: "always empty: HSQLDB has no enumerated type and no composite type"},
			{Name: "owner", Desc: "always absent: USER_DEFINED_TYPES has no owner"},
			{Name: "access", Desc: "always absent: Privileges reads the grants"},
			{Name: "comment", Desc: "always absent: COMMENT ON takes no type in HSQLDB"},
			{Name: "size", Desc: "always absent: the catalog has no storage size for a type"},
		},
		Params: []dbmeta.Param{
			{Name: "schema", Desc: "schema name pattern, empty for every schema", Default: ""},
			{Name: "name", Desc: "type name pattern, empty for every type", Default: ""},
			{Name: "with_system", Desc: "include the types of the engine, which belong to no schema", Default: false},
		},
		Scan: func(rows *sql.Rows) (dbmeta.Type, error) {
			var v dbmeta.Type
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Name, &v.Internal, &v.Kind,
				&v.Elements, &v.Owner, &v.Access, &v.Comment, &v.Size)
			return v, err
		},
	})
}

func registerDomains() {
	// The constraints of a domain are rows of DOMAIN_CONSTRAINTS, and the
	// text of each is in CHECK_CONSTRAINTS.
	dbmeta.Domains.Register(dbmeta.Avatica, &dbmeta.Binding[dbmeta.Domain]{
		Stmt: dbmeta.Stmt{
			always(`SELECT d.DOMAIN_CATALOG AS "catalog"`),
			always(`, d.DOMAIN_SCHEMA AS "schema"`),
			always(`, d.DOMAIN_NAME AS "name"`),
			always(`, ` + sqlType("d", "DATA_TYPE") + ` AS "data_type"`),
			always(`, d.COLLATION_NAME AS "collation"`),
			always(`, TRUE AS "nullable"`),
			always(`, d.DOMAIN_DEFAULT AS "default"`),
			always(`, COALESCE((SELECT GROUP_CONCAT(c.CHECK_CLAUSE ORDER BY c.CONSTRAINT_NAME SEPARATOR ' AND ')` +
				` FROM INFORMATION_SCHEMA.DOMAIN_CONSTRAINTS k JOIN INFORMATION_SCHEMA.CHECK_CONSTRAINTS c` +
				` ON c.CONSTRAINT_SCHEMA = k.CONSTRAINT_SCHEMA AND c.CONSTRAINT_NAME = k.CONSTRAINT_NAME` +
				` WHERE k.DOMAIN_SCHEMA = d.DOMAIN_SCHEMA AND k.DOMAIN_NAME = d.DOMAIN_NAME), '') AS "constraints"`),
			always(`, ` + text + ` AS "access"`),
			always(`, ` + text + ` AS "comment"`),
			always(`FROM INFORMATION_SCHEMA.DOMAINS d`),
			always(`WHERE ` + notSystem("d.DOMAIN_SCHEMA")),
			always(`AND ` + like("d.DOMAIN_SCHEMA", "@schema")),
			always(`AND ` + like("d.DOMAIN_NAME", "@name")),
			always(`ORDER BY 2, 3`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog", Desc: catalogDesc},
			{Name: "schema"}, {Name: "name"},
			{Name: "data_type", Desc: "the type with its length, or its precision and scale"},
			{Name: "collation", Desc: "empty for a domain that is not a character type, which has no collation"},
			{Name: "nullable", Desc: "always true: a domain has no NOT NULL of its own in HSQLDB, and a check says it, which constraints holds"},
			{Name: "default", Desc: "the expression, which keeps its quotes"},
			{Name: "constraints", Desc: "the CHECK text of every constraint of the domain, joined by AND. Empty where the domain has none"},
			{Name: "access", Desc: "always absent: Privileges reads the grants"},
			{Name: "comment", Desc: "always absent: COMMENT ON takes no domain in HSQLDB"},
		},
		Params: schemaAndName("domain"),
		Scan: func(rows *sql.Rows) (dbmeta.Domain, error) {
			var v dbmeta.Domain
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Name, &v.DataType, dbmeta.NullAsEmpty(&v.Collation),
				&v.Nullable, &v.Default, &v.Constraints, &v.Access, &v.Comment)
			return v, err
		},
	})
}
