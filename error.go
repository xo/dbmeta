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
	// ErrEmptyQuery is the empty query error. A query resolved to no SQL.
	ErrEmptyQuery Error = "empty query"
)
