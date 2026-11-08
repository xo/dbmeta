package dump

import (
	"database/sql/driver"
	"encoding/hex"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Values. A driver gives a column as nil, int64, float64, bool, []byte, string or
// time.Time, and dump reads a value by the kind of its column, because the value
// alone is not enough: a []byte is a binary value or the text of a number. A
// value of a type that has a String that writes ISO 8601, which every civil type
// of dbimp has, is read as that text. See D275.

// timeLayouts are the forms that databases write a date and a time in, and the
// ISO 8601 forms of dbimp, in the order that they are tried.
var timeLayouts = []string{
	time.RFC3339Nano,
	"2006-01-02T15:04:05.999999999",
	"2006-01-02 15:04:05.999999999Z07:00",
	"2006-01-02 15:04:05.999999999Z07",
	"2006-01-02 15:04:05.999999999-07:00:00",
	"2006-01-02 15:04:05.999999999",
	"2006-01-02",
	"15:04:05.999999999Z07:00",
	"15:04:05.999999999Z07",
	"15:04:05.999999999",
}

// parseTime reads a date or a time, or both. A year before 0001 or after 9999 is
// outside the forms of the standard library, so it is an error, and the plan
// says which columns can hold one (D275).
func parseTime(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	for _, layout := range timeLayouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("%q is not a date or a time that dump reads: %w", s, ErrBadValue)
}

// ErrBadValue is the bad value error. A value cannot be written as the type of
// its column.
const ErrBadValue Error = "value does not fit its type"

// valueText returns the text of a value, for a kind that reads text.
func valueText(v any) (string, bool) {
	switch x := v.(type) {
	case string:
		return x, true
	case []byte:
		return string(x), true
	case time.Time:
		return x.Format(time.RFC3339Nano), true
	case driver.Valuer:
		if out, err := x.Value(); err == nil {
			return valueText(out)
		}
	case fmt.Stringer:
		return x.String(), true
	}
	return "", false
}

var (
	integerText = regexp.MustCompile(`^[+-]?\d+$`)
	numberText  = regexp.MustCompile(`^[+-]?(\d+(\.\d*)?|\.\d+)([eE][+-]?\d+)?$`)
	uuidText    = regexp.MustCompile(`^(?i:[0-9a-f]{8}-?[0-9a-f]{4}-?[0-9a-f]{4}-?[0-9a-f]{4}-?[0-9a-f]{12})$`)
)

// Convert reads a value that a driver gave for a column of the type from, and
// returns the value in the form that a driver of the target takes for the kind of
// to. A NULL stays nil. It returns [ErrBadValue] for a value that the kind cannot
// hold, such as 2 for a boolean, and names the value, so that a column that was
// read as a kind by a convention stops the dump and does not guess.
//
// The values are: bool for a boolean, int64 for an integer, float64 for a float,
// string for a decimal, a string or a binary value, time.Time for a date or a
// time, and string for the rest.
func Convert(v any, from, to Type) (any, error) {
	if v == nil {
		return nil, nil
	}
	switch to.Kind {
	case Boolean:
		return toBool(v)
	case Integer, BigInteger:
		return toInteger(v)
	case Float:
		return toFloat(v)
	case Decimal:
		s, ok := valueText(v)
		if !ok {
			if f, isFloat := v.(float64); isFloat {
				return strconv.FormatFloat(f, 'f', -1, 64), nil
			}
			if f, isFloat := v.(float32); isFloat {
				return strconv.FormatFloat(float64(f), 'f', -1, 32), nil
			}
			if i, isInt := asInt64(v); isInt {
				return strconv.FormatInt(i, 10), nil
			}
			return nil, fmt.Errorf("%v is not a decimal: %w", v, ErrBadValue)
		}
		if !numberText.MatchString(strings.TrimSpace(s)) {
			return nil, fmt.Errorf("%q is not a decimal: %w", s, ErrBadValue)
		}
		return strings.TrimSpace(s), nil
	case Binary:
		switch x := v.(type) {
		case []byte:
			return x, nil
		case string:
			return []byte(x), nil
		}
		return nil, fmt.Errorf("%v is not bytes: %w", v, ErrBadValue)
	case Date, TimeOfDay, TimeOfDayOffset, LocalTimestamp, Timestamp:
		return toTime(v, to.Kind)
	case UUID:
		return toUUID(v)
	case String, JSON, IPAddress:
		if s, ok := valueText(v); ok {
			return s, nil
		}
		return nil, fmt.Errorf("%v is not text: %w", v, ErrBadValue)
	}
	_ = from
	if s, ok := valueText(v); ok {
		return s, nil
	}
	return v, nil
}

// toBool reads a boolean, which a product writes as true and false, as 0 and 1,
// as the bit that MySQL gives for bit(1), or as t and f. Any other value is a
// fault, so a column that held 2 stops the dump.
func toBool(v any) (any, error) {
	if i, ok := asInt64(v); ok {
		if i == 0 || i == 1 {
			return i == 1, nil
		}
		return nil, fmt.Errorf("%v is not a boolean: %w", v, ErrBadValue)
	}
	switch x := v.(type) {
	case bool:
		return x, nil
	case []byte:
		if len(x) == 1 && x[0] <= 1 {
			return x[0] == 1, nil
		}
		return toBool(string(x))
	case string:
		switch strings.ToLower(strings.TrimSpace(x)) {
		case "1", "t", "true", "y", "yes", "on":
			return true, nil
		case "0", "f", "false", "n", "no", "off":
			return false, nil
		}
	}
	return nil, fmt.Errorf("%v is not a boolean: %w", v, ErrBadValue)
}

// asInt64 reads any of the integer types that a driver gives. A uint64 above the
// range of int64 is not one.
func asInt64(v any) (int64, bool) {
	switch x := v.(type) {
	case int:
		return int64(x), true
	case int8:
		return int64(x), true
	case int16:
		return int64(x), true
	case int32:
		return int64(x), true
	case int64:
		return x, true
	case uint:
		return int64(x), x <= math.MaxInt64
	case uint8:
		return int64(x), true
	case uint16:
		return int64(x), true
	case uint32:
		return int64(x), true
	case uint64:
		return int64(x), x <= math.MaxInt64
	}
	return 0, false
}

// toInteger reads an integer.
func toInteger(v any) (any, error) {
	if i, ok := asInt64(v); ok {
		return i, nil
	}
	switch x := v.(type) {
	case bool:
		if x {
			return int64(1), nil
		}
		return int64(0), nil
	case float64:
		if x == math.Trunc(x) && math.Abs(x) < 1<<63 {
			return int64(x), nil
		}
	case uint64:
		return strconv.FormatUint(x, 10), nil
	case uint:
		return strconv.FormatUint(uint64(x), 10), nil
	}
	if s, ok := valueText(v); ok && integerText.MatchString(strings.TrimSpace(s)) {
		s = strings.TrimSpace(s)
		if i, err := strconv.ParseInt(s, 10, 64); err == nil {
			return i, nil
		}
		return s, nil
	}
	return nil, fmt.Errorf("%v is not an integer: %w", v, ErrBadValue)
}

// toFloat reads a floating point number.
func toFloat(v any) (any, error) {
	switch x := v.(type) {
	case float64:
		return x, nil
	case float32:
		return float64(x), nil
	}
	if i, ok := asInt64(v); ok {
		return float64(i), nil
	}
	if s, ok := valueText(v); ok {
		if f, err := strconv.ParseFloat(strings.TrimSpace(s), 64); err == nil {
			return f, nil
		}
	}
	return nil, fmt.Errorf("%v is not a float: %w", v, ErrBadValue)
}

// toTime reads a date or a time as a time.Time. A date has no time, a local
// timestamp has no zone and is the wall clock in UTC, and an instant is in UTC.
func toTime(v any, kind Kind) (any, error) {
	var t time.Time
	switch x := v.(type) {
	case time.Time:
		t = x
	default:
		s, ok := valueText(v)
		if !ok {
			return nil, fmt.Errorf("%v is not a date or a time: %w", v, ErrBadValue)
		}
		var err error
		if t, err = parseTime(s); err != nil {
			return nil, err
		}
	}
	switch kind {
	case Date:
		y, m, d := t.Date()
		return time.Date(y, m, d, 0, 0, 0, 0, time.UTC), nil
	case LocalTimestamp:
		y, m, d := t.Date()
		h, mi, s := t.Clock()
		return time.Date(y, m, d, h, mi, s, t.Nanosecond(), time.UTC), nil
	case TimeOfDay:
		h, mi, s := t.Clock()
		return time.Date(0, 1, 1, h, mi, s, t.Nanosecond(), time.UTC), nil
	}
	return t, nil
}

// toUUID reads a UUID from its text, or from 16 bytes in the order of the text.
func toUUID(v any) (any, error) {
	if b, ok := v.([]byte); ok && len(b) == 16 {
		h := hex.EncodeToString(b)
		return h[:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:], nil
	}
	s, ok := valueText(v)
	if !ok || !uuidText.MatchString(strings.TrimSpace(s)) {
		return nil, fmt.Errorf("%v is not a UUID: %w", v, ErrBadValue)
	}
	s = strings.ToLower(strings.ReplaceAll(strings.TrimSpace(s), "-", ""))
	return s[:8] + "-" + s[8:12] + "-" + s[12:16] + "-" + s[16:20] + "-" + s[20:], nil
}
