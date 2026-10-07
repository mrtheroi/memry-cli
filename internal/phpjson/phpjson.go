// Package phpjson reads and writes JSON the way PHP's json functions do,
// where the memry PHP CLI's behavior depends on them.
package phpjson

import (
	"encoding/json"
	"fmt"
	"strconv"
	"unicode/utf8"
)

// maxNesting is the deepest nesting of arrays and objects PHP's JSON
// parser accepts with its default depth of 512.
const maxNesting = 511

// Valid reports whether data is JSON that PHP's json_validate (and
// json_decode) accepts: valid UTF-8, no unpaired UTF-16 surrogate escape
// and at most 511 nested arrays or objects.
func Valid(data []byte) bool {
	return utf8.Valid(data) && json.Valid(data) && phpLimits(data)
}

// phpLimits reports whether the valid JSON data stays within PHP's
// nesting limit and every UTF-16 surrogate escape in it is a high one
// followed by a low one.
func phpLimits(data []byte) bool {
	inString := false
	depth := 0
	for i := 0; i < len(data); i++ {
		switch c := data[i]; {
		case c == '"':
			inString = !inString
		case !inString && (c == '[' || c == '{'):
			if depth++; depth > maxNesting {
				return false
			}
		case !inString && (c == ']' || c == '}'):
			depth--
		case !inString || c != '\\':
		case data[i+1] != 'u':
			i++
		default:
			r := hex4(data[i+2 : i+6])
			i += 5
			if r >= 0xDC00 && r <= 0xDFFF {
				return false
			}
			if r >= 0xD800 && r <= 0xDBFF {
				if i+6 >= len(data) || data[i+1] != '\\' || data[i+2] != 'u' {
					return false
				}
				if low := hex4(data[i+3 : i+7]); low < 0xDC00 || low > 0xDFFF {
					return false
				}
				i += 6
			}
		}
	}
	return true
}

func hex4(b []byte) uint64 {
	r, _ := strconv.ParseUint(string(b), 16, 16)
	return r
}

// EncodeString returns s as PHP's json_encode writes a string: non-ASCII
// characters as \u escapes, and "/" escaped unless unescapedSlashes
// (JSON_UNESCAPED_SLASHES). s must be valid UTF-8.
func EncodeString(s string, unescapedSlashes bool) []byte {
	var flags Flags
	if unescapedSlashes {
		flags = UnescapedSlashes
	}
	out, _ := encodeString(nil, s, flags)
	return out
}

// controlEscape is how PHP's json_encode escapes a control character.
func controlEscape(r rune) string {
	switch r {
	case '\b':
		return `\b`
	case '\f':
		return `\f`
	case '\n':
		return `\n`
	case '\r':
		return `\r`
	case '\t':
		return `\t`
	}
	return fmt.Sprintf(`\u%04x`, r)
}

// Reencode returns what PHP's json_encode(json_decode(raw, true), flags)
// writes for the valid JSON raw, where flags is JSON_UNESCAPED_SLASHES, or
// false when json_encode fails.
