// Package all registers the metadata models.
//
// Import it for its effect, then use the root package:
//
//	import _ "github.com/xo/dbmeta/all"
//
// Which models are registered depends on the build tags. The default is the
// base set, which is every model. Build with `no_<model>` to leave one out,
// with `no_base` for none, and with `no_base` and a model name for that model
// alone. A model can always be imported directly instead, from
// `models/<driver>`.
//
// This package exists because a model imports the root package to
// register, so the root package cannot import a model back. See D31.
package all
