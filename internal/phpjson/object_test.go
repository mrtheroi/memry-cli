package phpjson_test

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/mrtheroi/memry-cli/internal/phpjson"
)

// What PHP's json_encode(json_decode($input), $flags) writes, from
// testdata/php_json_pretty.php.
func TestDecodeAndEncodeMatchPHP(t *testing.T) {
	data, err := os.ReadFile("testdata/php_json_pretty.txt")
	if err != nil {
		t.Fatal(err)
	}
	flags := []phpjson.Flags{
		phpjson.PrettyPrint | phpjson.UnescapedSlashes | phpjson.UnescapedUnicode,
		phpjson.PrettyPrint | phpjson.UnescapedSlashes,
	}
	for line := range strings.Lines(string(data)) {
		fields := strings.Split(strings.TrimSuffix(line, "\n"), "\t")
		var input string
		if err := json.Unmarshal([]byte(fields[0]), &input); err != nil {
			t.Fatalf("bad golden line %q: %v", line, err)
		}
		for i, want := range fields[1:] {
			got := "false"
			if value, ok := phpjson.Decode([]byte(input)); ok {
				if encoded, ok := phpjson.Encode(value, flags[i]); ok {
					quoted, _ := phpjson.Encode(string(encoded), phpjson.UnescapedSlashes|phpjson.UnescapedUnicode)
					got = string(quoted)
				}
			}
			if got != want {
				t.Errorf("Encode(Decode(%q), flags %d) = %s, want %s", input, flags[i], got, want)
			}
		}
	}
}
