// Package hook holds memry's Claude Code hooks: `memry
// hook:session-start` prints the memry context of the current project.
package hook

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"

	"github.com/mrtheroi/memry-cli/internal/client"
	"github.com/mrtheroi/memry-cli/internal/config"
	"github.com/mrtheroi/memry-cli/internal/phpjson"
)

// Env is what the hook runs with.
type Env struct {
	LookupEnv func(string) (string, bool)
	In        io.Reader
	Out       io.Writer
	HTTP      *http.Client
	// Getwd is the working directory, used when the hook input has no cwd.
	Getwd func() (string, error)
	// GitTopLevel is the top-level directory of the git repository dir is
	// in, or false outside one.
	GitTopLevel func(dir string) (string, bool)
}

// SessionStart prints the usage protocol and the memry context of the
// project Claude Code starts in. It always exits 0, printing nothing when
// anything fails, so it never blocks a session.
func SessionStart(env Env) int {
	if context, ok := env.context(); ok {
		_, _ = io.WriteString(env.Out, context)
	}
	return 0
}

// context is what the hook prints, or false when it prints nothing.
func (env Env) context() (string, bool) {
	input, err := io.ReadAll(env.In)
	if err != nil {
		return "", false
	}
	cwd, ok := env.cwd(input)
	if !ok {
		return "", false
	}
	url, token, ok := config.Login(env.getenv)
	if !ok {
		return "", false
	}
	root, ok := env.GitTopLevel(cwd)
	if !ok {
		root = cwd
	}
	repo := phpBasename(root)
	project, declared := declaredProject(root)
	if !declared {
		project = repo
	}

	req, err := http.NewRequest(http.MethodGet, url+"/api/context?project="+rawURLEncode(project), nil)
	if err != nil {
		return "", false
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "text/plain")
	resp, err := env.HTTP.Do(req)
	if err != nil {
		return "", false
	}
	defer func() { _ = resp.Body.Close() }()
	if !client.Succeeded(resp) {
		return "", false
	}
	body, err := client.ReadBody(resp.Body, maxContextBytes)
	if err != nil {
		return "", false
	}
	// Like the bash hook's $(curl ...), drop the body's trailing newlines.
	return fmt.Sprintf(protocol, project, project, project, repo, strings.TrimRight(string(body), "\n")), true
}

// protocol is the usage protocol printed before the context, with the
// project (three times), the repo and the context body.
const protocol = "## memry memory (project: %s)\n" +
	"memry is available through the `memry` MCP tools.\n" +
	"- Use get-memory with an id to read a memory from the context below in full, and search-memory to find older ones.\n" +
	"- Save decisions, bug fixes and discoveries with save-memory (project \"%s\", with a topic_key for evolving topics). Save a memory about another product under that product's project instead.\n" +
	"- Before ending the session, save a summary with session-summary (project \"%s\", repo \"%s\").\n" +
	"\n" +
	"%s\n"

// maxContextBytes bounds the context the hook reads.
const maxContextBytes = 1 << 20

func (env Env) getenv(key string) string {
	value, _ := env.LookupEnv(key)
	return value
}

// GitTopLevel runs git to find the top-level directory of the repository
// dir is in.
func GitTopLevel(dir string) (string, bool) {
	out, err := exec.Command("git", "-C", dir, "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", false
	}
	return strings.Trim(string(out), phpTrimmed), true
}

// phpTrimmed are the characters PHP's trim() strips.
const phpTrimmed = " \t\n\r\x00\x0B"

// declaredProject is the project declared in the .memry.json of root, or
// false when there is no usable one.
func declaredProject(root string) (string, bool) {
	data, err := os.ReadFile(root + "/.memry.json")
	if err != nil {
		return "", false
	}
	// PHP matches the key exactly, where Go's struct decoding would not.
	var file map[string]json.RawMessage
	var project string
	if !phpjson.Valid(data) || json.Unmarshal(data, &file) != nil || json.Unmarshal(file["project"], &project) != nil {
		return "", false
	}
	project = strings.Trim(project, phpTrimmed)
	if project == "" {
		return "", false
	}
	return project, true
}

// cwd is the cwd of the hook input, or the working directory when it has
// none: what PHP's ?: treats as false ("", "0", 0, false, null, empty or
// missing). Unlike the PHP CLI, which casts any other value that is not a
// string (such as 42) to one, it is no cwd at all: the hook prints nothing.
func (env Env) cwd(input []byte) (string, bool) {
	var hookInput map[string]json.RawMessage
	if phpjson.Valid(input) {
		_ = json.Unmarshal(input, &hookInput)
	}
	var value any
	_ = json.Unmarshal(hookInput["cwd"], &value)
	if phpFalsy(value) {
		wd, err := env.Getwd()
		return wd, err == nil
	}
	cwd, ok := value.(string)
	return cwd, ok
}

// phpFalsy reports whether PHP converts the decoded JSON value to false.
func phpFalsy(value any) bool {
	switch v := value.(type) {
	case nil:
		return true
	case bool:
		return !v
	case string:
		return v == "" || v == "0"
	case float64:
		return v == 0
	case []any:
		return len(v) == 0
	case map[string]any:
		return len(v) == 0
	}
	return false
}

// rawURLEncode encodes s like PHP's rawurlencode: every byte but letters,
// digits and "-_.~" as %XX.
func rawURLEncode(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z' || '0' <= c && c <= '9' || strings.IndexByte("-_.~", c) >= 0 {
			b.WriteByte(c)
		} else {
			fmt.Fprintf(&b, "%%%02X", c)
		}
	}
	return b.String()
}

// phpBasename is the last element of path like PHP's basename: trailing
// slashes are ignored, and the root directory has none ("").
func phpBasename(path string) string {
	path = strings.TrimRight(path, "/")
	return path[strings.LastIndexByte(path, '/')+1:]
}
