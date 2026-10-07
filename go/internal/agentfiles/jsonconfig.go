package agentfiles

import (
	"errors"
	"io/fs"
	"os"

	"github.com/mrtheroi/memry-cli/internal/fsx"
	"github.com/mrtheroi/memry-cli/internal/phpjson"
)

// JSONConfig is a JSON config file of an agent that lists its MCP servers
// by name under one top-level key, like "mcpServers" or "mcp".
type JSONConfig struct {
	path, key string
}

// NewJSONConfig returns the config file at path, with its servers under key.
func NewJSONConfig(path, key string) *JSONConfig {
	return &JSONConfig{path: path, key: key}
}

// Path is the file's path.
func (c *JSONConfig) Path() string {
	return c.path
}

// Put sets the server name.
func (c *JSONConfig) Put(name string, server *phpjson.Object) (bool, error) {
	config, ok, err := c.read()
	if !ok || err != nil {
		return false, err
	}
	// Like PHP's ??=, a null server list is replaced too.
	servers, _ := config.Get(c.key)
	if servers == nil {
		servers = phpjson.NewObject()
		config.Set(c.key, servers)
	}
	servers.(*phpjson.Object).Set(name, server)
	return true, c.write(config)
}

// read decodes the file, or returns an empty object when there is none.
// It returns false when the file is not well formed: a JSON object (with
// no comments) whose servers, unless missing or null, are an object, and
// that can be written back (PHP would write a lone newline instead).
func (c *JSONConfig) read() (*phpjson.Object, bool, error) {
	contents, err := os.ReadFile(c.path)
	if errors.Is(err, fs.ErrNotExist) {
		return phpjson.NewObject(), true, nil
	}
	if err != nil {
		return nil, false, err
	}
	decoded, _ := phpjson.Decode(contents)
	config, ok := decoded.(*phpjson.Object)
	if !ok {
		return nil, false, nil
	}
	if !encodable(config) {
		return nil, false, nil
	}
	switch servers, _ := config.Get(c.key); servers.(type) {
	case nil, *phpjson.Object:
		return config, true, nil
	}
	return nil, false, nil
}

// write writes config pretty-printed with slashes and non-ASCII
// characters unescaped, like PHP's json_encode with those flags.
func (c *JSONConfig) write(config *phpjson.Object) error {
	return fsx.Replace(c.path, append(encode(config), '\n'))
}

// Remove removes the server name, and the server list when that leaves it
// empty, as Put creates it when missing.
func (c *JSONConfig) Remove(name string) (Removal, error) {
	config, ok, err := c.read()
	if err != nil {
		return NotFound, err
	}
	if !ok {
		return Unsafe, nil
	}
	servers, _ := config.Get(c.key)
	list, _ := servers.(*phpjson.Object)
	if list == nil {
		return NotFound, nil
	}
	// Like PHP's isset, a null server is no server.
	if server, _ := list.Get(name); server == nil {
		return NotFound, nil
	}
	list.Delete(name)
	if list.Len() == 0 {
		config.Delete(c.key)
	}
	return Removed, c.write(config)
}

// Snippet is the server name under the server list, as a JSON file of
// its own.
func (c *JSONConfig) Snippet(name string, server *phpjson.Object) string {
	return string(encode(phpjson.NewObject(c.key, phpjson.NewObject(name, server))))
}
