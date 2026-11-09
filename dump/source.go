package dump

import (
	"context"
	"fmt"
	"strings"

	"github.com/xo/dbmeta"
)

// sourceKey names a stored object whose source a product keeps by line.
type sourceKey struct{ schema, kind, name string }

// sourceTexts joins the lines of the source of each stored object. It is empty for
// a product that does not keep its source this way.
func (r *reader) sourceTexts(ctx context.Context, a dbmeta.Args) (map[sourceKey]string, error) {
	if r.source != nil {
		return r.source, nil
	}
	r.source = map[sourceKey]string{}
	if dbmeta.Source.Support(r.m) != dbmeta.Supported {
		return r.source, nil
	}
	lines, err := collect(dbmeta.Source.Each(ctx, r.m, r.db, a))
	if err != nil {
		return nil, fmt.Errorf("reading source: %w", err)
	}
	built := map[sourceKey][]string{}
	for _, l := range lines {
		k := sourceKey{l.Schema, l.Type, l.Name}
		built[k] = append(built[k], l.Text)
	}
	for k, parts := range built {
		r.source[k] = strings.Join(parts, "")
	}
	return r.source, nil
}

// routineKinds are the kinds of stored object that a script writes from their
// source, in the order that a body follows its specification.
var routineKinds = []string{"type", "type body", "package", "package body", "function", "procedure"}

// readSourceRoutines makes a routine of each stored object that has source. It
// replaces the routines of the model for a product that keeps source by line,
// because the model does not give the text of a package or of a type.
func (r *reader) readSourceRoutines(ctx context.Context, a dbmeta.Args, schemas map[string]*Schema) (bool, error) {
	source, err := r.sourceTexts(ctx, a)
	if err != nil {
		return false, err
	}
	if len(source) == 0 {
		return false, nil
	}
	for k, text := range source {
		s, ok := schemas[k.schema]
		if !ok || k.kind == "trigger" {
			continue
		}
		s.Routines = append(s.Routines, Routine{
			Name: k.name, Kind: k.kind, Language: "PL/SQL", Definition: "CREATE OR REPLACE " + strings.TrimSpace(text),
		})
	}
	return true, nil
}
