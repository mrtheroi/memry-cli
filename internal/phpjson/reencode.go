package phpjson

import (
	"bytes"
	"encoding/json"
	"math"
	"strconv"
	"strings"
)

// Reencode returns what PHP's json_encode(json_decode(raw, true),
// JSON_UNESCAPED_SLASHES) writes for the valid JSON raw, or false when
// json_encode fails (on a number too large for a float).
func Reencode(raw []byte) ([]byte, bool) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	value, err := decode(decoder)
	if err != nil {
		return nil, false
	}
	return encode(nil, value)
}

// member is one key and value of a decoded object.
type member struct {
	key   string
	value any
}

// object is a decoded object, like a PHP array: its keys in the order
// they first appear, each with its last value.
type object []member

// decode reads the next value: nil, a bool, a string, a json.Number, a
// []any or an object.
func decode(decoder *json.Decoder) (any, error) {
	token, err := decoder.Token()
	if err != nil {
		return nil, err
	}
	delim, ok := token.(json.Delim)
	if !ok {
		return token, nil
	}
	if delim == '[' {
		list := []any{}
		for decoder.More() {
			value, err := decode(decoder)
			if err != nil {
				return nil, err
			}
			list = append(list, value)
		}
		_, err := decoder.Token()
		return list, err
	}
	obj := object{}
	for decoder.More() {
		key, err := decoder.Token()
		if err != nil {
			return nil, err
		}
		value, err := decode(decoder)
		if err != nil {
			return nil, err
		}
		obj = obj.set(key.(string), value)
	}
	_, err = decoder.Token()
	return obj, err
}

func (o object) set(key string, value any) object {
	for i := range o {
		if o[i].key == key {
			o[i].value = value
			return o
		}
	}
	return append(o, member{key, value})
}

// isList reports whether PHP sees the object as a list: its keys are 0,
// 1, 2… in order (PHP turns the keys "0", "1"… into integers).
func (o object) isList() bool {
	for i, m := range o {
		if m.key != strconv.Itoa(i) {
			return false
		}
	}
	return true
}

func encode(out []byte, value any) ([]byte, bool) {
	switch v := value.(type) {
	case nil:
		return append(out, "null"...), true
	case bool:
		return strconv.AppendBool(out, v), true
	case string:
		return append(out, EncodeString(v, true)...), true
	case json.Number:
		return encodeNumber(out, string(v))
	case []any:
		return encodeList(out, v)
	case object:
		if v.isList() {
			values := make([]any, len(v))
			for i, m := range v {
				values[i] = m.value
			}
			return encodeList(out, values)
		}
		out = append(out, '{')
		for i, m := range v {
			if i > 0 {
				out = append(out, ',')
			}
			out = append(out, EncodeString(m.key, true)...)
			out = append(out, ':')
			var ok bool
			if out, ok = encode(out, m.value); !ok {
				return nil, false
			}
		}
		return append(out, '}'), true
	}
	return nil, false
}

func encodeList(out []byte, values []any) ([]byte, bool) {
	out = append(out, '[')
	for i, value := range values {
		if i > 0 {
			out = append(out, ',')
		}
		var ok bool
		if out, ok = encode(out, value); !ok {
			return nil, false
		}
	}
	return append(out, ']'), true
}

// encodeNumber writes a JSON number the way PHP decodes and re-encodes
// it: an integer that fits in 64 bits stays one, anything else becomes a
// float.
func encodeNumber(out []byte, number string) ([]byte, bool) {
	if !strings.ContainsAny(number, ".eE") {
		if n, err := strconv.ParseInt(number, 10, 64); err == nil {
			return strconv.AppendInt(out, n, 10), true
		}
	}
	f, _ := strconv.ParseFloat(number, 64)
	if math.IsInf(f, 0) {
		return nil, false
	}
	return append(out, formatFloat(f)...), true
}

// formatFloat formats f like PHP's json_encode with serialize_precision
// -1: the shortest digits that read back as f, in exponential notation
// when the decimal point would sit more than 17 digits right or 4 left of
// the first digit.
func formatFloat(f float64) string {
	sign := ""
	if math.Signbit(f) {
		sign = "-"
	}
	if f == 0 {
		return sign + "0"
	}
	mantissa, exponent, _ := strings.Cut(strconv.FormatFloat(math.Abs(f), 'e', -1, 64), "e")
	digits := strings.Replace(mantissa, ".", "", 1)
	exp, _ := strconv.Atoi(exponent)
	point := exp + 1
	switch {
	case point < -3 || point > 17:
		fraction := digits[1:]
		if fraction == "" {
			fraction = "0"
		}
		expSign := "+"
		if exp < 0 {
			expSign, exp = "-", -exp
		}
		return sign + digits[:1] + "." + fraction + "e" + expSign + strconv.Itoa(exp)
	case point <= 0:
		return sign + "0." + strings.Repeat("0", -point) + digits
	case point >= len(digits):
		return sign + digits + strings.Repeat("0", point-len(digits))
	}
	return sign + digits[:point] + "." + digits[point:]
}
