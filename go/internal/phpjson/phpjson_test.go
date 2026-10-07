package phpjson_test

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/mrtheroi/memry-cli/internal/phpjson"
)

// PHP's json_validate rejects an unpaired UTF-16 surrogate escape, which
// Go's decoder would silently replace.
func TestValidRejectsUnpairedSurrogates(t *testing.T) {
	tests := map[string]bool{
		`"\ud800\udc00"`: true,
		`{"a":1}`:        true,
		`"\ud800"`:       false,
		`"\udc00"`:       false,
		`"\ud800x"`:      false,
		`"\ud800\u0041"`: false,
	}
	for input, want := range tests {
		if got := phpjson.Valid([]byte(input)); got != want {
			t.Errorf("Valid(%s) = %v, want %v", input, got, want)
		}
	}
}

// Like PHP's json_validate, a string that is not valid UTF-8 is not JSON
// (Go's decoder accepts it).
func TestValidRejectsInvalidUTF8(t *testing.T) {
	if phpjson.Valid([]byte("\"\xc3\x28\"")) {
		t.Error("Valid accepted invalid UTF-8")
	}
}

// PHP's json_validate (default depth 512) accepts at most 511 nested
// arrays or objects.
func TestValidLimitsTheNesting(t *testing.T) {
	nested := func(n int, inner string) []byte {
		return []byte(strings.Repeat("[", n) + inner + strings.Repeat("]", n))
	}
	for _, accepted := range [][]byte{nested(511, ""), nested(510, `{"a":1}`), nested(511, `"[{"`)} {
		if !phpjson.Valid(accepted) {
			t.Errorf("Valid rejected %d bytes of nesting", len(accepted))
		}
	}
	for _, rejected := range [][]byte{nested(512, ""), nested(511, `{"a":1}`)} {
		if phpjson.Valid(rejected) {
			t.Errorf("Valid accepted %d bytes of nesting", len(rejected))
		}
	}
}

// The strings PHP 8.4's json_encode wrote for these.
func TestEncodeStringEscapesLikePHP(t *testing.T) {
	tests := []struct {
		in               string
		unescapedSlashes bool
		want             string
	}{
		{"a/b", false, `"a\/b"`},
		{"a/b", true, `"a/b"`},
		{"é😀", true, `"\u00e9\ud83d\ude00"`},
		{"\x01\x1f\x7f", true, "\"\\u0001\\u001f\x7f\""},
		{"<>&'\"\\\n\t\r\b\f", true, `"<>&'\"\\\n\t\r\b\f"`},
		{"\u2028", true, `"\u2028"`},
	}
	for _, tt := range tests {
		if got := string(phpjson.EncodeString(tt.in, tt.unescapedSlashes)); got != tt.want {
			t.Errorf("EncodeString(%q, %v) = %s, want %s", tt.in, tt.unescapedSlashes, got, tt.want)
		}
	}
}

// Reencode writes what PHP 8.4 wrote for each case of
// testdata/php_json_encode.txt.
func TestReencodeLikePHP(t *testing.T) {
	data, err := os.ReadFile("testdata/php_json_encode.txt")
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		var input, want string
		quotedInput, quotedWant, _ := strings.Cut(line, "\t")
		if err := json.Unmarshal([]byte(quotedInput), &input); err != nil {
			t.Fatalf("bad line %q: %v", line, err)
		}
		encodes := quotedWant != "false"
		if encodes {
			if err := json.Unmarshal([]byte(quotedWant), &want); err != nil {
				t.Fatalf("bad line %q: %v", line, err)
			}
		}
		got, ok := phpjson.Reencode([]byte(input))
		if ok != encodes || string(got) != want {
			t.Errorf("Reencode(%s) = %s, %v; want %s, %v", input, got, ok, want, encodes)
		}
	}
}
