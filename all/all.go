// Package all registers the metadata models.
//
// Import it for its effect, then use the root package:
//
//	import _ "github.com/xo/dbmeta/all"
//
// Which models are registered depends on the build tags. The default is the
// base set. Build with `most` or `all` for more, with a model name for one
// more, and with `no_base` for none. A model can always be imported directly
// instead, from `models/<driver>`.
//
// This package exists because a model imports the root package in order to
// register, so the root package cannot import a model back. See D31.
package all
