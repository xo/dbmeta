package sqlserver

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/xo/dbmeta"
	"github.com/xo/dbmeta/dump"
)

// The statement style of the dump. See D274, D280 and docs/DUMP.md.

func init() {
	dump.RegisterStyle(dbmeta.SQLServer, dump.Style{
		Quote:             quoteIdentifier,
		Literal:           literal,
		DefaultSchema:     "dbo",
		ExistingSchemas:   []string{"dbo", "guest", "sys", "INFORMATION_SCHEMA"},
		CreateSchema:      func(quoted string) string { return "CREATE SCHEMA " + quoted },
		Column:            column,
		Sequence:          sequence,
		Index:             index,
		TriggerState:      triggerState,
		EventTrigger:      eventTrigger,
		Policies:          policies,
		XMLSchema:         xmlSchema,
		PartitionFunction: partitionFunction,
		PartitionScheme:   partitionScheme,
		PartitionBy:       partitionBy,
		SequenceValue:     sequenceValue,
		TriggerPlacement:  triggerPlacement,
		View:              view,
		KeyType:           keyType,
		PrimaryKeyNotNull: true,
		TypedNullBinary:   true,
		NoRestrict:        true,
		Binary:            func(hex string) string { return "0x" + hex },
		Boolean: func(v bool) string {
			if v {
				return "1"
			}
			return "0"
		},
		IdentityInsert: func(quotedTable string, on bool) string {
			if on {
				return "SET IDENTITY_INSERT " + quotedTable + " ON"
			}
			return "SET IDENTITY_INSERT " + quotedTable + " OFF"
		},
		Comment:             comment,
		KeyKind:             keyKind,
		MixedEndianUUID:     true,
		Domain:              aliasType,
		RoutinesAfterTables: true,
		TableType:           tableTypeSQL,
		Synonym:             synonym,
		SystemTime:          systemTime,
		VersioningOff:       versioningOff,
		VersioningOn:        versioningOn,
		CastBinary:          castBinary,
		ZoneInTimestamps:    true,
		// A table value constructor takes at most 1000 rows, and a statement takes
		// at most 2100 parameters.
		MaxRows:       1000,
		MaxParameters: 2100,
	})
}

// quoteIdentifier quotes a name with brackets.
func quoteIdentifier(name string) string {
	return dbmeta.QuoteIdentifier(name, "[", "]")
}

// literal writes a string literal as a Unicode string, so that no character is
// lost to the code page of the database.
func literal(s string) string {
	return "N'" + strings.ReplaceAll(s, "'", "''") + "'"
}

// column writes a column in the order that SQL Server reads: the type, the
// collation, IDENTITY, the nullability and the default. A computed column has an
// expression and no type.
func column(c dump.Column, typeSQL string, q func(string) string, lit func(string) string) string {
	if c.Generated != "" && c.Default != nil {
		text := q(c.Name) + " AS (" + *c.Default + ")"
		if c.Generated == "stored" || c.Generated == "s" {
			text += " PERSISTED"
		}
		return text
	}
	var b strings.Builder
	b.WriteString(q(c.Name) + " " + typeSQL)
	switch c.Generated {
	case "row start":
		b.WriteString(" GENERATED ALWAYS AS ROW START")
	case "row end":
		b.WriteString(" GENERATED ALWAYS AS ROW END")
	}
	if c.Collation != "" {
		b.WriteString(" COLLATE " + c.Collation)
	}
	if c.Identity != "" {
		b.WriteString(" IDENTITY(1,1)")
	}
	if c.Nullable {
		b.WriteString(" NULL")
	} else {
		b.WriteString(" NOT NULL")
	}
	if c.Default != nil {
		text := *c.Default
		if c.DefaultLiteral {
			text = lit(text)
		}
		// A default that a person named keeps its name.
		if c.DefaultName != "" {
			b.WriteString(" CONSTRAINT " + q(c.DefaultName))
		}
		b.WriteString(" DEFAULT " + text)
	}
	return b.String()
}

// sequence writes CREATE SEQUENCE with the bounds that the catalog gives.
func sequence(quoted string, s dump.Sequence) string {
	var b strings.Builder
	b.WriteString("CREATE SEQUENCE " + quoted)
	if s.DataType != "" {
		b.WriteString(" AS " + s.DataType)
	}
	for _, f := range []struct{ word, value string }{
		{"START WITH", s.Start}, {"INCREMENT BY", s.Increment}, {"MINVALUE", s.Minimum}, {"MAXVALUE", s.Maximum},
	} {
		if f.value != "" {
			b.WriteString(" " + f.word + " " + f.value)
		}
	}
	if s.Cycles {
		b.WriteString(" CYCLE")
	} else {
		b.WriteString(" NO CYCLE")
	}
	return b.String()
}

// index writes CREATE INDEX with the columns that are keys, the columns that are
// only included, and the filter.
func index(table string, ix dump.Index, q func(string) string) string {
	var keys, include []string
	for _, c := range ix.Columns {
		text := q(c.Name)
		if c.Descending {
			text += " DESC"
		}
		if c.Include {
			include = append(include, q(c.Name))
			continue
		}
		keys = append(keys, text)
	}
	sql := "CREATE "
	if ix.Unique {
		sql += "UNIQUE "
	}
	switch ix.Type {
	case "clustered":
		sql += "CLUSTERED "
	case "nonclustered":
		sql += "NONCLUSTERED "
	case "clustered columnstore":
		return "CREATE CLUSTERED COLUMNSTORE INDEX " + q(ix.Name) + " ON " + table
	case "nonclustered columnstore":
		sql = "CREATE NONCLUSTERED COLUMNSTORE "
		// A columnstore index has no included columns. The catalog marks each of
		// its columns as included.
		keys, include = nil, nil
		for _, c := range ix.Columns {
			keys = append(keys, q(c.Name))
		}
	}
	sql += "INDEX " + q(ix.Name) + " ON " + table + " (" + strings.Join(keys, ", ") + ")"
	if len(include) > 0 {
		sql += " INCLUDE (" + strings.Join(include, ", ") + ")"
	}
	if ix.Predicate != "" {
		sql += " WHERE " + ix.Predicate
	}
	return sql
}

// keyType gives a text or a binary value of no length a length, because SQL Server
// cannot key a column of type max. The length is the longest that a nonclustered
// key holds on every release, which is 900 bytes.
func keyType(t dump.Type) (dump.Rendering, bool) {
	switch {
	case t.Kind == dump.String && len(t.Values) == 0 && (t.Length == 0 || t.Length > 450):
		return dump.Rendering{SQL: "nvarchar(450)", Level: dump.Lossy,
			Note: "SQL Server cannot key a text longer than 450 characters, so the column is an nvarchar of 450"}, true
	case t.Kind == dump.Binary && (t.Length == 0 || t.Length > 900):
		return dump.Rendering{SQL: "varbinary(900)", Level: dump.Lossy,
			Note: "SQL Server cannot key a binary value longer than 900 bytes, so the column is a varbinary of 900"}, true
	}
	return dump.Rendering{}, false
}

// viewHead is what SQL Server keeps of a view before its query: CREATE VIEW, the
// name, a list of column names and the options. The catalog keeps the whole
// statement as it was written, with the check option inside it.
var viewHead = regexp.MustCompile(`(?is)^\s*CREATE\s+VIEW\s+(?:\[[^\]]+\]|\w+)(?:\s*\.\s*(?:\[[^\]]+\]|\w+))*` +
	`\s*(\([^)]*\))?\s*(?:WITH\s+[A-Za-z_, ]+?\s+)?AS\s+`)

// view writes CREATE VIEW with the name of the target and the rest of the
// statement that the source kept.
func view(quoted string, v dump.View) string {
	def := strings.TrimRight(strings.TrimSpace(v.Definition), "; \n\t")
	loc := viewHead.FindStringSubmatchIndex(def)
	if loc == nil {
		return "CREATE VIEW " + quoted + " AS " + def
	}
	columns := ""
	if loc[2] >= 0 {
		columns = " " + def[loc[2]:loc[3]]
	}
	return "CREATE VIEW " + quoted + columns + " AS " + def[loc[1]:]
}

// systemTime writes the period of a system versioned table. The table has it
// while versioning is off, so that the server accepts the period columns.
func systemTime(v dump.Versioning, q func(string) string) string {
	return "PERIOD FOR SYSTEM_TIME (" + q(v.Start) + ", " + q(v.End) + ")"
}

// versioningOff drops the period, because the server refuses an explicit value
// for a period column while the table has one.
func versioningOff(quotedTable string) string {
	return "ALTER TABLE " + quotedTable + " DROP PERIOD FOR SYSTEM_TIME"
}

// versioningOn makes the period again and turns versioning on. The server checks
// that no row of the history table overlaps the current row.
func versioningOn(quotedTable, quotedHistory string, v dump.Versioning, q func(string) string) []string {
	return []string{
		"ALTER TABLE " + quotedTable + " ADD PERIOD FOR SYSTEM_TIME (" + q(v.Start) + ", " + q(v.End) + ")",
		"ALTER TABLE " + quotedTable + " SET (SYSTEM_VERSIONING = ON (HISTORY_TABLE = " + quotedHistory + "))",
	}
}

// castBinary names the type that a binary value is cast to. The spatial types and
// hierarchyid are types of the common language runtime, and the server reads
// their binary form back only through a cast.
func castBinary(t dump.Type) string {
	switch strings.ToLower(t.Raw) {
	case "geography", "geometry", "hierarchyid":
		return strings.ToLower(t.Raw)
	}
	return ""
}

// aliasType writes CREATE TYPE ... FROM for an alias type. The base type has its
// length, and the type has no check and no default of its own.
func aliasType(quoted string, d dump.Domain, _ func(string) string, _ func(string) string) string {
	sql := "CREATE TYPE " + quoted + " FROM " + d.Type
	if d.NotNull {
		sql += " NOT NULL"
	} else {
		sql += " NULL"
	}
	return sql
}

// tableTypeSQL writes CREATE TYPE ... AS TABLE with the columns and the keys, and
// the indexes that no key owns.
func tableTypeSQL(quoted string, parts []string, t dump.Table, q func(string) string) string {
	for _, ix := range t.Indexes {
		if ix.Constraint != "" {
			continue
		}
		var keys []string
		for _, c := range ix.Columns {
			if c.Include {
				continue
			}
			text := q(c.Name)
			if c.Descending {
				text += " DESC"
			}
			keys = append(keys, text)
		}
		unique := ""
		if ix.Unique {
			unique = "UNIQUE "
		}
		parts = append(parts, "INDEX "+q(ix.Name)+" "+unique+"("+strings.Join(keys, ", ")+")")
	}
	return "CREATE TYPE " + quoted + " AS TABLE (\n  " + strings.Join(parts, ",\n  ") + "\n)"
}

// keyKind says that a primary key is stored as a nonclustered index, which is not
// the default for a key, or that a unique key is stored as a clustered index,
// which is not the default for a unique key either.
func keyKind(kind string, ix dump.Index) string {
	switch {
	case kind == "primary key" && ix.Type == "nonclustered":
		return " NONCLUSTERED"
	case kind == "unique" && ix.Type == "clustered":
		return " CLUSTERED"
	}
	return ""
}

// bracketed matches one name in brackets, where a closing bracket is doubled.
var bracketed = regexp.MustCompile(`\[((?:[^\]]|\]\])+)\]`)

// comment writes the MS_Description extended property, which is what SQL Server
// has instead of a comment. The name is the quoted name that the generator writes,
// with a schema unless the table is in dbo.
func comment(target, quoted, text string, literal func(string) string) string {
	var names []string
	for _, m := range bracketed.FindAllStringSubmatch(quoted, -1) {
		names = append(names, strings.ReplaceAll(m[1], "]]", "]"))
	}
	want := 2
	if target == "COLUMN" {
		want = 3
	}
	// A name in the default schema has no schema in front of it.
	if len(names) < want {
		names = append([]string{"dbo"}, names...)
	}
	sql := "EXEC sp_addextendedproperty @name = N'MS_Description', @value = " + literal(text) +
		", @level0type = N'SCHEMA', @level0name = " + literal(names[0]) +
		", @level1type = N'TABLE', @level1name = " + literal(names[1])
	if target == "COLUMN" {
		sql += ", @level2type = N'COLUMN', @level2name = " + literal(names[2])
	}
	return sql
}

// synonym writes CREATE SYNONYM for an object, which can be in another database
// or on another server.
func synonym(quoted string, y dump.Synonym, q func(string) string) string {
	var parts []string
	for _, p := range []string{y.TargetServer, y.TargetDatabase, y.TargetSchema} {
		if p != "" {
			parts = append(parts, q(p))
		}
	}
	// A name with a database and no schema has an empty part in the middle.
	if y.TargetSchema == "" && y.TargetDatabase != "" {
		parts = append(parts, "")
	}
	parts = append(parts, q(y.Target))
	return "CREATE SYNONYM " + quoted + " FOR " + strings.Join(parts, ".")
}

// triggerState switches a trigger off. A trigger belongs to the schema of its
// table.
func triggerState(quotedSchema, quotedTable, quotedTrigger, state string) string {
	if state != "disabled" {
		return ""
	}
	// A trigger of the database has no table.
	if quotedTable == "" {
		return "DISABLE TRIGGER " + quotedTrigger + " ON DATABASE"
	}
	return "DISABLE TRIGGER " + quotedSchema + "." + quotedTrigger + " ON " + quotedTable
}

// eventTrigger is the statement of the trigger on the database, which SQL Server keeps.
func eventTrigger(e dump.EventTriggerDef, _ func(string) string, _ func(string) string) string {
	return strings.TrimSpace(e.Definition)
}

// predicate takes the brackets off a predicate that the catalog wraps in them, because
// the statement names the function and its arguments with no brackets round them.
func predicate(expr string) string {
	expr = strings.TrimSpace(expr)
	if strings.HasPrefix(expr, "(") && strings.HasSuffix(expr, ")") {
		depth := 0
		for i, r := range expr {
			switch r {
			case '(':
				depth++
			case ')':
				depth--
				if depth == 0 && i < len(expr)-1 {
					return expr
				}
			}
		}
		return strings.TrimSpace(expr[1 : len(expr)-1])
	}
	return expr
}

// policies writes a security policy for each name, and adds the predicates of the
// other tables to it.
func policies(schema string, t dump.Table, created map[string]bool, q func(string) string, _ func(string) string) []string {
	var out []string
	for _, p := range t.Policies {
		owner := p.Schema
		if owner == "" {
			owner = schema
		}
		name := q(owner) + "." + q(p.Name)
		kind, expr := "FILTER", p.Using
		switch p.Command {
		case "insert":
			kind, expr = "BLOCK", p.WithCheck
		case "update":
			kind, expr = "BLOCK", p.WithCheck
			if p.WithCheck == "" {
				expr = p.Using
			}
		case "delete":
			kind, expr = "BLOCK", p.Using
		}
		clause := kind + " PREDICATE " + predicate(expr) + " ON " + q(schema) + "." + q(t.Name)
		switch {
		case kind == "BLOCK" && p.Command == "insert":
			clause += " AFTER INSERT"
		case kind == "BLOCK" && p.Command == "update" && p.WithCheck != "":
			clause += " AFTER UPDATE"
		case kind == "BLOCK" && p.Command == "update":
			clause += " BEFORE UPDATE"
		case kind == "BLOCK":
			clause += " BEFORE DELETE"
		}
		if !created[name] {
			state := "ON"
			if p.Disabled {
				state = "OFF"
			}
			out = append(out, "CREATE SECURITY POLICY "+name+" ADD "+clause+" WITH (STATE = "+state+")")
			created[name] = true
			continue
		}
		out = append(out, "ALTER SECURITY POLICY "+name+" ADD "+clause)
	}
	return out
}

// xmlSchema writes CREATE XML SCHEMA COLLECTION from the text of the schemas.
func xmlSchema(quoted string, x dump.XMLSchema, lit func(string) string) string {
	return "CREATE XML SCHEMA COLLECTION " + quoted + " AS " + lit(x.Definition)
}

// partitionFunction writes CREATE PARTITION FUNCTION.
func partitionFunction(f dump.PartitionFunction, q func(string) string) string {
	sql := "CREATE PARTITION FUNCTION " + q(f.Name) + " (" + f.DataType + ") AS RANGE " + strings.ToUpper(f.Range) + " FOR VALUES ("
	return sql + f.Boundaries + ")"
}

// partitionScheme writes CREATE PARTITION SCHEME. The copy has the filegroup PRIMARY
// and the filegroups that a person made are not made again, so every partition is put
// on PRIMARY.
func partitionScheme(sc dump.PartitionScheme, q func(string) string) string {
	return "CREATE PARTITION SCHEME " + q(sc.Name) + " AS PARTITION " + q(sc.Function) + " ALL TO ([PRIMARY])"
}

// partitionBy writes the clause that lays a table out by its scheme.
func partitionBy(p dump.Partitioning, q func(string) string, _ func(string) string) string {
	var cols []string
	for name := range strings.SplitSeq(p.Expression, ", ") {
		cols = append(cols, q(name))
	}
	return " ON " + q(p.Scheme) + "(" + strings.Join(cols, ", ") + ")"
}

// sequenceValue gives the sequence the values that it gave, so that its next value is
// the one after the last. RESTART WITH changes the start value of the sequence, so
// the values are drawn, which leaves the start as it was. A sequence that gave more
// values than a script can draw is restarted.
func sequenceValue(quoted string, s dump.Sequence, _ func(string) string) string {
	last, err1 := strconv.ParseInt(s.Last, 10, 64)
	step, err2 := strconv.ParseInt(s.Increment, 10, 64)
	start, err3 := strconv.ParseInt(s.Start, 10, 64)
	if err1 != nil || err2 != nil {
		return ""
	}
	if err3 == nil && step != 0 && (last-start)%step == 0 && (last-start)/step >= 0 && (last-start)/step < 10000 {
		return "DECLARE @drawn int = 0, @value bigint; WHILE @drawn <= " + strconv.FormatInt((last-start)/step, 10) +
			" BEGIN SET @value = NEXT VALUE FOR " + quoted + "; SET @drawn = @drawn + 1; END"
	}
	return "ALTER SEQUENCE " + quoted + " RESTART WITH " + strconv.FormatInt(last+step, 10)
}

// triggerPlacement writes sp_settriggerorder for each event that the trigger is first or
// last for.
func triggerPlacement(schema, trigger, placement string, q func(string) string) []string {
	var out []string
	for part := range strings.SplitSeq(placement, ", ") {
		order, event, ok := strings.Cut(part, " ")
		if !ok {
			continue
		}
		out = append(out, "EXEC sp_settriggerorder @triggername = N'"+strings.ReplaceAll(q(schema)+"."+q(trigger), "'", "''")+
			"', @order = '"+strings.ToUpper(order[:1])+order[1:]+"', @stmttype = '"+event+"'")
	}
	return out
}
