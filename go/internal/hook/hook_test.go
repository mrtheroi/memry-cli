package hook_test

import (
	"errors"
	"net/http"
	"os"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/mrtheroi/memry-cli/internal/hook"
)

// Ported from "prints the protocol block and the context body using the
// git top-level as project".
func TestPrintsTheProtocolAndTheContextOfTheGitTopLevel(t *testing.T) {
	s := newServer(t, respond(200, "## Latest session\nDid things"))
	h := newHarness(t, s.URL)
	repo := gitRepo(t, h.dir+"/MyProject")
	mkdir(t, repo+"/src/deep")

	code, output := h.run(map[string]any{"session_id": "abc", "cwd": repo + "/src/deep", "source": "startup"})

	assertOutput(t, code, output, "## memry memory (project: MyProject)\n"+
		"memry is available through the `memry` MCP tools.\n"+
		"- Use get-memory with an id to read a memory from the context below in full, and search-memory to find older ones.\n"+
		"- Save decisions, bug fixes and discoveries with save-memory (project \"MyProject\", with a topic_key for evolving topics). Save a memory about another product under that product's project instead.\n"+
		"- Before ending the session, save a summary with session-summary (project \"MyProject\", repo \"MyProject\").\n"+
		"\n"+
		"## Latest session\n"+
		"Did things\n")
}

// Ported from "uses the project from .memry.json at the git top-level and
// the folder name as repo".
func TestUsesTheProjectOfMemryJSONAtTheGitTopLevel(t *testing.T) {
	s := newServer(t, respond(200, "body"))
	h := newHarness(t, s.URL)
	repo := gitRepo(t, h.dir+"/memry-cli")
	mkdir(t, repo+"/src/deep")
	writeFile(t, repo+"/.memry.json", `{"project": "memry"}`)

	code, output := h.run(map[string]any{"session_id": "abc", "cwd": repo + "/src/deep", "source": "startup"})

	assertExit(t, code)
	assertPrefix(t, output, "## memry memory (project: memry)\n")
	if !strings.Contains(output, `(project "memry", repo "memry-cli")`) {
		t.Errorf("output does not name project memry and repo memry-cli:\n%s", output)
	}
}

// Ported from "trims the project from .memry.json".
func TestTrimsTheProjectOfMemryJSON(t *testing.T) {
	s := newServer(t, respond(200, "body"))
	h := newHarness(t, s.URL)
	repo := gitRepo(t, h.dir+"/memry-cli")
	writeFile(t, repo+"/.memry.json", `{"project": "  memry \n"}`)

	_, output := h.run(map[string]any{"session_id": "abc", "cwd": repo, "source": "startup"})

	assertPrefix(t, output, "## memry memory (project: memry)\n")
}

// Ported from "falls back to the repo name when .memry.json has no usable
// project", plus what PHP's json_decode rejects: invalid UTF-8 and an
// unpaired surrogate.
func TestFallsBackToTheRepoNameWithoutAUsableProject(t *testing.T) {
	for name, contents := range map[string]string{
		"invalid json":       "not json",
		"missing project":    `{"name": "memry"}`,
		"empty project":      `{"project": ""}`,
		"blank project":      `{"project": "   "}`,
		"non-string project": `{"project": 42}`,
		"non-object json":    `"memry"`,
		"invalid UTF-8":      "{\"project\": \"memry\xc3\x28\"}",
		"lone surrogate":     `{"project": "memry\` + `ud800"}`,
		"other key case":     `{"Project": "memry"}`,
	} {
		t.Run(name, func(t *testing.T) {
			s := newServer(t, respond(200, "body"))
			h := newHarness(t, s.URL)
			repo := gitRepo(t, h.dir+"/memry-cli")
			writeFile(t, repo+"/.memry.json", contents)

			code, output := h.run(map[string]any{"session_id": "abc", "cwd": repo, "source": "startup"})

			assertExit(t, code)
			assertPrefix(t, output, "## memry memory (project: memry-cli)\n")
		})
	}
}

// Ported from "falls back to the cwd basename outside a git repository".
func TestFallsBackToTheCwdBasenameOutsideGit(t *testing.T) {
	s := newServer(t, respond(200, "body"))
	h := newHarness(t, s.URL)
	dir := mkdir(t, h.dir+"/PlainFolder")

	code, output := h.run(map[string]any{"session_id": "abc", "cwd": dir, "source": "startup"})

	assertExit(t, code)
	assertPrefix(t, output, "## memry memory (project: PlainFolder)\n")
}

// Ported from "honors .memry.json in the cwd outside a git repository".
func TestHonorsMemryJSONInTheCwdOutsideGit(t *testing.T) {
	s := newServer(t, respond(200, "body"))
	h := newHarness(t, s.URL)
	dir := mkdir(t, h.dir+"/PlainFolder")
	writeFile(t, dir+"/.memry.json", `{"project": "memry"}`)

	_, output := h.run(map[string]any{"session_id": "abc", "cwd": dir, "source": "startup"})

	assertPrefix(t, output, "## memry memory (project: memry)\n")
	if !strings.Contains(output, `repo "PlainFolder"`) {
		t.Errorf("output does not name repo PlainFolder:\n%s", output)
	}
}

// Ported from "falls back to the repo name when .memry.json is
// unreadable".
func TestFallsBackToTheRepoNameWhenMemryJSONIsUnreadable(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads any file")
	}
	s := newServer(t, respond(200, "body"))
	h := newHarness(t, s.URL)
	repo := gitRepo(t, h.dir+"/memry-cli")
	writeFile(t, repo+"/.memry.json", `{"project": "memry"}`)
	if err := os.Chmod(repo+"/.memry.json", 0); err != nil {
		t.Fatal(err)
	}

	code, output := h.run(map[string]any{"session_id": "abc", "cwd": repo, "source": "startup"})

	assertExit(t, code)
	assertPrefix(t, output, "## memry memory (project: memry-cli)\n")
}

// Ported from "uses the working directory when the input has no cwd",
// plus the other cwd values PHP's ?: treats as none ("0", 0, false) and a
// key in another case, which PHP does not match.
func TestUsesTheWorkingDirectoryWithoutACwd(t *testing.T) {
	for name, input := range map[string]map[string]any{
		"empty cwd":      {"session_id": "abc", "cwd": "", "source": "startup"},
		"null cwd":       {"session_id": "abc", "cwd": nil, "source": "startup"},
		"missing cwd":    {"session_id": "abc", "source": "startup"},
		"zero string":    {"cwd": "0"},
		"zero":           {"cwd": 0},
		"false":          {"cwd": false},
		"other key case": {"CWD": "/elsewhere"},
	} {
		t.Run(name, func(t *testing.T) {
			s := newServer(t, respond(200, "body"))
			h := newHarness(t, s.URL)
			dir := mkdir(t, h.dir+"/FromPwd")
			h.getwd = func() (string, error) { return dir, nil }

			code, output := h.run(input)

			assertExit(t, code)
			assertPrefix(t, output, "## memry memory (project: FromPwd)\n")
		})
	}
}

// Ported from "requests the context with the bearer token, an encoded
// project and a 3 second timeout" (the timeout is the command's client)
// and "requests the context of the project from .memry.json".
func TestRequestsTheContextOfTheEncodedProject(t *testing.T) {
	s := newServer(t, respond(200, "body"))
	h := newHarness(t, s.URL)
	plain := mkdir(t, h.dir+"/My Project")
	repo := gitRepo(t, h.dir+"/memry-cli")
	writeFile(t, repo+"/.memry.json", `{"project": "memry app"}`)

	_, output := h.run(map[string]any{"session_id": "abc", "cwd": plain, "source": "startup"})
	h.run(map[string]any{"session_id": "abc", "cwd": repo, "source": "startup"})

	sent := s.received()
	if len(sent) != 2 {
		t.Fatalf("sent %d requests, want 2", len(sent))
	}
	for i, query := range []string{"project=My%20Project", "project=memry%20app"} {
		r := sent[i]
		if r.Method != "GET" || r.Path != "/api/context" || r.RawQuery != query {
			t.Errorf("sent %s %s?%s, want GET /api/context?%s", r.Method, r.Path, r.RawQuery, query)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer secret-token" {
			t.Errorf("Authorization = %q, want Bearer secret-token", got)
		}
		if got := r.Header.Get("Accept"); got != "text/plain" {
			t.Errorf("Accept = %q, want text/plain", got)
		}
	}
	if strings.Contains(output, "secret-token") {
		t.Errorf("output shows the token:\n%s", output)
	}
}

// Not in the PHP tests: the project is encoded exactly like PHP's
// rawurlencode (the expected query is PHP 8.4's).
func TestEncodesTheProjectLikeRawURLEncode(t *testing.T) {
	s := newServer(t, respond(200, "body"))
	h := newHarness(t, s.URL)
	dir := mkdir(t, h.dir+"/plain")
	writeFile(t, dir+"/.memry.json", `{"project": "a+b/c?d=é&~ *_-.!"}`)

	h.run(map[string]any{"cwd": dir})

	if sent := s.received(); len(sent) != 1 || sent[0].RawQuery != "project=a%2Bb%2Fc%3Fd%3D%C3%A9%26~%20%2A_-.%21" {
		t.Errorf("sent %+v, want the project encoded like rawurlencode", sent)
	}
}

// Not in the PHP tests: a cwd that is neither a string nor empty is no
// cwd; the hook prints nothing (the PHP CLI would cast it to a string).
func TestPrintsNothingForACwdThatIsNotAString(t *testing.T) {
	for _, cwd := range []any{42, true, []string{"/tmp"}} {
		s := newServer(t, respond(200, "body"))

		code, output := newHarness(t, s.URL).run(map[string]any{"cwd": cwd})

		assertOutput(t, code, output, "")
	}
}

// Ported from "prints nothing, sends nothing and exits zero without a
// usable config".
func TestPrintsNothingWithoutAUsableConfig(t *testing.T) {
	for name, contents := range map[string]string{
		"missing file":  "",
		"without token": `{"url": "URL"}`,
		"without url":   `{"token": "secret-token"}`,
		"empty token":   `{"url": "URL", "token": ""}`,
		"invalid json":  "not json",
	} {
		t.Run(name, func(t *testing.T) {
			s := newServer(t, respond(200, "body"))
			h := newHarness(t, s.URL)
			if contents == "" {
				_ = os.Remove(h.configPath)
			} else {
				h.writeConfig(strings.ReplaceAll(contents, "URL", s.URL))
			}

			code, output := h.run(map[string]any{"session_id": "abc", "cwd": gitRepo(t, h.dir+"/MyProject"), "source": "startup"})

			assertOutput(t, code, output, "")
			if n := len(s.received()); n != 0 {
				t.Errorf("sent %d requests, want none", n)
			}
		})
	}
}

// Ported from "prints nothing and exits zero when the request fails".
func TestPrintsNothingWhenTheRequestFails(t *testing.T) {
	for name, handle := range map[string]func(http.ResponseWriter, *http.Request){
		"unauthorized":       respond(401, "Unauthenticated."),
		"server error":       respond(500, "partial"),
		"connection failure": dropConnection,
	} {
		t.Run(name, func(t *testing.T) {
			s := newServer(t, handle)
			h := newHarness(t, s.URL)

			code, output := h.run(map[string]any{"session_id": "abc", "cwd": gitRepo(t, h.dir+"/MyProject"), "source": "startup"})

			assertOutput(t, code, output, "")
		})
	}
}

// Ported from "prints nothing and exits zero instead of following a
// redirect".
func TestPrintsNothingInsteadOfFollowingARedirect(t *testing.T) {
	s := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/login" {
			respond(200, "<html>Log in</html>")(w, r)
			return
		}
		http.Redirect(w, r, "/login", http.StatusFound)
	})
	h := newHarness(t, s.URL)

	code, output := h.run(map[string]any{"session_id": "abc", "cwd": gitRepo(t, h.dir+"/MyProject"), "source": "startup"})

	assertOutput(t, code, output, "")
	for _, r := range s.received() {
		if r.Path == "/login" {
			t.Error("followed the redirect")
		}
	}
}

// Ported from "prints nothing and exits zero on an unexpected error".
func TestPrintsNothingOnAnUnexpectedError(t *testing.T) {
	s := newServer(t, respond(200, "body"))

	code, output := newHarness(t, s.URL).runWith(iotest.ErrReader(errors.New("stdin exploded")))

	assertOutput(t, code, output, "")
}

// Ported from "ends the output with exactly one newline like the bash
// hook".
func TestEndsWithExactlyOneNewline(t *testing.T) {
	s := newServer(t, respond(200, "body\n\n"))
	h := newHarness(t, s.URL)

	_, output := h.run(map[string]any{"session_id": "abc", "cwd": gitRepo(t, h.dir+"/MyProject"), "source": "startup"})

	if !strings.HasSuffix(output, "\n\nbody\n") || strings.HasSuffix(output, "body\n\n") {
		t.Errorf("output = %q, want it to end with \"\\n\\nbody\\n\"", output)
	}
}

// Not in the PHP tests: the repo is named like PHP's basename, which is
// empty for the root directory (Go's filepath.Base says "/").
func TestNamesTheRootDirectoryLikePHPBasename(t *testing.T) {
	if _, inGit := hook.GitTopLevel("/"); inGit {
		t.Skip("the root directory is in a git repository")
	}
	s := newServer(t, respond(200, "body"))

	_, output := newHarness(t, s.URL).run(map[string]any{"cwd": "/"})

	assertPrefix(t, output, "## memry memory (project: )\n")
}

// Not in the PHP tests: the hook reads a context of up to MaxContextBytes
// and prints nothing for a longer one instead of buffering it.
func TestPrintsNothingForAContextThatIsTooLarge(t *testing.T) {
	for size, printed := range map[int]bool{hook.MaxContextBytes: true, hook.MaxContextBytes + 1: false} {
		s := newServer(t, respond(200, strings.Repeat("a", size)))
		h := newHarness(t, s.URL)

		code, output := h.run(map[string]any{"cwd": mkdir(t, h.dir+"/MyProject")})

		assertExit(t, code)
		if (output != "") != printed {
			t.Errorf("printed %d bytes for a %d-byte context, want printed %v", len(output), size, printed)
		}
	}
}
