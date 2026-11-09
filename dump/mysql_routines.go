package dump

import (
	"fmt"
	"strings"

	"github.com/xo/dbmeta"
)

// mysqlRoutineStatement writes the statement that makes a MySQL or MariaDB
// routine, because the catalog keeps the body and the declarations apart and
// has no whole statement. The name has no database, so the statement makes the
// routine in the current one, which a script chooses first. See D293.
func mysqlRoutineStatement(f dbmeta.Function, params []dbmeta.RoutineParameter) (statement, arguments string) {
	if !f.Source.Valid || f.Source.V == "" {
		return "", ""
	}
	quote := func(name string) string { return "`" + strings.ReplaceAll(name, "`", "``") + "`" }
	procedure := f.Kind == "proc"
	var list []string
	for _, p := range params {
		if p.Ordinal == 0 || !p.Name.Valid {
			continue
		}
		text := quote(p.Name.V) + " " + p.DataType
		if procedure {
			text = strings.ToUpper(p.Mode) + " " + text
		}
		list = append(list, text)
	}
	arguments = strings.Join(list, ", ")
	var b strings.Builder
	b.WriteString("CREATE ")
	if f.Owner.Valid {
		if user, host, ok := strings.Cut(f.Owner.V, "@"); ok {
			fmt.Fprintf(&b, "DEFINER = %s@%s ", quote(user), quote(host))
		}
	}
	if procedure {
		b.WriteString("PROCEDURE ")
	} else {
		b.WriteString("FUNCTION ")
	}
	b.WriteString(quote(f.Name) + "(" + arguments + ")")
	if !procedure && f.ResultType.Valid {
		b.WriteString(" RETURNS " + f.ResultType.V)
	}
	if f.Comment.Valid && f.Comment.V != "" {
		b.WriteString(" COMMENT '" + strings.ReplaceAll(strings.ReplaceAll(f.Comment.V, `\`, `\\`), "'", "''") + "'")
	}
	b.WriteString(" LANGUAGE SQL")
	if strings.EqualFold(f.Volatility, "yes") {
		b.WriteString(" DETERMINISTIC")
	} else {
		b.WriteString(" NOT DETERMINISTIC")
	}
	if f.DataAccess.Valid && f.DataAccess.V != "" {
		b.WriteString(" " + strings.ToUpper(f.DataAccess.V))
	}
	if f.Security != "" {
		b.WriteString(" SQL SECURITY " + strings.ToUpper(f.Security))
	}
	b.WriteString(" " + strings.TrimSpace(f.Source.V))
	return b.String(), arguments
}

// duckdbMacroStatement writes the statement that makes a DuckDB macro. The
// catalog keeps the body of a macro and the names of its parameters apart, and
// the body of a table macro is a query. A function that is compiled has no body,
// so it is not written. See D293.
func duckdbMacroStatement(f dbmeta.Function, params []dbmeta.RoutineParameter) (statement, arguments string) {
	if (f.Kind != "macro" && f.Kind != "table_macro") || !f.Source.Valid || f.Source.V == "" {
		return "", ""
	}
	quote := func(name string) string { return `"` + strings.ReplaceAll(name, `"`, `""`) + `"` }
	var names []string
	for _, p := range params {
		if p.Name.Valid {
			names = append(names, quote(p.Name.V))
		}
	}
	arguments = strings.Join(names, ", ")
	as := "AS "
	if f.Kind == "table_macro" {
		as = "AS TABLE "
	}
	return "CREATE OR REPLACE MACRO " + quote(f.Schema) + "." + quote(f.Name) + "(" + arguments + ") " + as +
		strings.TrimSpace(f.Source.V), arguments
}
