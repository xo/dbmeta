package postgres

import (
	"strings"

	"github.com/xo/dbmeta"
)

// Matching the argument types of a function or an operator, which is what the
// type patterns of \df and \do do. See D272.
//
// A caller passes one pattern for each argument in Args.ArgTypes. This file
// turns each pattern into the three things the statement compares: the
// pattern for the name of the type, the pattern for the name that format_type
// gives, and the pattern for the schema of the type. The statement reads them
// from one derived parameter, because it cannot know how many arguments the
// caller names.

const (
	// itemSeparator separates the patterns of two arguments in the derived
	// parameter, and fieldSeparator separates the three parts of one pattern.
	// Neither can appear in a type name, so no escaping is needed. The
	// statement splits on chr(30) and chr(31).
	itemSeparator  = "\x1e"
	fieldSeparator = "\x1f"

	// noMoreArguments is the pattern that says the routine takes no argument
	// at this position.
	noMoreArguments = "-"
)

// typeAliases maps a name that a person types to the name format_type gives.
// It is the table of map_typename_pattern in psql, which reads the same aliases
// before it compares. The names are lower case, because the caller folds the
// case of a name that is not quoted before it calls.
var typeAliases = map[string]string{
	"bool":        "boolean",
	"decimal":     "numeric",
	"float":       "double precision",
	"float4":      "real",
	"float8":      "double precision",
	"int":         "integer",
	"int2":        "smallint",
	"int4":        "integer",
	"int8":        "bigint",
	"time":        "time without time zone",
	"timestamp":   "timestamp without time zone",
	"timestamptz": "timestamp with time zone",
	"timetz":      "time with time zone",
	"varbit":      "bit varying",
	"varchar":     "character varying",
}

// argTypeItem turns one pattern into the three parts the statement compares.
//
// A pattern with a dot names a schema, and psql does not map the alias of a
// type that has one. A pattern that ends in [] is an array type, and the alias
// is read from what is before it, because the internal name of an array type
// is not the one a person types.
func argTypeItem(pattern string) string {
	if pattern == noMoreArguments {
		return noMoreArguments
	}
	schema, name := "", pattern
	if i := strings.IndexByte(pattern, '.'); i >= 0 {
		schema, name = pattern[:i], pattern[i+1:]
	}
	format := name
	if schema == "" {
		base := strings.TrimSuffix(name, "[]")
		if alias, ok := typeAliases[base]; ok {
			format = alias + name[len(base):]
		}
	}
	return schema + fieldSeparator + name + fieldSeparator + format
}

// argTypeItems is the derived parameter arg_type_items. It is empty when the
// caller names no type.
func argTypeItems(args map[string]any) any {
	patterns, _ := args["arg_types"].([]string)
	items := make([]string, len(patterns))
	for i, pattern := range patterns {
		items[i] = argTypeItem(pattern)
	}
	return strings.Join(items, itemSeparator)
}

// argTypeParams is the parameter a statement that matches argument types
// declares.
func argTypeParams(kind string) []dbmeta.Param {
	return append(schemaNameSystem(kind), dbmeta.Param{
		Name:    "arg_types",
		Desc:    "a name pattern for the type of each argument in order, and - for no more arguments, empty for any",
		Default: "",
	})
}

// argTypeDerived is the derived parameter that a statement with argTypeParams
// reads.
func argTypeDerived() []dbmeta.Derived {
	return []dbmeta.Derived{{Name: "arg_type_items", From: argTypeItems}}
}

// argTypeFilter is the condition that every argument matches its pattern.
// argAt is the expression for the type of the argument at the position a.i,
// which starts at 1.
//
// A position past the last argument has no type, so the left join gives NULL
// there, and only the pattern - matches it. A type is matched by its internal
// name or by the name format_type gives. A type in no schema named is matched
// only when it is on the search path, which is what psql does.
func argTypeFilter(argAt string) dbmeta.Choice {
	item := func(n string) string {
		return `pg_catalog.split_part(a.item, pg_catalog.chr(31), ` + n + `)`
	}
	return dbmeta.Choice{{Query: `AND (@arg_type_items = '' OR NOT EXISTS (SELECT 1` +
		` FROM pg_catalog.unnest(pg_catalog.string_to_array(@arg_type_items, pg_catalog.chr(30)))` +
		` WITH ORDINALITY AS a(item, i)` +
		` LEFT JOIN pg_catalog.pg_type ty ON ty.oid = ` + argAt +
		` LEFT JOIN pg_catalog.pg_namespace tn ON tn.oid = ty.typnamespace` +
		` WHERE NOT CASE WHEN a.item = '` + noMoreArguments + `' THEN ty.oid IS NULL ELSE ty.oid IS NOT NULL` +
		` AND (ty.typname LIKE ` + item("2") + ` OR pg_catalog.format_type(ty.oid, NULL) LIKE ` + item("3") + `)` +
		` AND CASE WHEN ` + item("1") + ` = '' THEN pg_catalog.pg_type_is_visible(ty.oid)` +
		` ELSE tn.nspname LIKE ` + item("1") + ` END END))`}}
}
