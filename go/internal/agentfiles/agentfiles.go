// Package agentfiles edits the files of the agents memry is wired into:
// their instructions, MCP server configs and Claude Code's settings. Like
// the PHP CLI's App\Support classes, it changes only memry's own entries,
// keeps everything else byte for byte where it can, and leaves a file it
// cannot edit safely untouched.
package agentfiles

import (
	"errors"
	"io/fs"
	"os"
	"strings"

	"github.com/mrtheroi/memry-cli/internal/phpjson"
)

// Removal is what removing memry's entry from a file did.
type Removal int

const (
	// NotFound means there was no entry to remove; the file is untouched.
	NotFound Removal = iota
	// Removed means the entry was removed.
	Removed
	// Unsafe means the file cannot be edited safely and was left untouched.
	Unsafe
)

// readOrEmpty returns the contents of the file at path, or "" when there
// is none, like PHP's is_file($path) ? file_get_contents($path) : ”.
func readOrEmpty(path string) (string, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return "", nil
	}
	return string(data), err
}

// phpTrim trims like PHP's trim(): spaces, tabs, newlines, carriage
// returns, NUL bytes and vertical tabs.
func phpTrim(s string) string {
	return strings.Trim(s, " \t\n\r\x00\x0B")
}

// encodable reports whether json_encode can write value back: it cannot a
// number too large for a float.
func encodable(value *phpjson.Object) bool {
	_, ok := phpjson.Encode(value, 0)
	return ok
}

// encode writes value like PHP's json_encode with pretty printing and
// slashes and non-ASCII characters unescaped. value must be encodable.
func encode(value *phpjson.Object) []byte {
	encoded, _ := phpjson.Encode(value, phpjson.PrettyPrint|phpjson.UnescapedSlashes|phpjson.UnescapedUnicode)
	return encoded
}
