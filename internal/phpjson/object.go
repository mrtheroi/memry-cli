package phpjson

import (
	"bytes"
	"encoding/json"
	"fmt"
	"iter"
	"strconv"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

// Object is a JSON object as PHP's json_decode gives it without
// $associative, a stdClass: its keys in the order they first appear, each
// with its last value. It is always encoded as an object, even when empty.
type Object struct {
	members []member
}

// NewObject returns an object with the given keys and values, in order:
// key, value, key, value…
func NewObject(pairs ...any) *Object {
	o := &Object{}
	for i := 0; i+1 < len(pairs); i += 2 {
		o.Set(pairs[i].(string), pairs[i+1])
	}
	return o
}

// Get returns the value of key, and whether the object has it.
func (o *Object) Get(key string) (any, bool) {
	for _, m := range o.members {
		if m.key == key {
			return m.value, true
		}
	}
	return nil, false
}

// Set replaces the value of key in place, or adds it after the others.
func (o *Object) Set(key string, value any) {
	for i := range o.members {
		if o.members[i].key == key {
			o.members[i].value = value
			return
		}
	}
	o.members = append(o.members, member{key, value})
}

// Delete removes key, like PHP's unset.
func (o *Object) Delete(key string) {
	for i, m := range o.members {
		if m.key == key {
			o.members = append(o.members[:i:i], o.members[i+1:]...)
			return
		}
	}
}

// All yields the keys and values, in order.
func (o *Object) All() iter.Seq2[string, any] {
	return func(yield func(string, any) bool) {
		for _, m := range o.members {
			if !yield(m.key, m.value) {
				return
			}
		}
	}
}

// Len is the number of keys.
func (o *Object) Len() int {
	return len(o.members)
}

// Flags are the json_encode flags memry's agent files are written with.
type Flags int

// The json_encode flags.
const (
	PrettyPrint Flags = 1 << iota
	UnescapedSlashes
	UnescapedUnicode
)

// Decode returns what PHP's json_decode(raw) returns, objects as *Object,
// arrays as []any, numbers as json.Number, or false when it returns null
// for anything but the JSON null.
func Decode(raw []byte) (any, bool) {
	if !Valid(raw) {
		return nil, false
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	value, err := decode(decoder)
	if err != nil {
		return nil, false
	}
	return toObjects(value)
}

// toObjects turns the decoded objects into *Object, failing like PHP on a
// key starting with a NUL byte, which is not a valid property name.
func toObjects(value any) (any, bool) {
	switch v := value.(type) {
	case []any:
		for i := range v {
			var ok bool
			if v[i], ok = toObjects(v[i]); !ok {
				return nil, false
			}
		}
	case object:
		o := &Object{members: v}
		for i, m := range o.members {
			if strings.HasPrefix(m.key, "\x00") {
				return nil, false
			}
			var ok bool
			if o.members[i].value, ok = toObjects(m.value); !ok {
				return nil, false
			}
		}
		return o, true
	}
	return value, true
}

// Encode returns what PHP's json_encode(value, flags) writes, or false
// when it fails. value holds nil, bools, ints, strings (valid UTF-8),
// json.Numbers, []any, []string and *Object.
func Encode(value any, flags Flags) ([]byte, bool) {
	return encodeValue(nil, value, flags, 0)
}

func encodeValue(out []byte, value any, flags Flags, depth int) ([]byte, bool) {
	switch v := value.(type) {
	case nil:
		return append(out, "null"...), true
	case bool:
		return strconv.AppendBool(out, v), true
	case int:
		return strconv.AppendInt(out, int64(v), 10), true
	case string:
		return encodeString(out, v, flags)
	case json.Number:
		return encodeNumber(out, string(v))
	case []string:
		values := make([]any, len(v))
		for i, s := range v {
			values[i] = s
		}
		return encodeValue(out, values, flags, depth)
	case []any:
		if len(v) == 0 {
			return append(out, "[]"...), true
		}
		out = append(out, '[')
		for i, item := range v {
			out = separator(out, i, flags, depth+1)
			var ok bool
			if out, ok = encodeValue(out, item, flags, depth+1); !ok {
				return nil, false
			}
		}
		return append(newline(out, flags, depth), ']'), true
	case *Object:
		if v.Len() == 0 {
			return append(out, "{}"...), true
		}
		out = append(out, '{')
		for i, m := range v.members {
			out = separator(out, i, flags, depth+1)
			var ok bool
			if out, ok = encodeString(out, m.key, flags); !ok {
				return nil, false
			}
			out = append(out, ':')
			if flags&PrettyPrint != 0 {
				out = append(out, ' ')
			}
			if out, ok = encodeValue(out, m.value, flags, depth+1); !ok {
				return nil, false
			}
		}
		return append(newline(out, flags, depth), '}'), true
	}
	return nil, false
}

// separator starts the element i of an array or object at depth.
func separator(out []byte, i int, flags Flags, depth int) []byte {
	if i > 0 {
		out = append(out, ',')
	}
	return newline(out, flags, depth)
}

// newline starts a new line indented to depth, when pretty printing.
func newline(out []byte, flags Flags, depth int) []byte {
	if flags&PrettyPrint == 0 {
		return out
	}
	return append(append(out, '\n'), strings.Repeat("    ", depth)...)
}

// encodeString writes s as json_encode does with flags, failing on
// invalid UTF-8. Even unescaped, U+2028 and U+2029 stay escaped
// (there is no JSON_UNESCAPED_LINE_TERMINATORS here).
func encodeString(out []byte, s string, flags Flags) ([]byte, bool) {
	if !utf8.ValidString(s) {
		return nil, false
	}
	out = append(out, '"')
	for _, r := range s {
		switch {
		case r == '"' || r == '\\':
			out = append(out, '\\', byte(r))
		case r == '/' && flags&UnescapedSlashes == 0:
			out = append(out, `\/`...)
		case r < 0x20:
			out = append(out, controlEscape(r)...)
		case r < 0x80:
			out = append(out, byte(r))
		case flags&UnescapedUnicode != 0 && r != 0x2028 && r != 0x2029:
			out = utf8.AppendRune(out, r)
		default:
			for _, unit := range utf16.Encode([]rune{r}) {
				out = fmt.Appendf(out, `\u%04x`, unit)
			}
		}
	}
	return append(out, '"'), true
}
