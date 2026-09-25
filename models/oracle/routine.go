package oracle

import (
	"database/sql"

	"github.com/xo/dbmeta"
)

// procedureStmt builds the statement behind both \df and \da.
//
// Oracle keeps a function, a procedure and an aggregate in one view, and
// all_procedures.aggregate is the only thing that separates the last from the
// first two. The two queries are otherwise the same statement, so they are
// written once.
//
// A package member is left out. all_procedures reports one row per package
// and one more per member, with procedure_name naming the member, and psql
// lists a routine rather than a routine inside a container. A member is
// reachable by naming the package.
func procedureStmt(aggregate string) dbmeta.Stmt {
	return dbmeta.Stmt{
		always(`SELECT SYS_CONTEXT('USERENV', 'DB_NAME') AS "catalog"`),
		always(`, p.owner AS "schema"`),
		always(`, p.object_name AS "name"`),
		always(`, TO_CHAR(p.object_id) AS "id"`),
		always(`, LOWER(p.object_type) AS "kind"`),
		// The return type is argument zero, which is the row Oracle writes
		// for a function and does not write for a procedure.
		always(`, NVL((SELECT a.data_type FROM all_arguments a`),
		always(`  WHERE a.owner = p.owner AND a.object_id = p.object_id`),
		always(`  AND a.data_level = 0 AND a.position = 0), '') AS "result_type"`),
		always(`, NVL((SELECT `),
		listagg("a.data_type", "a.position"),
		always(`  FROM all_arguments a`),
		always(`  WHERE a.owner = p.owner AND a.object_id = p.object_id`),
		always(`  AND a.data_level = 0 AND a.position > 0), '') AS "arg_types"`),
		// Oracle has two states where PostgreSQL has three. DETERMINISTIC is
		// the promise that the same arguments give the same answer, which is
		// what immutable means. There is no stable in between.
		always(`, CASE p.deterministic WHEN 'YES' THEN 'immutable'` +
			` ELSE 'volatile' END AS "volatility"`),
		always(`, CASE p.parallel WHEN 'YES' THEN 'safe' ELSE 'unsafe' END AS "parallel"`),
		always(`, p.owner AS "owner"`),
		always(`, CASE p.authid WHEN 'CURRENT_USER' THEN 'invoker'` +
			` ELSE 'definer' END AS "security"`),
		always(`, NULL AS "access"`),
		always(`, CASE p.interface WHEN 'YES' THEN 'EXTERNAL' ELSE 'PL/SQL' END AS "language"`),
		always(`, NULL AS "source"`),
		always(`, NULL AS "comment"`),
		always(`FROM all_procedures p`),
		always(`WHERE p.procedure_name IS NULL`),
		always(`AND p.object_type IN ('FUNCTION', 'PROCEDURE')`),
		always(`AND p.aggregate = '` + aggregate + `'`),
		notSystem("AND", "p.owner"),
		always(`AND (@schema IS NULL OR p.owner LIKE @schema)`),
		always(`AND (@name IS NULL OR p.object_name LIKE @name)`),
		always(`ORDER BY p.owner, p.object_name`),
	}
}

// procedureFields are the fields both routine queries return.
func procedureFields() []dbmeta.Field {
	return []dbmeta.Field{
		{Name: "catalog"}, {Name: "schema"}, {Name: "name"},
		{Name: "id", Desc: "the object id, which Oracle numbers rather than names"},
		{Name: "kind"},
		{Name: "result_type", Desc: "empty for a procedure, which returns nothing"},
		{Name: "arg_types"},
		{
			Name: "volatility",
			Desc: "immutable for a DETERMINISTIC routine and volatile for every" +
				" other: Oracle has no stable in between",
		},
		{Name: "parallel"}, {Name: "owner"}, {Name: "security"},
		{
			Name: "access",
			Desc: "always absent: Oracle records a grant as a row, which the" +
				" privileges query returns",
		},
		{Name: "language"},
		{
			Name: "source",
			Desc: "always absent: Oracle keeps the body in all_source as one" +
				" row per line, which no single column can hold",
		},
		{Name: "comment", Desc: "always absent: Oracle records no comment on a routine"},
	}
}

// scanFunction reads one routine row.
func scanFunction(rows *sql.Rows) (dbmeta.Function, error) {
	var v dbmeta.Function
	err := rows.Scan(&v.Catalog, &v.Schema, &v.Name, &v.ID, &v.Kind, &v.ResultType,
		&v.ArgTypes, &v.Volatility, &v.Parallel, &v.Owner, &v.Security, &v.Access,
		&v.Language, &v.Source, &v.Comment)
	return v, err
}

func registerRoutines() {
	// \df.
	dbmeta.Functions.Register(dbmeta.Oracle, &dbmeta.Binding[dbmeta.Function]{
		Stmt:   procedureStmt("NO"),
		Fields: procedureFields(),
		Params: schemaNameSystem("routine"),
		Scan:   scanFunction,
	})

	// \da. An Oracle aggregate is a function with an implementation type
	// behind it, and the view flags it.
	dbmeta.Aggregates.Register(dbmeta.Oracle, &dbmeta.Binding[dbmeta.Function]{
		Stmt:   procedureStmt("YES"),
		Fields: procedureFields(),
		Params: schemaNameSystem("aggregate"),
		Scan:   scanFunction,
	})

	// The arguments of a routine, flat, which is what D47 asks for rather
	// than a slice on the routine.
	dbmeta.RoutineParameters.Register(dbmeta.Oracle,
		&dbmeta.Binding[dbmeta.RoutineParameter]{
			Stmt: dbmeta.Stmt{
				always(`SELECT SYS_CONTEXT('USERENV', 'DB_NAME') AS "catalog"`),
				always(`, a.owner AS "schema"`),
				always(`, a.object_name AS "routine"`),
				always(`, TO_CHAR(a.object_id) AS "routine_id"`),
				always(`, a.argument_name AS "name"`),
				always(`, a.position AS "ordinal"`),
				always(`, a.in_out AS "mode"`),
				always(`, a.data_type AS "data_type"`),
				// default_value is a LONG. A LONG cannot be joined, compared
				// or wrapped, and is selected bare and scanned as text, the
				// same way all_tab_cols.data_default is.
				always(`, a.default_value AS "default"`),
				always(`FROM all_arguments a`),
				// data_level above zero is a field of a record argument, and
				// position zero is the return value rather than an argument.
				always(`WHERE a.package_name IS NULL AND a.data_level = 0 AND a.position > 0`),
				notSystem("AND", "a.owner"),
				always(`AND (@schema IS NULL OR a.owner LIKE @schema)`),
				always(`AND (@name IS NULL OR a.object_name LIKE @name)`),
				always(`ORDER BY a.owner, a.object_name, a.position`),
			},
			Fields: []dbmeta.Field{
				{Name: "catalog"}, {Name: "schema"}, {Name: "routine"},
				{Name: "routine_id"},
				{Name: "name", Desc: "absent for an argument the caller passes by position"},
				{Name: "ordinal"}, {Name: "mode"}, {Name: "data_type"},
				{Name: "default", Desc: "the DEFAULT text, which Oracle stores as a LONG"},
			},
			Params: schemaNameSystem("routine"),
			Scan: func(rows *sql.Rows) (dbmeta.RoutineParameter, error) {
				var v dbmeta.RoutineParameter
				err := rows.Scan(&v.Catalog, &v.Schema, &v.Routine, &v.RoutineID,
					&v.Name, &v.Ordinal, &v.Mode, &v.DataType, &v.Default)
				return v, err
			},
		})

	// \dT.
	dbmeta.Types.Register(dbmeta.Oracle, &dbmeta.Binding[dbmeta.Type]{
		Stmt: dbmeta.Stmt{
			always(`SELECT SYS_CONTEXT('USERENV', 'DB_NAME') AS "catalog"`),
			always(`, t.owner AS "schema"`),
			always(`, t.type_name AS "name"`),
			always(`, t.type_name AS "internal"`),
			always(`, LOWER(t.typecode) AS "kind"`),
			// A collection is the one kind with an element type, and Oracle
			// keeps it in a view of its own.
			always(`, NVL((SELECT c.elem_type_name FROM all_coll_types c`),
			always(`  WHERE c.owner = t.owner AND c.type_name = t.type_name), '') AS "elements"`),
			always(`, t.owner AS "owner"`),
			always(`, NULL AS "access"`),
			always(`, NULL AS "comment"`),
			always(`FROM all_types t`),
			notSystem("WHERE", "t.owner"),
			always(`AND (@schema IS NULL OR t.owner LIKE @schema)`),
			always(`AND (@name IS NULL OR t.type_name LIKE @name)`),
			always(`ORDER BY t.owner, t.type_name`),
		},
		Fields: []dbmeta.Field{
			{Name: "catalog"}, {Name: "schema"}, {Name: "name"},
			{
				Name: "internal",
				Desc: "the same as the name: Oracle keeps one name for a type",
			},
			{Name: "kind"},
			{Name: "elements", Desc: "the element type of a collection, empty for every other kind"},
			{Name: "owner"},
			{
				Name: "access",
				Desc: "always absent: Oracle records a grant as a row, which the" +
					" privileges query returns",
			},
			{Name: "comment", Desc: "always absent: Oracle records no comment on a type"},
		},
		Params: schemaNameSystem("type"),
		Scan: func(rows *sql.Rows) (dbmeta.Type, error) {
			var v dbmeta.Type
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Name, &v.Internal, &v.Kind,
				&v.Elements, &v.Owner, &v.Access, &v.Comment)
			return v, err
		},
	})

	// \dD. The SQL domain arrived in 23ai, and all_domains with it. Every
	// fragment gates on 23, so an older release produces no statement and the
	// query reports that the server is too old rather than a wrong answer.
	dbmeta.Domains.Register(dbmeta.Oracle, &dbmeta.Binding[dbmeta.Domain]{
		Stmt: dbmeta.Stmt{
			{{Min: v23, SQL: `SELECT SYS_CONTEXT('USERENV', 'DB_NAME') AS "catalog"`}},
			{{Min: v23, SQL: `, d.owner AS "schema"`}},
			{{Min: v23, SQL: `, d.name AS "name"`}},
			{{Min: v23, SQL: `, NVL(c.data_type, '') AS "data_type"`}},
			{{Min: v23, SQL: `, NVL(c.collation, '') AS "collation"`}},
			{{Min: v23, SQL: `, CASE c.nullable WHEN 'N' THEN 0 ELSE 1 END AS "nullable"`}},
			{{Min: v23, SQL: `, c.data_default AS "default"`}},
			{{Min: v23, SQL: `, '' AS "constraints"`}},
			{{Min: v23, SQL: `, NULL AS "access"`}},
			{{Min: v23, SQL: `, NULL AS "comment"`}},
			{{Min: v23, SQL: `FROM all_domains d`}},
			// A domain over several columns has a row per column. The first
			// is the one a single column domain has, which is every domain a
			// consumer prints beside a column type.
			{{Min: v23, SQL: `LEFT JOIN all_domain_cols c ON c.owner = d.owner` +
				` AND c.domain_name = d.name AND c.column_id = 1`}},
			notSystemAt(v23, "WHERE", "d.owner"),
			{{Min: v23, SQL: `AND (@schema IS NULL OR d.owner LIKE @schema)`}},
			{{Min: v23, SQL: `AND (@name IS NULL OR d.name LIKE @name)`}},
			{{Min: v23, SQL: `ORDER BY d.owner, d.name`}},
		},
		Fields: []dbmeta.Field{
			{Name: "catalog"}, {Name: "schema"}, {Name: "name"},
			{Name: "data_type", Desc: "the type of the first column of the domain"},
			{Name: "collation"}, {Name: "nullable"},
			{Name: "default", Desc: "the DEFAULT text, which Oracle stores as a LONG"},
			{
				Name: "constraints",
				Desc: "always empty: Oracle records a domain constraint as a row" +
					" rather than as one text",
			},
			{
				Name: "access",
				Desc: "always absent: Oracle records a grant as a row, which the" +
					" privileges query returns",
			},
			{Name: "comment", Desc: "always absent: Oracle records no comment on a domain"},
		},
		Params: schemaNameSystem("domain"),
		Scan: func(rows *sql.Rows) (dbmeta.Domain, error) {
			var v dbmeta.Domain
			err := rows.Scan(&v.Catalog, &v.Schema, &v.Name, &v.DataType, &v.Collation,
				&v.Nullable, &v.Default, &v.Constraints, &v.Access, &v.Comment)
			return v, err
		},
	})

	// \do. An Oracle operator is a name with one binding per argument list,
	// and a binding is what carries the types, so the row is the binding.
	dbmeta.Operators.Register(dbmeta.Oracle, &dbmeta.Binding[dbmeta.Operator]{
		Stmt: dbmeta.Stmt{
			always(`SELECT o.owner AS "schema"`),
			always(`, o.operator_name AS "name"`),
			always(`, NVL((SELECT g.argument_type FROM all_oparguments g`),
			always(`  WHERE g.owner = b.owner AND g.operator_name = b.operator_name`),
			always(`  AND g.binding# = b.binding# AND g.position = 1), '') AS "left_type"`),
			always(`, NVL((SELECT g.argument_type FROM all_oparguments g`),
			always(`  WHERE g.owner = b.owner AND g.operator_name = b.operator_name`),
			always(`  AND g.binding# = b.binding# AND g.position = 2), '') AS "right_type"`),
			always(`, NVL(b.return_type, '') AS "result_type"`),
			always(`, NVL(b.function_name, '') AS "function"`),
			always(`, NULL AS "comment"`),
			always(`FROM all_operators o`),
			always(`JOIN all_opbindings b ON b.owner = o.owner` +
				` AND b.operator_name = o.operator_name`),
			notSystem("WHERE", "o.owner"),
			always(`AND (@schema IS NULL OR o.owner LIKE @schema)`),
			always(`AND (@name IS NULL OR o.operator_name LIKE @name)`),
			always(`ORDER BY o.owner, o.operator_name, b.binding#`),
		},
		Fields: []dbmeta.Field{
			{Name: "schema"}, {Name: "name"},
			{
				Name: "left_type",
				Desc: "the first argument: an Oracle operator is a prefix call," +
					" so there is no left and right in the PostgreSQL sense",
			},
			{Name: "right_type", Desc: "the second argument, empty where the binding takes one"},
			{Name: "result_type"}, {Name: "function"},
			{Name: "comment", Desc: "always absent: Oracle records no comment on an operator"},
		},
		Params: schemaNameSystem("operator"),
		Scan: func(rows *sql.Rows) (dbmeta.Operator, error) {
			var v dbmeta.Operator
			err := rows.Scan(&v.Schema, &v.Name, &v.LeftType, &v.RightType,
				&v.ResultType, &v.Function, &v.Comment)
			return v, err
		},
	})
}
