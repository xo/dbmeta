package dbmeta

import "fmt"

// Describing the columns of a result without running the query, which is what
// psql's \gdesc shows. See D273.
//
// Like [Dialect.ChangePassword], the two methods here return the text of a
// statement and run nothing. They take no database, so the caller runs the
// statement, and everything dbmeta executes is still a read. Both statements
// only read, too: the first asks the server to describe a query and does not
// run it, and the second names types.
//
// There are two methods because products divide the work two ways. SQL Server
// describes a query in one statement, with the type names in SQL. PostgreSQL
// describes it in the protocol, which a driver reaches and a statement does
// not, and gives only the number of each type, so a second statement turns the
// numbers into the names that format_type gives.

// ResultDescription is a statement that describes the columns of a query, and
// the names of the columns of the rows it returns.
type ResultDescription struct {
	// Stmt is the statement to run. It does not run the query it describes.
	Stmt string
	// Name is the column of the rows that holds the name of a result column.
	// The value is NULL for a column that has no name.
	Name string
	// Type is the column of the rows that holds the SQL name of its type, such
	// as varchar(10).
	Type string
}

// TypeRef is the type of one result column as a driver reports it.
type TypeRef struct {
	// OID is the number of the type.
	OID uint32
	// Modifier is the modifier of the type, such as the length of a
	// varchar(10) plus four. It is -1 when the type has none.
	Modifier int32
}

// DescribeResult returns the statement that describes the first result of
// query without running it.
//
// It describes one statement. A batch is described by its first statement, so
// the caller refuses anything that is not a single statement. It returns
// [ErrModelNotBuilt] when the model is not built, [ErrNotSupported] when the
// product cannot describe a query in one statement, and [ErrMissingParam] for
// an empty query. SQL Server is the one product that answers. A caller with
// another product describes the result through its driver, or by running the
// query with no rows wanted, which it decides, because that runs the query.
func (d Dialect) DescribeResult(query string) (ResultDescription, error) {
	info, ok := d.Info()
	if !ok {
		return ResultDescription{}, ErrModelNotBuilt
	}
	if info.DescribeResult == nil {
		return ResultDescription{}, ErrNotSupported
	}
	if query == "" {
		return ResultDescription{}, fmt.Errorf("describing a result: %w", ErrMissingParam)
	}
	return info.DescribeResult(query)
}

// TypeNames returns the statement that names the types of the columns of a
// result, and does not run it. The rows have two columns, ord and type. The
// first is the place of the type in types, counted from 1, and the second is
// its name as the product spells it in SQL, such as "character varying(10)".
//
// It is for a product that describes a result with numbers, which is
// PostgreSQL and the products that speak its protocol and keep format_type.
// It returns [ErrNotSupported] for any other and [ErrMissingParam] for an
// empty list.
func (d Dialect) TypeNames(types []TypeRef) (string, error) {
	info, ok := d.Info()
	if !ok {
		return "", ErrModelNotBuilt
	}
	if info.TypeNames == nil {
		return "", ErrNotSupported
	}
	if len(types) == 0 {
		return "", fmt.Errorf("naming types: %w", ErrMissingParam)
	}
	return info.TypeNames(types)
}
