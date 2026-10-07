// Package config reads and writes the memry config file, in the same JSON
// shape and format as the PHP CLI, so either one reads what the other wrote.
package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/mrtheroi/memry-cli/internal/fsx"
	"github.com/mrtheroi/memry-cli/internal/phpjson"
)

// Path is the config file: $MEMRY_CONFIG, or ~/.config/memry/config.json
// by default.
func Path(getenv func(string) string) string {
	if path := getenv("MEMRY_CONFIG"); path != "" {
		return path
	}
	return getenv("HOME") + "/.config/memry/config.json"
}

// File is the decoded config file. It keeps every key, known or not, in
// the order of the file.
type File struct {
	path   string
	keys   []string
	values map[string]any
}

// Load reads the config file at path. A missing file is an empty config;
// a file that is not a JSON object is an error.
func Load(path string) (*File, error) {
	return load(path, func([]byte) bool { return false })
}

// LoadOrEmpty is Load, except that a file the PHP CLI reads as empty is an
// empty config: one that is not valid JSON (or not valid UTF-8), or is null
// or []. Setup, which replaces the credentials anyway, then overwrites it,
// like PHP's json_decode(...) ?? []. Other JSON that is not an object is
// still an error.
func LoadOrEmpty(path string) (*File, error) {
	return load(path, func(data []byte) bool {
		var list []any
		return !phpjson.Valid(data) ||
			string(bytes.TrimSpace(data)) == "null" ||
			(json.Unmarshal(data, &list) == nil && len(list) == 0)
	})
}

// load reads the config file at path, or an empty config when it is
// missing or isEmpty says so.
func load(path string, isEmpty func([]byte) bool) (*File, error) {
	f := &File{path: path, values: map[string]any{}}

	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return f, nil
	}
	if err != nil {
		return nil, err
	}
	if isEmpty(data) {
		return f, nil
	}
	// Like PHP's json_decode, which rejects invalid UTF-8 and lone
	// surrogates that Go's decoder would silently replace with U+FFFD,
	// altering the saved URL or token.
	if !phpjson.Valid(data) {
		return nil, fmt.Errorf("%s is not valid JSON", path)
	}
	if err := f.decode(data); err != nil {
		return nil, fmt.Errorf("%s is not a valid JSON object: %w", path, err)
	}
	return f, nil
}

// decode reads the top-level object key by key, so the order is kept.
func (f *File) decode(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	if token, err := decoder.Token(); err != nil {
		return err
	} else if token != json.Delim('{') {
		return errors.New("not an object")
	}
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return err
		}
		f.Set(token.(string), value)
	}
	if _, err := decoder.Token(); err != nil {
		return err
	}
	if _, err := decoder.Token(); err == nil {
		return errors.New("data after the object")
	}
	return nil
}

// URL is the saved server URL, when it is a string.
func (f *File) URL() (string, bool) {
	return f.str("url")
}

// Token is the saved API token, when it is a string.
func (f *File) Token() (string, bool) {
	return f.str("token")
}

// Agents is the saved agent selection, when it is a list. Entries that
// are not strings are skipped.
func (f *File) Agents() ([]string, bool) {
	var list []any
	if !f.decodeValue("agents", &list) || list == nil {
		return nil, false
	}
	agents := []string{}
	for _, entry := range list {
		if key, ok := entry.(string); ok {
			agents = append(agents, key)
		}
	}
	return agents, true
}

func (f *File) str(key string) (string, bool) {
	var value any
	if !f.decodeValue(key, &value) {
		return "", false
	}
	s, ok := value.(string)
	return s, ok
}

// decodeValue decodes the value of key into target, reporting whether the
// key exists and decodes.
func (f *File) decodeValue(key string, target any) bool {
	value, ok := f.values[key]
	if !ok {
		return false
	}
	raw, err := json.Marshal(value)
	return err == nil && json.Unmarshal(raw, target) == nil
}

// Set replaces the value of key, or adds it after the others.
func (f *File) Set(key string, value any) {
	if _, ok := f.values[key]; !ok {
		f.keys = append(f.keys, key)
	}
	f.values[key] = value
}

// Save writes the config back to its file atomically, readable by the owner
// only, pretty-printed with four spaces and a trailing newline like the PHP
// CLI.
func (f *File) Save() error {
	var compact bytes.Buffer
	compact.WriteByte('{')
	for i, key := range f.keys {
		if i > 0 {
			compact.WriteByte(',')
		}
		if err := encode(&compact, key); err != nil {
			return err
		}
		compact.WriteByte(':')
		if err := encode(&compact, f.values[key]); err != nil {
			return err
		}
	}
	compact.WriteByte('}')

	var pretty bytes.Buffer
	if err := json.Indent(&pretty, compact.Bytes(), "", "    "); err != nil {
		return err
	}
	pretty.WriteByte('\n')

	// An existing file is made owner-only first, so the write keeps that.
	if err := os.Chmod(f.path, 0o600); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return fsx.WriteAtomic(f.path, pretty.Bytes(), 0o600)
}

// encode writes value as JSON without escaping <, > and &, which PHP's
// json_encode leaves as they are.
func encode(buf *bytes.Buffer, value any) error {
	encoder := json.NewEncoder(buf)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return err
	}
	buf.Truncate(buf.Len() - 1) // Encode ends with a newline.
	return nil
}

// Login returns the saved server URL and token, or false when not logged
// in: the PHP CLI needs both as non-empty strings, and reads a file that
// is missing or not a JSON object as no login.
func Login(getenv func(string) string) (url, token string, ok bool) {
	cfg, err := Load(Path(getenv))
	if err != nil {
		return "", "", false
	}
	url, hasURL := cfg.URL()
	token, hasToken := cfg.Token()
	return url, token, hasURL && url != "" && hasToken && token != ""
}
