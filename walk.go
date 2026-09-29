package dbmeta

import "strings"

// withDefaults returns args with the default of every parameter it leaves
// out, for a query that Binding.Walk answers. An argument the query does not
// take is ErrUnknownParam, and a parameter with no default that args leaves
// out is ErrMissingParam, as they are for a statement.
func withDefaults(params []Param, args map[string]any) (map[string]any, error) {
	known := make(map[string]bool, len(params))
	for _, p := range params {
		known[p.Name] = true
	}
	for name := range args {
		if !known[name] {
			return nil, ErrUnknownParam
		}
	}
	out := make(map[string]any, len(params))
	for _, p := range params {
		v, ok := args[p.Name]
		switch {
		case ok:
			// A list is joined by commas, as bind joins it for a statement.
			if list, isList := v.([]string); isList {
				v = strings.Join(list, ",")
			}
			out[p.Name] = v
		case p.Default != nil:
			out[p.Name] = p.Default
		default:
			return nil, ErrMissingParam
		}
	}
	return out, nil
}

// Like reports whether s matches pattern the way SQL's LIKE does: % is any
// run of characters, _ is one, and a backslash makes the next character its
// own. An empty pattern matches everything, which is what every parameter
// here means by empty. It is for a query that Binding.Walk answers, whose
// statements cannot take the pattern, so the rows are matched here. See D146.
func Like(pattern, s string) bool {
	if pattern == "" {
		return true
	}
	p, t := []rune(pattern), []rune(s)
	var match func(i, j int) bool
	match = func(i, j int) bool {
		for i < len(p) {
			switch c := p[i]; c {
			case '%':
				for i < len(p) && p[i] == '%' {
					i++
				}
				if i == len(p) {
					return true
				}
				for k := j; k <= len(t); k++ {
					if match(i, k) {
						return true
					}
				}
				return false
			case '_':
				if j == len(t) {
					return false
				}
				i, j = i+1, j+1
			default:
				if c == '\\' && i+1 < len(p) {
					i++
					c = p[i]
				}
				if j == len(t) || t[j] != c {
					return false
				}
				i, j = i+1, j+1
			}
		}
		return j == len(t)
	}
	return match(0, 0)
}
