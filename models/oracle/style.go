package oracle

import (
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/xo/dbmeta"
	"github.com/xo/dbmeta/dump"
)

// The statement style of the dump. See D274, D280 and docs/DUMP.md.

func init() {
	dump.RegisterStyle(dbmeta.Oracle, dump.Style{
		Quote:         quoteIdentifier,
		Literal:       literal,
		DefaultSchema: "",
		QualifyAll:    true,
		// A schema is a user, so a script makes one with no way to log in. It is
		// where the objects live, and the owner of the script gives the password.
		ExistingSchemas: []string{"SYS", "SYSTEM"},
		CreateSchema: func(quoted string) string {
			return "CREATE USER " + quoted + " NO AUTHENTICATION QUOTA UNLIMITED ON USERS"
		},
		SchemaSetup: func(quoted string) []string {
			return []string{"GRANT CREATE MATERIALIZED VIEW, CREATE TABLE TO " + quoted}
		},
		Preamble:            []string{"ALTER SESSION SET TIME_ZONE = 'UTC'"},
		RoutinesAfterTables: true,
		Column:              column,
		TableOptions:        tableOptions,
		ObjectTables:        true,
		Principal:           principal,
		Grant:               grant,
		TableKeyword:        tableKeyword,
		MaterializedViewLog: materializedViewLog,
		EventTrigger: func(e dump.EventTriggerDef, _ func(string) string, _ func(string) string) string {
			return strings.TrimRight(strings.TrimSpace(e.Definition), " \n\t")
		},
		LostValues: func(c dump.Column) bool { return isRef(c.Type.Raw) },
		ObjectGrant: func(kind, object, grantee string) string {
			if kind == "routine" {
				return "GRANT EXECUTE ON " + object + " TO " + grantee
			}
			return "GRANT SELECT ON " + object + " TO " + grantee
		},
		DeferrableKeys:   true,
		Sequence:         sequence,
		Index:            index,
		MaterializedView: materializedView,
		Comment:          comment,
		PartitionBy:      partitionBy,
		UseSchema:        func(quoted string) string { return "ALTER SESSION SET CURRENT_SCHEMA = " + quoted },
		IdentityAfter:    identityAfter,
		KeepTerminator:   keepTerminator,
		SelectColumn:     selectColumn,
		ExpressionColumn: expressionColumn,
		Binary:           binary,
		Synonym:          synonym,
		LongBinary:       longBinary,
		ReferenceGrant: func(parent, child string) string {
			return "GRANT REFERENCES ON " + parent + " TO " + child
		},
		Block:            block,
		Boolean:          boolean,
		FloatSpecial:     floatSpecial,
		TimeLiteral:      timeLiteral,
		ZoneInTimestamps: true,
		KeepOffset:       true,
		// Both sides read and write an instant in UTC.
		Session: []string{"ALTER SESSION SET TIME_ZONE = 'UTC'"},
		// A statement holds one row, because a release before 23ai has no list of
		// rows in VALUES.
		MaxRows:       1,
		MaxParameters: 65535,
		NoRestrict:    true,
	})
}

// quoteIdentifier quotes a name with double quotes, which keeps its case.
func quoteIdentifier(name string) string {
	return dbmeta.QuoteIdentifier(name, `"`, `"`)
}

// literal writes a string literal. A literal holds at most 4000 bytes, so a longer
// text is a CLOB that joins pieces.
func literal(s string) string {
	if len(s) <= 4000 {
		return "'" + strings.ReplaceAll(s, "'", "''") + "'"
	}
	var parts []string
	for len(s) > 0 {
		n := 0
		size := 0
		for _, r := range s {
			if size+len(string(r)) > 2000 {
				break
			}
			size += len(string(r))
			n += len(string(r))
		}
		parts = append(parts, "TO_CLOB('"+strings.ReplaceAll(s[:n], "'", "''")+"')")
		s = s[n:]
	}
	return strings.Join(parts, " || ")
}

// binary writes a RAW value from its bytes. A value of more than 2000 bytes is a
// variable, which longBinary makes.
func binary(hex string) string {
	return "HEXTORAW('" + hex + "')"
}

// longBinary makes a temporary BLOB of the bytes, in pieces that a PL/SQL literal
// holds, and names it.
func longBinary(n int, hex string) (string, []string, string) {
	name := "b" + strconv.Itoa(n)
	fill := []string{"DBMS_LOB.CREATETEMPORARY(" + name + ", TRUE);"}
	for len(hex) > 0 {
		size := min(len(hex), 30000)
		fill = append(fill, "DBMS_LOB.APPEND("+name+", TO_BLOB(HEXTORAW('"+hex[:size]+"')));")
		hex = hex[size:]
	}
	return name + " BLOB;", fill, name
}

// block runs an INSERT in a PL/SQL block with the variables that it names.
func block(declare, fill []string, stmt string) string {
	return "DECLARE\n  " + strings.Join(declare, "\n  ") + "\nBEGIN\n  " + strings.Join(fill, "\n  ") + "\n  " + stmt + ";\nEND;"
}

// boolean writes 1 or 0, because a release before 23ai has no boolean.
func boolean(v bool) string {
	if v {
		return "1"
	}
	return "0"
}

// floatSpecial writes NaN and the infinities of a binary float.
func floatSpecial(v float64) (string, bool) {
	switch {
	case v != v:
		return "BINARY_DOUBLE_NAN", true
	case v > 0:
		return "BINARY_DOUBLE_INFINITY", true
	}
	return "-BINARY_DOUBLE_INFINITY", true
}

// timeLiteral writes a date or a timestamp with its type, so that the literal does
// not depend on the date format of the session.
func timeLiteral(kind dump.Kind, text string) string {
	switch kind {
	case dump.Date:
		return "DATE " + text
	case dump.TimeOfDay, dump.TimeOfDayOffset:
		return text
	}
	return "TIMESTAMP " + text
}

// column writes a column in the order that Oracle reads: the type, the collation,
// the default or the identity, and NOT NULL. A virtual column has an expression.
// An identity column is BY DEFAULT while the rows load, and the statement after
// them makes it what it was.
func column(c dump.Column, typeSQL string, q func(string) string, lit func(string) string) string {
	var b strings.Builder
	b.WriteString(q(c.Name) + " " + typeSQL)
	if c.Collation != "" {
		b.WriteString(" COLLATE " + c.Collation)
	}
	if c.Invisible {
		b.WriteString(" INVISIBLE")
	}
	switch {
	case c.Generated != "" && c.Default != nil:
		b.WriteString(" GENERATED ALWAYS AS (" + *c.Default + ") VIRTUAL")
		return b.String()
	case c.Identity != "":
		b.WriteString(" GENERATED BY DEFAULT AS IDENTITY")
	case c.Default != nil:
		text := *c.Default
		if c.DefaultLiteral {
			text = lit(text)
		}
		b.WriteString(" DEFAULT " + text)
	}
	if !c.Nullable {
		b.WriteString(" NOT NULL")
	}
	return b.String()
}

// identityAfter gives an identity column its own kind again and the number that
// follows the rows that were loaded, which is the greatest value and one more.
func identityAfter(quotedTable, quotedColumn string, c dump.Column) string {
	kind := "BY DEFAULT"
	if strings.EqualFold(c.Identity, "always") {
		kind = "ALWAYS"
	}
	return "ALTER TABLE " + quotedTable + " MODIFY (" + quotedColumn + " GENERATED " + kind +
		" AS IDENTITY (START WITH LIMIT VALUE))"
}

// sequence writes CREATE SEQUENCE with the bounds that the catalog gives.
func sequence(quoted string, s dump.Sequence) string {
	var b strings.Builder
	b.WriteString("CREATE SEQUENCE " + quoted)
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
		b.WriteString(" NOCYCLE")
	}
	return b.String()
}

// index writes CREATE INDEX. A column of a function based index is its expression.
func index(table string, ix dump.Index, q func(string) string) string {
	cols := make([]string, len(ix.Columns))
	for i, c := range ix.Columns {
		text := q(c.Name)
		if c.Expression != "" {
			text = strings.TrimSpace(c.Expression)
		}
		if c.Descending {
			text += " DESC"
		}
		cols[i] = text
	}
	sql := "CREATE "
	if ix.Unique {
		sql += "UNIQUE "
	}
	if strings.Contains(ix.Type, "bitmap") {
		sql += "BITMAP "
	}
	// The index is in the schema of its table.
	name := q(ix.Name)
	if owner, _, ok := strings.Cut(table, "."); ok {
		name = owner + "." + name
	}
	return sql + "INDEX " + name + " ON " + table + " (" + strings.Join(cols, ", ") + ")"
}

// materializedView writes CREATE MATERIALIZED VIEW. It is built when it is made,
// which is after the rows of its tables are loaded.
func materializedView(quoted string, v dump.View) string {
	options := v.Options
	if options == "" {
		options = "BUILD IMMEDIATE REFRESH COMPLETE ON DEMAND"
	}
	return "CREATE MATERIALIZED VIEW " + quoted + " " + options + " AS " +
		strings.TrimRight(strings.TrimSpace(v.Definition), "; \n\t")
}

// comment writes COMMENT ON for a table or a column.
func comment(target, quoted, text string, literal func(string) string) string {
	return "COMMENT ON " + target + " " + quoted + " IS " + literal(text)
}

// partitionBy writes the clause that splits a table into partitions. An interval makes
// partitions as rows need them, a subpartition template splits each partition again, and a
// table of reference partitioning takes its partitions from its parent.
func partitionBy(p dump.Partitioning, q func(string) string, _ func(string) string) string {
	if p.Strategy == "reference" {
		return " PARTITION BY REFERENCE (" + q(p.Reference) + ")"
	}
	method := strings.ToUpper(p.Strategy)
	sql := " PARTITION BY " + method + " (" + p.Expression + ")"
	if p.Interval != "" {
		sql += " INTERVAL (" + p.Interval + ")"
	}
	if p.SubStrategy != "" {
		sql += " SUBPARTITION BY " + strings.ToUpper(p.SubStrategy) + " (" + p.SubKey + ")"
		if p.SubTemplate != "" {
			sql += " SUBPARTITION TEMPLATE (" + p.SubTemplate + ")"
		}
	}
	var bound string
	switch {
	case strings.Contains(method, "RANGE"):
		bound = "LESS THAN"
	case strings.Contains(method, "LIST"):
		bound = ""
	default:
		if len(p.Parts) > 0 {
			sql += " PARTITIONS " + strconv.Itoa(len(p.Parts))
		}
		return sql
	}
	parts := make([]string, len(p.Parts))
	for i, part := range p.Parts {
		parts[i] = "PARTITION " + q(part.Name) + " VALUES " + strings.TrimSpace(bound+" ("+part.Bound+")")
		if len(part.Subs) > 0 {
			parts[i] += " (" + strings.Join(part.Subs, ", ") + ")"
		}
	}
	return sql + " (" + strings.Join(parts, ", ") + ")"
}

// endBlock matches the end of a PL/SQL block, which has a semicolon of its own.
var endBlock = regexp.MustCompile(`(?is)\bend\s*(?:[A-Za-z0-9_$#"]+\s*)?;$`)

// keepTerminator says that the semicolon belongs to the text, because the text
// ends a block.
func keepTerminator(def string) bool {
	return endBlock.MatchString(strings.TrimSpace(def))
}

// synonym writes CREATE SYNONYM for an object, which can be in another database
// through a link.
func synonym(quoted string, y dump.Synonym, q func(string) string) string {
	target := q(y.Target)
	if y.TargetSchema != "" {
		target = q(y.TargetSchema) + "." + target
	}
	if y.TargetServer != "" {
		target += "@" + q(y.TargetServer)
	}
	return "CREATE OR REPLACE SYNONYM " + quoted + " FOR " + target
}

// tableOptions writes what follows the columns: an index organized table keeps its
// rows in its primary key, and a nested table column has the table that stores it.
func tableOptions(t dump.Table, _ func(string) string) string {
	var b strings.Builder
	if t.Organization == "index" {
		b.WriteString(" ORGANIZATION INDEX")
	}
	// A temporary table says what happens to its rows when the transaction ends.
	for option := range strings.SplitSeq(t.Options, ", ") {
		switch option {
		case "on_commit=delete_rows":
			b.WriteString(" ON COMMIT DELETE ROWS")
		case "on_commit=preserve_rows":
			b.WriteString(" ON COMMIT PRESERVE ROWS")
		}
	}
	// The storage options that the model lists when they are not the default.
	moves := false
	for option := range strings.SplitSeq(t.Options, ", ") {
		name, value, _ := strings.Cut(option, "=")
		switch name {
		case "nologging":
			b.WriteString(" NOLOGGING")
		case "pctfree":
			b.WriteString(" PCTFREE " + value)
		case "compress":
			b.WriteString(" COMPRESS")
		case "row_movement":
			moves = true
		}
	}
	for _, c := range t.Columns {
		if c.Storage != "" {
			b.WriteString(" NESTED TABLE " + quoteIdentifier(c.Name) + " STORE AS " + quoteIdentifier(c.Storage))
		}
	}
	// ENABLE ROW MOVEMENT is the last clause, after the partitions.
	if moves && t.Partitioning == nil {
		b.WriteString(" ENABLE ROW MOVEMENT")
	}
	return b.String()
}

// materializedViewLog writes CREATE MATERIALIZED VIEW LOG with what each entry holds.
func materializedViewLog(master string, l dump.MaterializedViewLog) string {
	var with []string
	if l.PrimaryKey {
		with = append(with, "PRIMARY KEY")
	}
	if l.RowID {
		with = append(with, "ROWID")
	}
	if l.ObjectID {
		with = append(with, "OBJECT ID")
	}
	if l.Sequence {
		with = append(with, "SEQUENCE")
	}
	sql := "CREATE MATERIALIZED VIEW LOG ON " + master
	if len(with) > 0 || l.Columns != "" {
		sql += " WITH " + strings.Join(with, ", ")
	}
	if l.Columns != "" {
		sql += " (" + l.Columns + ")"
	}
	if l.IncludeNew {
		sql += " INCLUDING NEW VALUES"
	}
	return sql
}

// tableKeyword says GLOBAL TEMPORARY for a table that keeps its rows for a session or a
// transaction.
func tableKeyword(t dump.Table) string {
	if strings.Contains(t.Options, "on_commit=") {
		return "GLOBAL TEMPORARY TABLE"
	}
	return "TABLE"
}

// principal makes a user or a role that a grant names when it is not there. A user has no
// way to log in. The statements ignore the error for a name that exists.
func principal(name, kind string, q func(string) string) string {
	create, code := "CREATE USER "+q(name)+" NO AUTHENTICATION", "-1920"
	if kind == "role" {
		create, code = "CREATE ROLE "+q(name), "-1921"
	}
	return "BEGIN EXECUTE IMMEDIATE '" + strings.ReplaceAll(create, "'", "''") +
		"'; EXCEPTION WHEN OTHERS THEN IF SQLCODE != " + code + " THEN RAISE; END IF; END;"
}

// grant writes the grants of one object, with the privileges of each grantee and the ones
// that it can pass on apart. A privilege on a column is given one column at a time.
func grant(_, _, _, qualified string, grants []dump.GrantDef, q func(string) string) []string {
	type who struct {
		grantee string
		option  bool
	}
	var order []who
	whole := map[who][]string{}
	columns := map[who]map[string][]string{}
	for _, gr := range grants {
		w := who{gr.Grantee, gr.Option}
		if _, ok := whole[w]; !ok && columns[w] == nil {
			order = append(order, w)
		}
		if gr.Column == "" {
			whole[w] = append(whole[w], gr.Privilege)
			continue
		}
		if columns[w] == nil {
			columns[w] = map[string][]string{}
		}
		columns[w][gr.Privilege] = append(columns[w][gr.Privilege], q(gr.Column))
	}
	var out []string
	for _, w := range order {
		to := "PUBLIC"
		if w.grantee != "" {
			to = q(w.grantee)
		}
		tail := " ON " + qualified + " TO " + to
		if w.option {
			tail += " WITH GRANT OPTION"
		}
		if privs := whole[w]; len(privs) > 0 {
			out = append(out, "GRANT "+strings.Join(privs, ", ")+tail)
		}
		var parts []string
		for priv, list := range columns[w] {
			parts = append(parts, priv+" ("+strings.Join(list, ", ")+")")
		}
		slices.Sort(parts)
		if len(parts) > 0 {
			out = append(out, "GRANT "+strings.Join(parts, ", ")+tail)
		}
	}
	return out
}
