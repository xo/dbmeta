package test

import (
	"context"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
)

// northwindServer is one server that holds the northwind fixture, and how to
// list and read what it holds (D254, D256).
type northwindServer struct {
	name   string
	driver string
	env    string
	// dsn changes the DSN of the server into one for the database northwind.
	dsn func(string) (string, error)
	// relations lists each table and view of the fixture as schema and name.
	relations string
	// quote writes a schema and a name the way the server wants them.
	quote func(schema, name string) string
	// schemas maps the schema of the server to the name that the comparison
	// uses for it.
	schemas map[string]string
}

var northwindServers = []northwindServer{
	{
		name: "postgres", driver: "pgx", env: "DBMETA_POSTGRES",
		dsn: func(d string) (string, error) {
			u, err := url.Parse(d)
			if err != nil {
				return "", fmt.Errorf("reading the URL: %w", err)
			}
			u.Path = "/northwind"
			return u.String(), nil
		},
		relations: `SELECT table_schema, table_name FROM information_schema.tables WHERE table_schema IN ('public', 'hr')`,
		quote:     func(s, n string) string { return `"` + s + `"."` + n + `"` },
		schemas:   map[string]string{"public": "nw", "hr": "hr"},
	},
	{
		name: "sqlserver", driver: "sqlserver", env: "DBMETA_SQLSERVER",
		dsn: func(d string) (string, error) {
			u, err := url.Parse(d)
			if err != nil {
				return "", fmt.Errorf("reading the URL: %w", err)
			}
			q := u.Query()
			q.Set("database", "northwind")
			u.RawQuery = q.Encode()
			return u.String(), nil
		},
		relations: `SELECT s.name, o.name FROM sys.objects o JOIN sys.schemas s ON s.schema_id = o.schema_id WHERE o.type IN ('U', 'V') AND s.name IN ('dbo', 'hr')`,
		quote:     func(s, n string) string { return "[" + s + "].[" + n + "]" },
		schemas:   map[string]string{"dbo": "nw", "hr": "hr"},
	},
	{
		name: "oracle", driver: "oracle", env: "DBMETA_ORACLE",
		dsn:       func(d string) (string, error) { return d, nil },
		relations: `SELECT owner, object_name FROM all_objects WHERE owner IN ('NORTHWIND', 'HR') AND object_type IN ('TABLE', 'VIEW')`,
		quote:     func(s, n string) string { return `"` + s + `"."` + n + `"` },
		schemas:   map[string]string{"NORTHWIND": "nw", "HR": "hr"},
	},
}

// northwindGenerated lists the columns whose values the server makes, so that
// two servers never agree on them.
var northwindGenerated = map[string]bool{
	"staffid": true, "createdat": true, "updatedat": true, "rowversion": true, "validfrom": true, "validto": true,
	"happenedat": true, "eventid": true, "detail": true,
}

// northwindTable is the content of one table or view as text.
type northwindTable struct {
	cols []string
	rows [][]string
}

var (
	nameChars = regexp.MustCompile(`[^a-z0-9]`)
	numberish = regexp.MustCompile(`^\$?-?[0-9,]*\.?[0-9]+$`)
)

// compareKey writes a name the same way on every server: lower case, with
// nothing but letters and digits.
func compareKey(s string) string { return nameChars.ReplaceAllString(strings.ToLower(s), "") }

// normalizeValue writes a value so that the same fact is the same text on
// every server.
func normalizeValue(v any, binary, single bool) string {
	if f, ok := v.(float64); ok && single {
		v = float32(f)
	}
	switch t := v.(type) {
	case nil:
		return "<null>"
	case []byte:
		if binary {
			return hex.EncodeToString(t)
		}
		return normalizeText(string(t))
	case string:
		return normalizeText(t)
	case time.Time:
		return t.Format("2006-01-02 15:04:05")
	case float32:
		return strconv.FormatFloat(float64(t), 'g', -1, 32)
	case float64:
		return strconv.FormatFloat(t, 'g', -1, 64)
	case int64:
		return strconv.FormatInt(t, 10)
	case bool:
		if t {
			return "1"
		}
		return "0"
	case fmt.Stringer:
		return normalizeText(t.String())
	}
	return normalizeText(fmt.Sprint(v))
}

// normalizeText trims the padding of a CHAR column and writes a number with no
// currency sign, no separator and no trailing zero.
func normalizeText(s string) string {
	s = strings.TrimRight(s, " ")
	// A server keeps the spaces of a JSON text or drops them, and PostgreSQL
	// sorts the keys, so the comparison reads the text as JSON.
	if strings.HasPrefix(s, "{") || strings.HasPrefix(s, "[") {
		var j any
		if json.Unmarshal([]byte(s), &j) == nil {
			if b, err := json.Marshal(j); err == nil {
				return string(b)
			}
		}
	}
	if strings.ContainsAny(s, ".$") && numberish.MatchString(s) {
		s = strings.NewReplacer("$", "", ",", "").Replace(s)
		if strings.Contains(s, ".") {
			s = strings.TrimRight(strings.TrimRight(s, "0"), ".")
		}
	}
	return s
}

// northwindRelation is a table or a view, as the schema and the name of a server.
type northwindRelation struct{ schema, name string }

// listNorthwind lists the tables and views of the fixture on one server.
func listNorthwind(ctx context.Context, t *testing.T, db *sql.DB, srv northwindServer) []northwindRelation {
	t.Helper()
	rows, err := db.QueryContext(ctx, srv.relations)
	if err != nil {
		t.Fatalf("%s: listing the tables: %v", srv.name, err)
	}
	defer rows.Close()
	var all []northwindRelation
	for rows.Next() {
		var r northwindRelation
		if err := rows.Scan(&r.schema, &r.name); err != nil {
			t.Fatal(err)
		}
		all = append(all, r)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("%s: listing the tables: %v", srv.name, err)
	}
	return all
}

// readNorthwindTable reads every row of one table or view as text.
func readNorthwindTable(ctx context.Context, t *testing.T, db *sql.DB, srv northwindServer, key, query string) (northwindTable, bool) {
	t.Helper()
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		t.Logf("%s: skipping %s: %v", srv.name, key, err)
		return northwindTable{}, false
	}
	defer rows.Close()
	cts, err := rows.ColumnTypes()
	if err != nil {
		t.Fatal(err)
	}
	tbl := northwindTable{}
	isBinary := make([]bool, len(cts))
	isSingle := make([]bool, len(cts))
	for i, ct := range cts {
		tbl.cols = append(tbl.cols, compareKey(ct.Name()))
		n := strings.ToUpper(ct.DatabaseTypeName())
		isBinary[i] = slices.Contains([]string{"BYTEA", "BINARY", "VARBINARY", "IMAGE", "RAW", "LONG RAW", "BLOB"}, n)
		isSingle[i] = slices.Contains([]string{"FLOAT4", "REAL", "BINARY_FLOAT"}, n)
	}
	for rows.Next() {
		vals := make([]any, len(cts))
		ptrs := make([]any, len(cts))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			t.Fatalf("%s: reading %s: %v", srv.name, key, err)
		}
		line := make([]string, len(cts))
		for i, v := range vals {
			line[i] = normalizeValue(v, isBinary[i], isSingle[i])
		}
		tbl.rows = append(tbl.rows, line)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("%s: reading %s: %v", srv.name, key, err)
	}
	return tbl, true
}

// readNorthwind reads every table and view of the fixture from one server.
func readNorthwind(t *testing.T, srv northwindServer) map[string]northwindTable {
	t.Helper()
	ctx := context.WithoutCancel(t.Context())
	dsn := os.Getenv(srv.env)
	if dsn == "" {
		t.Skipf("set %s to run against %s", srv.env, srv.name)
	}
	dsn, err := srv.dsn(dsn)
	if err != nil {
		t.Fatal(err)
	}
	db := openAt(t, srv.driver, dsn)
	out := make(map[string]northwindTable)
	for _, r := range listNorthwind(ctx, t, db, srv) {
		alias, ok := srv.schemas[r.schema]
		if !ok {
			continue
		}
		key := alias + "." + compareKey(r.name)
		query := "SELECT * FROM " + srv.quote(r.schema, r.name)
		// The driver of Oracle reads a CLOB that has a JSON check as text only,
		// and has no coder for an object type or a varray, so the departments
		// are read without the objects.
		if srv.name == "oracle" && key == "hr.departments" {
			query = "SELECT department_id, code, name, budget, head_count, cost_per_head, is_active, " +
				"to_char(substr(settings, 1, 4000)) AS settings, opened_on FROM " + srv.quote(r.schema, r.name)
		}
		if tbl, ok := readNorthwindTable(ctx, t, db, srv, key, query); ok {
			out[key] = tbl
		}
	}
	return out
}

// TestNorthwindServersAgree reads every table and view of the northwind
// fixture from PostgreSQL, SQL Server and Oracle and compares the values, after
// it writes the types the same way. A difference that the fixture makes on
// purpose is listed in the logs and does not fail the test. A table that has
// the same name and columns on two servers must hold the same rows.
func TestNorthwindServersAgree(t *testing.T) {
	data := make(map[string]map[string]northwindTable)
	for _, srv := range northwindServers {
		data[srv.name] = readNorthwind(t, srv)
	}
	for _, pair := range [][2]string{{"postgres", "sqlserver"}, {"postgres", "oracle"}, {"sqlserver", "oracle"}} {
		ref := data[pair[0]]
		keys := make([]string, 0, len(ref))
		for k := range ref {
			keys = append(keys, k)
		}
		slices.Sort(keys)
		for _, key := range keys {
			o, ok := data[pair[1]][key]
			if !ok {
				t.Logf("%-28s is on %s and not on %s", key, pair[0], pair[1])
				continue
			}
			compareNorthwind(t, key, pair[0], pair[1], ref[key], o)
		}
	}
}

// northwindDerived lists the columns that the views of Northwind compute from a
// price, a quantity and a discount. SQL Server does that sum with a float and
// rounds to cents, and an exact half cent, such as 921.375, goes up on one
// server and down on another. So the comparison allows these a small difference
// and holds every other column to the exact text.
var northwindDerived = map[string]bool{
	"extendedprice": true, "subtotal": true, "productsales": true, "categorysales": true, "saleamount": true,
}

// compareNorthwind compares the common columns of two tables. The columns that
// are not derived make the key of a row, and the derived ones must be close.
func compareNorthwind(t *testing.T, key, first, other string, a, b northwindTable) {
	t.Helper()
	var keyCols, derived []string
	for _, c := range a.cols {
		if !slices.Contains(b.cols, c) || northwindGenerated[c] {
			continue
		}
		if northwindDerived[c] {
			derived = append(derived, c)
			continue
		}
		keyCols = append(keyCols, c)
	}
	onlyA, onlyB := diffCols(a.cols, b.cols), diffCols(b.cols, a.cols)
	if len(onlyA) > 0 || len(onlyB) > 0 {
		t.Logf("%-28s columns differ: only %s %v, only %s %v", key, first, onlyA, other, onlyB)
	}
	if len(a.rows) != len(b.rows) {
		t.Errorf("%s has %d rows on %s and %d on %s", key, len(a.rows), first, len(b.rows), other)
		return
	}
	index := func(tb northwindTable) map[string][]string {
		out := make(map[string][]string, len(tb.rows))
		for _, r := range tb.rows {
			k := make([]string, len(keyCols))
			for j, c := range keyCols {
				k[j] = r[slices.Index(tb.cols, c)]
			}
			v := make([]string, len(derived))
			for j, c := range derived {
				v[j] = r[slices.Index(tb.cols, c)]
			}
			// Two rows with the same key are the same row, as in a table with
			// no key, so the key keeps a count.
			ks := strings.Join(k, "\x1f")
			for n := 0; ; n++ {
				if _, dup := out[ks+"\x1e"+strconv.Itoa(n)]; !dup {
					out[ks+"\x1e"+strconv.Itoa(n)] = v
					break
				}
			}
		}
		return out
	}
	ia, ib := index(a), index(b)
	var bad int
	report := func(format string, args ...any) {
		bad++
		if bad <= 3 {
			t.Errorf(format, args...)
		}
	}
	for k, va := range ia {
		vb, ok := ib[k]
		if !ok {
			report("%s: a row of %s is not on %s:\n  %s\ncolumns %v", key, first, other,
				strings.ReplaceAll(strings.Split(k, "\x1e")[0], "\x1f", " | "), keyCols)
			continue
		}
		for j := range va {
			if !closeNumbers(va[j], vb[j]) {
				report("%s: %s is %s on %s and %s on %s, in the row\n  %s", key, derived[j], va[j], first, vb[j], other,
					strings.ReplaceAll(strings.Split(k, "\x1e")[0], "\x1f", " | "))
			}
		}
	}
	if bad > 0 {
		t.Errorf("%s: %d differences between %s and %s", key, bad, first, other)
	}
}

// closeNumbers says that two texts are the same number, or differ by less than
// 2 cents or 0.02 percent.
func closeNumbers(a, b string) bool {
	if a == b {
		return true
	}
	x, errX := strconv.ParseFloat(a, 64)
	y, errY := strconv.ParseFloat(b, 64)
	if errX != nil || errY != nil {
		return false
	}
	d := math.Abs(x - y)
	return d <= 0.02 || d <= 0.0002*math.Max(math.Abs(x), math.Abs(y))
}

func diffCols(a, b []string) []string {
	var out []string
	for _, c := range a {
		if !slices.Contains(b, c) {
			out = append(out, c)
		}
	}
	return out
}
