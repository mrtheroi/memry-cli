package mcp

import (
	"io"

	"github.com/mrtheroi/memry-cli/internal/config"
	"github.com/mrtheroi/memry-cli/internal/phpjson"
)

// HeadersEnv is what `memry mcp-headers` runs with.
type HeadersEnv struct {
	LookupEnv func(string) (string, bool)
	Out, Err  io.Writer
}

// Headers prints the authorization header of the saved login as a JSON
// object, for Claude Code's headersHelper, and returns the exit code.
func Headers(env HeadersEnv) int {
	getenv := func(key string) string {
		value, _ := env.LookupEnv(key)
		return value
	}
	cfg, err := config.Load(config.Path(getenv))
	var token string
	if err == nil {
		token, _ = cfg.Token()
	}
	if token == "" {
		_, _ = io.WriteString(env.Err, "memry is not logged in. Run memry setup.\n")
		return 1
	}
	// json_encode without flags, as the PHP CLI writes it.
	line := `{"Authorization":` + string(phpjson.EncodeString("Bearer "+token, false)) + "}\n"
	_, _ = io.WriteString(env.Out, line)
	return 0
}
