package dbmeta

import "fmt"

// errWrap wraps err the way the package does, so that a test can check that
// [errors.Is] still reaches the constant.
func errWrap(err error) error {
	return fmt.Errorf("reading columns for %s: %w", "some_table", err)
}
