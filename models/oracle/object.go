package oracle

import (
	"strings"

	"github.com/xo/dbmeta/dump"
)

// A column of an object type or of a collection type has a value that the driver
// cannot read. The dump reads it as the text of the expression that builds the
// value, such as "HR"."POSTAL_ADDRESS"('1 Market St', 'Seattle'), and writes that
// text into the INSERT as it is. See D289.

// expressionColumn says that the column holds a type that a person made.
func expressionColumn(t dump.Type) bool {
	return t.Kind == dump.Other && (strings.HasPrefix(t.Raw, `"`) || isRef(t.Raw))
}

// isRef says that a declared type is a REF, which points at a row of an object
// table.
func isRef(raw string) bool {
	return strings.HasPrefix(strings.TrimSpace(raw), "REF ")
}

// typeName splits "OWNER"."NAME" into its two names.
func typeName(raw string) (owner, name string, ok bool) {
	raw = strings.TrimSpace(raw)
	owner, name, ok = strings.Cut(raw, `"."`)
	if !ok || !strings.HasPrefix(owner, `"`) || !strings.HasSuffix(name, `"`) {
		return "", "", false
	}
	return strings.TrimPrefix(owner, `"`), strings.TrimSuffix(name, `"`), true
}

// selectColumn writes the expression that reads a column of a type that a person
// made, and says that it did so.
func selectColumn(s *dump.Snapshot, schema string, c dump.Column, quoted string) (string, bool) {
	if !expressionColumn(c.Type) {
		return "", false
	}
	b := builder{snap: s}
	return b.value(c.Type.Raw, quoted, 0)
}

// builder writes the expressions of one snapshot.
type builder struct {
	snap *dump.Snapshot
}

// maxDepth is how deep a type can hold another type.
const maxDepth = 8

// value writes the SQL that gives the text of the value of ref, as a literal that
// an INSERT reads, for a column or an attribute of the declared type.
func (b builder) value(declared, ref string, depth int) (string, bool) {
	if depth > maxDepth {
		return "", false
	}
	// A reference holds the identifier of a row of the source, which the copy does
	// not have, so the copy gets no reference.
	if isRef(declared) {
		return "'NULL'", true
	}
	if owner, name, ok := typeName(declared); ok {
		text, found := b.typed(owner, name, ref, depth)
		if !found {
			return "", false
		}
		// A value that is NULL is the word, which an INSERT reads as NULL.
		null := "'NULL'"
		if strings.Contains(text, "XMLAGG(") {
			// A CASE cannot mix text with a CLOB.
			null = "TO_CLOB('NULL')"
		}
		return "CASE WHEN " + ref + " IS NULL THEN " + null + " ELSE " + text + " END", true
	}
	return scalar(declared, ref), true
}

// typed writes the constructor of an object type or of a collection type.
func (b builder) typed(owner, name, ref string, depth int) (string, bool) {
	for _, sc := range b.snap.Schemas {
		if sc.Name != owner {
			continue
		}
		for _, o := range sc.Composites {
			if o.Name != name {
				continue
			}
			parts := make([]string, 0, len(o.Columns))
			for _, a := range o.Columns {
				text, ok := b.value(a.Type.Raw, ref+"."+quote(a.Name), depth+1)
				if !ok {
					return "", false
				}
				parts = append(parts, text)
			}
			return `'` + quote(owner) + "." + quote(name) + `(' || ` + joinText(parts) + ` || ')'`, true
		}
		for _, c := range sc.Collections {
			if c.Name != name {
				continue
			}
			elem := c.Element
			text, ok := b.element(owner, elem, "e.COLUMN_VALUE", depth+1)
			if !ok {
				return "", false
			}
			// The order of the elements is the order that the collection gives them.
			// LISTAGG stops at 4000 bytes, so the elements go through XMLAGG, which makes a CLOB.
			// The text is made of entities, and CONVERT turns them back.
			list := `(SELECT DBMS_XMLGEN.CONVERT(RTRIM(XMLAGG(XMLELEMENT(x, ` + text + `, ',') ORDER BY ROWNUM).EXTRACT('//text()').GETCLOBVAL(), ','), 1)` +
				` FROM TABLE(` + ref + `) e)`
			return `'` + quote(owner) + "." + quote(name) + `(' || ` + list + ` || ')'`, true
		}
	}
	return "", false
}

// element writes the text of one element of a collection, which names its type
// without its length.
func (b builder) element(owner, elem, ref string, depth int) (string, bool) {
	for _, sc := range b.snap.Schemas {
		if sc.Name != owner {
			continue
		}
		for _, o := range sc.Composites {
			if o.Name == elem {
				return b.typed(owner, elem, ref, depth)
			}
		}
		for _, c := range sc.Collections {
			if c.Name == elem {
				return b.typed(owner, elem, ref, depth)
			}
		}
	}
	return scalar(elem, ref), true
}

// scalar writes the literal of a value of one of the types of Oracle.
func scalar(declared, ref string) string {
	up := strings.ToUpper(strings.TrimSpace(declared))
	null := func(text string) string {
		return "CASE WHEN " + ref + " IS NULL THEN 'NULL' ELSE " + text + " END"
	}
	switch {
	case strings.HasPrefix(up, "VARCHAR"), strings.HasPrefix(up, "NVARCHAR"), strings.HasPrefix(up, "CHAR"),
		strings.HasPrefix(up, "NCHAR"), strings.HasPrefix(up, "CLOB"):
		return null(`'''' || REPLACE(` + ref + `, '''', '''''') || ''''`)
	case strings.HasPrefix(up, "TIMESTAMP") && strings.Contains(up, "TIME ZONE"):
		return null(`'TIMESTAMP ''' || TO_CHAR(` + ref + `, 'YYYY-MM-DD HH24:MI:SS.FF9 TZH:TZM') || ''''`)
	case strings.HasPrefix(up, "TIMESTAMP"):
		return null(`'TIMESTAMP ''' || TO_CHAR(` + ref + `, 'YYYY-MM-DD HH24:MI:SS.FF9') || ''''`)
	case up == "DATE":
		return null(`'TIMESTAMP ''' || TO_CHAR(` + ref + `, 'YYYY-MM-DD HH24:MI:SS') || ''''`)
	case strings.HasPrefix(up, "RAW"):
		return null(`'HEXTORAW(''' || RAWTOHEX(` + ref + `) || ''')'`)
	}
	// A number, which a literal writes with a point and no group separator.
	return null(`TO_CHAR(` + ref + `, 'TM9', 'NLS_NUMERIC_CHARACTERS=''.,''')`)
}

// joinText joins the parts of a constructor with commas.
func joinText(parts []string) string {
	if len(parts) == 0 {
		return "''"
	}
	return strings.Join(parts, ` || ',' || `)
}

// quote quotes a name, as the catalog keeps it.
func quote(name string) string { return `"` + strings.ReplaceAll(name, `"`, `""`) + `"` }
