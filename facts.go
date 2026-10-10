package dbmeta

// unmodeled holds what a client needs to know about a dialect that has no
// model, or has one and leaves a field empty. A model fills [Info], and an
// Info field that is set wins over a row here. A row exists only for a product
// that usql reaches and that dbmeta has no model for, so that the fact
// has one home. See D230.
var unmodeled = map[Dialect]unmodeledInfo{
	CSVQ:     {Placeholder: questionMark},
	DynamoDB: {Product: "Amazon DynamoDB"},
	Pinot:    {Product: "Apache Pinot"},
}

// unmodeledInfo is the part of [Info] that a dialect with no model can have.
type unmodeledInfo struct {
	Product     string
	Placeholder func(n int) string
}

// questionMark writes the placeholder of a product that numbers none.
func questionMark(int) string { return "?" }

// Placeholder writes the bind parameter for position n, counting from 1, as
// the product writes it: $1, ?, :1 or @p1. The second result is false when the
// dialect has no model and no row of its own, or when its model sets no
// [Info.Placeholder]. It is the form that a client writes into an INSERT
// statement, as usql does for \copy.
//
// A product that cannot bind in a statement of dbmeta, such as Apache Hive and
// Snowflake, writes its values with [Info.Literal] there. This method still
// returns the placeholder that a client writes in its own statement.
func (d Dialect) Placeholder(n int) (string, bool) {
	if info, ok := d.Info(); ok && info.Placeholder != nil {
		return info.Placeholder(n), true
	}
	if row, ok := unmodeled[d]; ok && row.Placeholder != nil {
		return row.Placeholder(n), true
	}
	return "", false
}

// Product returns the text to show for the version of a product that has no
// statement that reads its version, such as Amazon DynamoDB and Apache Pinot.
// The second result is false when the dialect has no such text, which is
// every dialect that has a [Info.VersionQuery].
//
// A model can set [Info.Product] and a dialect with no model has a row of its
// own, so this works for both. See D230.
func (d Dialect) Product() (string, bool) {
	if info, ok := d.Info(); ok && info.Product != "" {
		return info.Product, true
	}
	if row, ok := unmodeled[d]; ok && row.Product != "" {
		return row.Product, true
	}
	return "", false
}

// EveryStatementIsAQuery reports whether every statement of the product's
// language returns a result and the server sends no count, so that a client
// runs each statement as a query and never as an exec. See
// [Info.EveryStatementIsAQuery].
//
// It is false for a dialect that no model was built for.
func (d Dialect) EveryStatementIsAQuery() bool {
	info, ok := d.Info()
	return ok && info.EveryStatementIsAQuery
}

// WritesNeedAutocommit reports whether a write must run outside a
// transaction. See [Info.WritesNeedAutocommit].
//
// It is false for a dialect that no model was built for.
func (d Dialect) WritesNeedAutocommit() bool {
	info, ok := d.Info()
	return ok && info.WritesNeedAutocommit
}

// ScanTypes reports whether a client must scan each column into the Go type
// that the driver reports. See [Info.ScanTypes].
//
// It is false for a dialect that no model was built for.
func (d Dialect) ScanTypes() bool {
	info, ok := d.Info()
	return ok && info.ScanTypes
}
