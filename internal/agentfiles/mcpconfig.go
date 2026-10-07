package agentfiles

import "github.com/mrtheroi/memry-cli/internal/phpjson"

// MCPConfig is an agent's config file that lists its MCP servers by name.
// A server is an object of strings, lists of strings and objects of
// strings, in the agent's format.
type MCPConfig interface {
	// Path is the file's path.
	Path() string
	// Put sets the server name, replacing any previous one of that name.
	// It returns false, leaving the file untouched, when the file cannot
	// be edited safely.
	Put(name string, server *phpjson.Object) (bool, error)
	// Remove removes the server name.
	Remove(name string) (Removal, error)
	// Snippet is the server name as it would appear in the file, for
	// manual setup.
	Snippet(name string, server *phpjson.Object) string
}
