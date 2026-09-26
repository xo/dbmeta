package dbmeta

// Error is an error.
type Error string

// Error satisfies the error interface.
func (err Error) Error() string {
	return string(err)
}

// Error values.
const (
	// ErrNotSupported is the not supported error. The model exists and the
	// database cannot answer for that object.
	ErrNotSupported Error = "not supported"
	// ErrModelNotBuilt is the model not built error. The model was left out of
	// this binary by a build tag. It says nothing about what the database can
	// do.
	ErrModelNotBuilt Error = "model not built"
	// ErrNotFound is the not found error. The database has no object of that
	// name. An empty result means the same thing and is not an error.
	ErrNotFound Error = "not found"
	// ErrPermissionDenied is the permission denied error. The connected user
	// cannot read that part of the catalog.
	ErrPermissionDenied Error = "permission denied"
	// ErrVersionTooOld is the version too old error. The server is older than
	// the oldest version the model supports.
	ErrVersionTooOld Error = "version too old"
	// ErrInvalidVersion is the invalid version error.
	ErrInvalidVersion Error = "invalid version"
	// ErrEmptyQuery is the empty query error. A statement resolved to no SQL.
	ErrEmptyQuery Error = "empty query"
	// ErrUnknownParam is the unknown parameter error. An argument was given a
	// name the statement does not take. It is never ignored, because ignoring
	// it turns a typo into a query that silently drops a filter.
	ErrUnknownParam Error = "unknown parameter"
	// ErrMissingParam is the missing parameter error. The statement names a
	// parameter that the arguments do not supply.
	ErrMissingParam Error = "missing parameter"
	// ErrInvalidParam is the invalid parameter error. A value cannot be
	// rendered into a statement by a dialect that has no way to bind one.
	// Only Apache Hive can return it, because only Hive renders literals,
	// and it returns one rather than guessing at a value it cannot escape
	// safely. See [Info.Literal] and D78.
	ErrInvalidParam Error = "invalid parameter"
	// ErrQuotingUnknown is the quoting unknown error. Building a statement
	// that carries a password needs session state the caller did not read,
	// and guessing it can let the password escape its own literal. See D56.
	ErrQuotingUnknown Error = "quoting unknown"
	// ErrInvalidPassword is the invalid password error. A password or a user
	// name holds a character no product can carry in a statement, which today
	// means a NUL.
	ErrInvalidPassword Error = "invalid password"
	// ErrAmbiguousFragment is the ambiguous fragment error. Two alternatives
	// of one choice name different versions and the server reports both, so
	// nothing decides between them. It is a fault in the model, not in the
	// database or the call. See D44.
	ErrAmbiguousFragment Error = "ambiguous fragment"
)
