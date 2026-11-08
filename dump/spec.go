package dump

import (
	"fmt"
	"strconv"
	"strings"
)

// Spec is a type text split into its parts, which is the same work for every
// product: a name, the numbers in the brackets, and the words after them.
//
//	numeric(10,2)                     name numeric, args 10 and 2
//	time(3) with time zone            name time, args 3, rest with time zone
//	decimal(10,2) unsigned zerofill   name decimal, args 10 and 2, rest unsigned zerofill
//	character varying(10)[]           name character varying, args 10, one array
//
// The name and the rest are in lower case with the white space between words
// reduced to one space.
type Spec struct {
	Name string
	Args []int
	Rest string
	// Arrays is the number of trailing [] that the text has.
	Arrays int
}

// Key returns the name and the rest as one lower case string, such as time with
// time zone, which is how a product that writes the zone after the brackets
// names the type.
func (s Spec) Key() string {
	if s.Rest == "" {
		return s.Name
	}
	return s.Name + " " + s.Rest
}

// ParseSpec splits the text of a type. It returns [ErrUnknownType] when the
// brackets do not close or hold anything but whole numbers.
func ParseSpec(text string) (Spec, error) {
	var s Spec
	text = strings.TrimSpace(text)
	for strings.HasSuffix(text, "[]") {
		text = strings.TrimSpace(strings.TrimSuffix(text, "[]"))
		s.Arrays++
	}
	i := strings.IndexByte(text, '(')
	if i < 0 {
		s.Name = words(text)
		return s, nil
	}
	j := strings.IndexByte(text[i:], ')')
	if j < 0 {
		return Spec{}, fmt.Errorf("the brackets of %q do not close: %w", text, ErrUnknownType)
	}
	j += i
	s.Name, s.Rest = words(text[:i]), words(text[j+1:])
	for part := range strings.SplitSeq(text[i+1:j], ",") {
		n, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil {
			return Spec{}, fmt.Errorf("the brackets of %q hold %q, which is not a number: %w", text, part, ErrUnknownType)
		}
		s.Args = append(s.Args, n)
	}
	return s, nil
}

// words lowers the case of s and reduces the white space between words to one
// space.
func words(s string) string {
	return strings.Join(strings.Fields(strings.ToLower(s)), " ")
}
