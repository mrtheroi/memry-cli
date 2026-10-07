package agentfiles

import (
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/mrtheroi/memry-cli/internal/fsx"
	"github.com/mrtheroi/memry-cli/internal/phpjson"
)

// TOMLConfig is a TOML config file of an agent that lists its MCP servers
// as [mcp_servers.<name>] tables, like Codex's config.toml. It is edited
// as text, line by line like the PHP CLI, so every comment and format
// outside memry's table is kept byte for byte.
type TOMLConfig struct {
	path string
}

// NewTOMLConfig returns the config file at path.
func NewTOMLConfig(path string) *TOMLConfig {
	return &TOMLConfig{path: path}
}

// Path is the file's path.
func (c *TOMLConfig) Path() string {
	return c.path
}

// Put sets the server name.
func (c *TOMLConfig) Put(name string, server *phpjson.Object) (bool, error) {
	contents, err := readOrEmpty(c.path)
	if err != nil {
		return false, err
	}
	lines, spans, ok := scanTOML(contents, name)
	if !ok {
		return false, nil
	}
	// Every key and value of the entry is a TOML string written with JSON's
	// escapes: refuse before writing when one cannot be encoded.
	if _, ok := phpjson.Encode(phpjson.NewObject(name, server), phpjson.UnescapedSlashes|phpjson.UnescapedUnicode); !ok {
		return true, errUnencodable
	}
	snippet := c.Snippet(name, server)
	if len(spans) == 0 {
		return true, fsx.Replace(c.path, []byte(blankLineAfter(contents)+snippet))
	}
	return true, fsx.Replace(c.path, []byte(withoutSpans(lines, spans, snippet)))
}

// span is the first and last line of a table.
type span struct{ first, last int }

// withoutSpans joins lines without the given spans, putting replacement in
// place of the first one.
func withoutSpans(lines []string, spans []span, replacement string) string {
	lines = slices.Clone(lines)
	for i := len(spans) - 1; i >= 0; i-- {
		var with []string
		if i == 0 {
			with = []string{replacement}
		}
		lines = slices.Replace(lines, spans[i].first, spans[i].last+1, with...)
	}
	return strings.Join(lines, "")
}

// The TOML syntax the scan follows, as the PHP CLI's regular expressions
// (PCRE's \s, unlike RE2's, includes the vertical tab).
const (
	tomlSpace = `[\t\n\x0B\f\r ]`
	// A bare, "basic" or 'literal' key segment.
	tomlSegment = `[A-Za-z0-9_-]+|"(?:[^"\\\n]|\\.)*"|'[^'\n]*'`
	// A dotted key.
	tomlDottedKey = `(?:` + tomlSegment + `)(?:` + tomlSpace + `*\.` + tomlSpace + `*(?:` + tomlSegment + `))*`
)

var (
	tomlHeader   = regexp.MustCompile(`^\[\[?` + tomlSpace + `*(` + tomlDottedKey + `)` + tomlSpace + `*\]\]?` + tomlSpace + `*(#.*)?$`)
	tomlKeyValue = regexp.MustCompile(`^(` + tomlDottedKey + `)` + tomlSpace + `*=`)
	tomlSegments = regexp.MustCompile(tomlSegment)
	tomlBasic    = regexp.MustCompile(`^"(?:[^"\\\n]|\\.)*"`)
	tomlLiteral  = regexp.MustCompile(`^'[^'\n]*'`)
)

// scanTOML splits contents into lines, each with its newline, and finds
// the line spans of the [mcp_servers.<name>] table and its subtables,
// without the blank and comment lines ending them. It returns false when
// the file cannot be edited safely: it is not valid TOML as far as this
// scan can tell, or it defines the server (or the mcp_servers table) with
// dotted keys or an inline table instead.
func scanTOML(contents, name string) ([]string, []span, bool) {
	lines := strings.SplitAfter(contents, "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	var spans []span
	open := -1
	var table []string
	var multiline string
	depth := 0
	ours := func(key []string) bool {
		return len(key) >= 2 && key[0] == "mcp_servers" && key[1] == name
	}

	for index, line := range lines {
		// Inside a multi-line string or array.
		if multiline != "" || depth > 0 {
			if !scanTOMLValue(line, &multiline, &depth) {
				return nil, nil, false
			}
			continue
		}

		trimmed := phpTrim(line)
		if trimmed == "" || trimmed[0] == '#' {
			continue
		}

		if trimmed[0] == '[' {
			match := tomlHeader.FindStringSubmatch(trimmed)
			if match == nil {
				return nil, nil, false
			}
			segments, ok := tomlKeySegments(match[1])
			if !ok {
				return nil, nil, false
			}
			table = segments
			switch {
			case open >= 0 && !ours(table):
				spans = append(spans, span{open, lastContentLine(lines, open, index-1)})
				open = -1
			case open < 0 && ours(table):
				open = index
			}
			continue
		}

		match := tomlKeyValue.FindStringSubmatch(trimmed)
		if match == nil {
			return nil, nil, false
		}
		segments, ok := tomlKeySegments(match[1])
		if !ok {
			return nil, nil, false
		}
		key := append(slices.Clone(table), segments...)
		if (open < 0 && ours(key)) || (len(table) == 0 && key[0] == "mcp_servers") {
			return nil, nil, false
		}
		if !scanTOMLValue(trimmed[len(match[0]):], &multiline, &depth) {
			return nil, nil, false
		}
	}

	if multiline != "" || depth != 0 {
		return nil, nil, false
	}
	if open >= 0 {
		spans = append(spans, span{open, lastContentLine(lines, open, len(lines)-1)})
	}
	return lines, spans, true
}

// scanTOMLValue follows the strings, arrays and inline tables of a value,
// or of a line of one that spans several lines, updating the multi-line
// string still open (its delimiter) and how many brackets are. It returns
// false when the text is not valid.
func scanTOMLValue(text string, multiline *string, depth *int) bool {
	for i := 0; i < len(text); i++ {
		switch {
		case *multiline == `"""` && text[i] == '\\':
			i++
		case *multiline != "":
			if strings.HasPrefix(text[i:], *multiline) {
				*multiline = ""
				i += 2
			}
		case text[i] == '#':
			return true
		case strings.HasPrefix(text[i:], `"""`) || strings.HasPrefix(text[i:], "'''"):
			*multiline = text[i : i+3]
			i += 2
		case text[i] == '"' || text[i] == '\'':
			pattern := tomlBasic
			if text[i] == '\'' {
				pattern = tomlLiteral
			}
			match := pattern.FindString(text[i:])
			if match == "" {
				return false
			}
			i += len(match) - 1
		case text[i] == '[' || text[i] == '{':
			*depth++
		case text[i] == ']' || text[i] == '}':
			if *depth--; *depth < 0 {
				return false
			}
		}
	}
	return true
}

// tomlKeySegments returns the segments of a dotted key, unquoted, or
// false when a "basic" segment cannot be decoded with certainty (see
// sharedEscapes).
func tomlKeySegments(key string) ([]string, bool) {
	segments := tomlSegments.FindAllString(key, -1)
	for i, segment := range segments {
		switch segment[0] {
		case '"':
			decoded, ok := phpjson.Decode([]byte(segment))
			if !ok || !sharedEscapes(segment) {
				return nil, false
			}
			segments[i] = decoded.(string)
		case '\'':
			segments[i] = segment[1 : len(segment)-1]
		}
	}
	return segments, true
}

// sharedEscapes reports whether every escape in the quoted segment is one
// JSON and TOML read the same way: \b \t \n \f \r \" \\ and \uXXXX
// outside the surrogates. JSON's \/ and surrogate pairs are not TOML, and
// TOML's \UXXXXXXXX, \e and \xHH are not JSON. The PHP CLI decodes the
// segment with json_decode and, when that fails, keeps it quoted, which
// misses memry's table and appends a duplicate.
func sharedEscapes(segment string) bool {
	for i := 0; i < len(segment); i++ {
		if segment[i] != '\\' {
			continue
		}
		i++
		switch segment[i] {
		case 'b', 't', 'n', 'f', 'r', '"', '\\':
		case 'u':
			unit, err := strconv.ParseUint(segment[i+1:i+5], 16, 16)
			if err != nil || (unit >= 0xD800 && unit <= 0xDFFF) {
				return false
			}
			i += 4
		default:
			return false
		}
	}
	return true
}

// lastContentLine is the last line from first to last that is not blank
// or a comment.
func lastContentLine(lines []string, first, last int) int {
	for last > first {
		trimmed := phpTrim(lines[last])
		if trimmed != "" && trimmed[0] != '#' {
			break
		}
		last--
	}
	return last
}

// Remove removes the server name and its subtables.
func (c *TOMLConfig) Remove(name string) (Removal, error) {
	contents, err := readOrEmpty(c.path)
	if err != nil {
		return NotFound, err
	}
	lines, spans, ok := scanTOML(contents, name)
	if !ok {
		return Unsafe, nil
	}
	if len(spans) == 0 {
		return NotFound, nil
	}
	// The lines before the first span are the same with or without the server.
	before := strings.Join(lines[:spans[0].first], "")
	after := withoutSpans(lines, spans, "")[len(before):]
	return Removed, fsx.Replace(c.path, []byte(blankLineJoin(before, after)))
}

// Snippet is the [mcp_servers.<name>] table for server, one key per line.
func (c *TOMLConfig) Snippet(name string, server *phpjson.Object) string {
	lines := []string{"[mcp_servers." + tomlKey(name) + "]"}
	for key, value := range server.All() {
		lines = append(lines, tomlKey(key)+" = "+tomlValue(value))
	}
	return strings.Join(lines, "\n") + "\n"
}

var bareKey = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

// tomlKey is key bare when it can be, else quoted.
func tomlKey(key string) string {
	if bareKey.MatchString(key) {
		return key
	}
	return tomlString(key)
}

// tomlValue is a string, a list of strings or an object of strings as a
// TOML string, array or inline table.
func tomlValue(value any) string {
	switch v := value.(type) {
	case []string:
		items := make([]string, len(v))
		for i, item := range v {
			items[i] = tomlString(item)
		}
		return "[" + strings.Join(items, ", ") + "]"
	case *phpjson.Object:
		var pairs []string
		for key, item := range v.All() {
			pairs = append(pairs, tomlKey(key)+" = "+tomlValue(item))
		}
		return "{ " + strings.Join(pairs, ", ") + " }"
	}
	return tomlString(value.(string))
}

// tomlString is a TOML basic string: JSON's escapes are all valid TOML
// escapes.
func tomlString(value string) string {
	encoded, _ := phpjson.Encode(value, phpjson.UnescapedSlashes|phpjson.UnescapedUnicode)
	return string(encoded)
}
