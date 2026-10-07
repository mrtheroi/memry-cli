package mcp_test

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/mrtheroi/memry-cli/internal/mcp"
)

// Ported from "forwards a request line and prints the server response as
// one line".
func TestForwardsALineAndPrintsTheReply(t *testing.T) {
	s := newServer(t, respond(200, `{"jsonrpc":"2.0","id":1,"result":{"tools":[]}}`))

	code, output := newHarness(t, s.URL).run(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)

	assertOutput(t, code, output, `{"jsonrpc":"2.0","id":1,"result":{"tools":[]}}`+"\n")
}

// Ported from "posts the line as is to the MCP endpoint with the bearer
// token and JSON headers".
func TestPostsTheLineWithTheTokenAndJSONHeaders(t *testing.T) {
	s := newServer(t, respond(200, `{"jsonrpc":"2.0","id":1,"result":[]}`))
	line := `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`

	newHarness(t, s.URL).run(line)

	sent := s.received()
	if len(sent) != 1 {
		t.Fatalf("sent %d requests, want 1", len(sent))
	}
	r := sent[0]
	if r.Method != "POST" || r.Path != "/mcp/memory" || r.Body != line {
		t.Errorf("sent %s %s %q, want POST /mcp/memory %q", r.Method, r.Path, r.Body, line)
	}
	for name, want := range map[string]string{
		"Authorization": "Bearer secret-token",
		"Accept":        "application/json, text/event-stream",
		"Content-Type":  "application/json",
	} {
		if got := r.Header.Values(name); len(got) != 1 || got[0] != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}
}

// Ported from "prints nothing for a notification the server accepts with
// an empty 202".
func TestPrintsNothingForAnAcceptedNotification(t *testing.T) {
	s := newServer(t, respond(202, ""))

	code, output := newHarness(t, s.URL).run(`{"jsonrpc":"2.0","method":"notifications/initialized"}`)

	assertOutput(t, code, output, "")
	if n := len(s.received()); n != 1 {
		t.Errorf("sent %d requests, want 1", n)
	}
}

// Ported from "processes every line in order until stdin ends".
func TestProcessesEveryLineInOrder(t *testing.T) {
	s := newServer(t, func(w http.ResponseWriter, r request) {
		var message struct {
			ID     any    `json:"id"`
			Method string `json:"method"`
		}
		_ = json.Unmarshal([]byte(r.Body), &message)
		if message.Method == "notifications/initialized" {
			return
		}
		_, _ = fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%v,"result":[]}`, message.ID)
	})

	code, output := newHarness(t, s.URL).run(
		`{"jsonrpc":"2.0","id":1,"method":"initialize"}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
	)

	assertOutput(t, code, output, `{"jsonrpc":"2.0","id":1,"result":[]}`+"\n"+`{"jsonrpc":"2.0","id":2,"result":[]}`+"\n")
	if n := len(s.received()); n != 3 {
		t.Errorf("sent %d requests, want 3", n)
	}
}

// Ported from "answers every request with a not logged in error without
// contacting the server".
func TestAnswersNotLoggedInWithoutContactingTheServer(t *testing.T) {
	for name, config := range map[string]string{
		"missing config": "",
		"missing url":    `{"token":"secret-token"}`,
		"empty token":    `{"url":"URL","token":""}`,
	} {
		t.Run(name, func(t *testing.T) {
			s := newServer(t, respond(200, `{"jsonrpc":"2.0","id":1,"result":[]}`))
			h := newHarness(t, s.URL)
			if config == "" {
				_ = os.Remove(h.configPath)
			} else {
				h.writeConfig(strings.ReplaceAll(config, "URL", s.URL))
			}

			code, output := h.run(
				`{"jsonrpc":"2.0","id":1,"method":"initialize"}`,
				`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
				`{"jsonrpc":"2.0","id":"two","method":"tools/list"}`,
			)

			assertOutput(t, code, output,
				`{"jsonrpc":"2.0","id":1,"error":{"code":-32000,"message":"Not logged in to memry. Run `+"`memry setup`"+`."}}`+"\n"+
					`{"jsonrpc":"2.0","id":"two","error":{"code":-32000,"message":"Not logged in to memry. Run `+"`memry setup`"+`."}}`+"\n")
			if n := len(s.received()); n != 0 {
				t.Errorf("sent %d requests, want none", n)
			}
		})
	}
}

// Ported from "answers a request with a login error when the server
// rejects the token".
func TestAnswersALoginErrorWhenTheTokenIsRejected(t *testing.T) {
	s := newServer(t, respond(401, `{"message":"Unauthenticated."}`))

	code, output := newHarness(t, s.URL).run(`{"jsonrpc":"2.0","id":7,"method":"tools/list"}`)

	assertOutput(t, code, output, `{"jsonrpc":"2.0","id":7,"error":{"code":-32000,"message":"Your memry login is no longer valid. Run `+"`memry setup`"+`."}}`+"\n")
}

// Ported from "answers a request with an internal error when the server
// fails without a JSON-RPC reply and keeps going".
func TestAnswersAnInternalErrorForAFailureWithoutAJSONRPCReply(t *testing.T) {
	for name, failure := range map[string]struct {
		status int
		body   string
	}{
		"server error page":       {500, "<html>Server Error</html>"},
		"rate limited":            {429, `{"message":"Too Many Attempts."}`},
		"unavailable, empty body": {503, ""},
	} {
		t.Run(name, func(t *testing.T) {
			s := newServer(t, sequence(respond(failure.status, failure.body), respond(200, `{"jsonrpc":"2.0","id":2,"result":[]}`)))

			code, output := newHarness(t, s.URL).run(
				`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`,
				`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
			)

			assertOutput(t, code, output, fmt.Sprintf(`{"jsonrpc":"2.0","id":1,"error":{"code":-32603,"message":"The memry server returned HTTP %d."}}`, failure.status)+"\n"+
				`{"jsonrpc":"2.0","id":2,"result":[]}`+"\n")
		})
	}
}

// Ported from "answers a request with an internal error instead of
// following a redirect".
func TestAnswersAnInternalErrorInsteadOfFollowingARedirect(t *testing.T) {
	s := newServer(t, func(w http.ResponseWriter, r request) {
		if r.Path == "/elsewhere" {
			respond(200, `{"jsonrpc":"2.0","id":1,"result":[]}`)(w, r)
			return
		}
		w.Header().Set("Location", "/elsewhere")
		w.WriteHeader(302)
	})

	code, output := newHarness(t, s.URL).run(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)

	assertOutput(t, code, output, `{"jsonrpc":"2.0","id":1,"error":{"code":-32603,"message":"The memry server returned HTTP 302."}}`+"\n")
	for _, r := range s.received() {
		if r.Path == "/elsewhere" {
			t.Error("followed the redirect")
		}
	}
}

// Ported from "answers a request with an internal error for a redirect
// whose body is a JSON-RPC reply".
func TestAnswersAnInternalErrorForARedirectWithAJSONRPCBody(t *testing.T) {
	s := newServer(t, func(w http.ResponseWriter, r request) {
		w.Header().Set("Location", "/elsewhere")
		respond(302, `{"jsonrpc":"2.0","id":1,"result":[]}`)(w, r)
	})

	code, output := newHarness(t, s.URL).run(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)

	assertOutput(t, code, output, `{"jsonrpc":"2.0","id":1,"error":{"code":-32603,"message":"The memry server returned HTTP 302."}}`+"\n")
}

// Ported from "forwards a JSON-RPC error the server sends with an error
// status".
func TestForwardsAJSONRPCErrorSentWithAnErrorStatus(t *testing.T) {
	jsonRPCError := `{"jsonrpc":"2.0","id":1,"error":{"code":-32601,"message":"The method [foo] was not found."}}`
	for _, status := range []int{400, 404, 500} {
		s := newServer(t, respond(status, jsonRPCError))

		code, output := newHarness(t, s.URL).run(`{"jsonrpc":"2.0","id":1,"method":"foo"}`)

		assertOutput(t, code, output, jsonRPCError+"\n")
	}
}

// Ported from "answers a request with an internal error when the server
// is unreachable and keeps going".
func TestAnswersAnInternalErrorWhenTheServerIsUnreachable(t *testing.T) {
	s := newServer(t, sequence(dropConnection, respond(200, `{"jsonrpc":"2.0","id":2,"result":[]}`)))

	code, output := newHarness(t, s.URL).run(
		`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
	)

	assertOutput(t, code, output, `{"jsonrpc":"2.0","id":1,"error":{"code":-32603,"message":"Could not reach the memry server."}}`+"\n"+
		`{"jsonrpc":"2.0","id":2,"result":[]}`+"\n")
}

// Not in the PHP tests: a reply cut off mid-body (here a valid JSON
// prefix) is a failed request, not a reply, as with PHP's HTTP client.
func TestAnswersAnInternalErrorForATruncatedReply(t *testing.T) {
	s := newServer(t, func(w http.ResponseWriter, _ request) {
		w.Header().Set("Content-Length", "100")
		_, _ = io.WriteString(w, `{"jsonrpc":"2.0","id":1,"result":[]}`)
	})

	code, output := newHarness(t, s.URL).run(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)

	assertOutput(t, code, output, `{"jsonrpc":"2.0","id":1,"error":{"code":-32603,"message":"Could not reach the memry server."}}`+"\n")
}

// Not in the PHP tests: the proxy reads a reply of up to MaxReplyBytes
// and answers a longer one with an internal error instead of buffering it.
func TestAnswersAnInternalErrorForAReplyThatIsTooLarge(t *testing.T) {
	reply := func(size int) string {
		prefix, suffix := `{"jsonrpc":"2.0","id":1,"result":"`, `"}`
		return prefix + strings.Repeat("a", size-len(prefix)-len(suffix)) + suffix
	}
	largest := reply(mcp.MaxReplyBytes)
	s := newServer(t, sequence(respond(200, largest), respond(200, reply(mcp.MaxReplyBytes+1))))

	code, output := newHarness(t, s.URL).run(
		`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`,
		`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`,
	)

	assertOutput(t, code, output, largest+"\n"+
		`{"jsonrpc":"2.0","id":1,"error":{"code":-32603,"message":"The memry server returned a response that is too large."}}`+"\n")
}

// Ported from "answers a request with an internal error when the server
// reply is not JSON"; like PHP's json_validate, a reply that is not valid
// UTF-8 is not JSON either.
func TestAnswersAnInternalErrorWhenTheReplyIsNotJSON(t *testing.T) {
	for _, body := range []string{"<html>Maintenance</html>", "\"\xc3\x28\"", `"\` + `ud800"`} {
		s := newServer(t, respond(200, body))

		code, output := newHarness(t, s.URL).run(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)

		assertOutput(t, code, output, `{"jsonrpc":"2.0","id":1,"error":{"code":-32603,"message":"The memry server returned an invalid response."}}`+"\n")
	}
}

// Ported from "prints a multi-line server reply as a single line".
func TestPrintsAMultiLineReplyAsOneLine(t *testing.T) {
	pretty := "{\n    \"jsonrpc\": \"2.0\",\n    \"id\": 1,\n    \"result\": {\n        \"text\": \"a\\nb\"\n    }\n}\r\n"
	s := newServer(t, respond(200, pretty))

	code, output := newHarness(t, s.URL).run(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)

	assertOutput(t, code, output, `{    "jsonrpc": "2.0",    "id": 1,    "result": {        "text": "a\nb"    }}`+"\n")
}

// Ported from "answers a line that is not JSON with a parse error without
// contacting the server and keeps going"; like PHP's json_validate, a line
// that is not valid UTF-8 is not JSON either.
func TestAnswersAParseErrorForALineThatIsNotJSON(t *testing.T) {
	for _, line := range []string{"{not json", "\"\xc3\x28\""} {
		s := newServer(t, respond(200, `{"jsonrpc":"2.0","id":2,"result":[]}`))

		code, output := newHarness(t, s.URL).run(line, `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`)

		assertOutput(t, code, output, `{"jsonrpc":"2.0","id":null,"error":{"code":-32700,"message":"Parse error"}}`+"\n"+
			`{"jsonrpc":"2.0","id":2,"result":[]}`+"\n")
		if n := len(s.received()); n != 1 {
			t.Errorf("sent %d requests, want 1", n)
		}
	}
}

// Ported from "ignores blank lines". Blank is what PHP's trim() strips:
// a form feed is not blank, so that line is not JSON.
func TestIgnoresBlankLines(t *testing.T) {
	s := newServer(t, respond(200, `{"jsonrpc":"2.0","id":1,"result":[]}`))

	code, output := newHarness(t, s.URL).run("", "   ", "\t\x00\x0b\r")
	assertOutput(t, code, output, "")

	code, output = newHarness(t, s.URL).run("\f")
	assertOutput(t, code, output, `{"jsonrpc":"2.0","id":null,"error":{"code":-32700,"message":"Parse error"}}`+"\n")
	if n := len(s.received()); n != 0 {
		t.Errorf("sent %d requests, want none", n)
	}
}

// Ported from "mirrors the protocol version, method and name into headers
// for stateless protocol requests".
func TestMirrorsTheProtocolHeadersOfStatelessRequests(t *testing.T) {
	meta := `"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28","io.modelcontextprotocol/clientCapabilities":{}}`
	tests := []struct{ method, params, name string }{
		{"tools/list", `{` + meta + `}`, ""},
		{"tools/call", `{"name":"search-memory","arguments":[],` + meta + `}`, "search-memory"},
		{"prompts/get", `{"name":"a-prompt",` + meta + `}`, "a-prompt"},
		{"resources/read", `{"uri":"memry://context",` + meta + `}`, "memry://context"},
	}
	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			s := newServer(t, respond(200, `{"jsonrpc":"2.0","id":1,"result":[]}`))

			newHarness(t, s.URL).run(`{"jsonrpc":"2.0","id":1,"method":"` + tt.method + `","params":` + tt.params + `}`)

			header := s.received()[0].Header
			assertHeader(t, header, "MCP-Protocol-Version", "2026-07-28")
			assertHeader(t, header, "Mcp-Method", tt.method)
			assertHeader(t, header, "Mcp-Name", tt.name)
		})
	}
}

// Ported from "sends no protocol headers for requests of the initialize
// handshake protocols".
func TestSendsNoProtocolHeadersForHandshakeProtocols(t *testing.T) {
	s := newServer(t, respond(200, `{"jsonrpc":"2.0","id":1,"result":[]}`))

	newHarness(t, s.URL).run(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"search-memory"}}`)

	header := s.received()[0].Header
	for _, name := range []string{"MCP-Protocol-Version", "Mcp-Method", "Mcp-Name"} {
		assertHeader(t, header, name, "")
	}
}

// Not in the PHP tests: the headers follow PHP's is_string checks (a null
// version is no version) and its array_filter, which drops the values ""
// and "0".
func TestProtocolHeadersFollowPHPTypeChecks(t *testing.T) {
	tests := []struct {
		line                  string
		version, method, name string
	}{
		{`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"x","_meta":{"io.modelcontextprotocol/protocolVersion":null}}}`, "", "", ""},
		{`{"jsonrpc":"2.0","id":1,"method":null,"params":{"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28"}}}`, "", "", ""},
		{`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"0","_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28"}}}`, "2026-07-28", "tools/call", ""},
		{`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":42,"_meta":{"io.modelcontextprotocol/protocolVersion":""}}}`, "", "tools/call", ""},
		{`{"jsonrpc":"2.0","id":1,"method":"0","params":{"_meta":{"io.modelcontextprotocol/protocolVersion":"0"}}}`, "", "", ""},
	}
	for _, tt := range tests {
		s := newServer(t, respond(200, `{"jsonrpc":"2.0","id":1,"result":[]}`))

		newHarness(t, s.URL).run(tt.line)

		header := s.received()[0].Header
		assertHeader(t, header, "MCP-Protocol-Version", tt.version)
		assertHeader(t, header, "Mcp-Method", tt.method)
		assertHeader(t, header, "Mcp-Name", tt.name)
	}
}

// Ported from "uses a new login without being restarted".
func TestUsesANewLoginWithoutBeingRestarted(t *testing.T) {
	var h *harness
	s := newServer(t, func(w http.ResponseWriter, r request) {
		if r.Header.Get("Authorization") == "Bearer secret-token" {
			h.writeConfig(`{"url":"` + h.url + `","token":"new-token"}`)
			respond(401, `{"message":"Unauthenticated."}`)(w, r)
			return
		}
		respond(200, `{"jsonrpc":"2.0","id":2,"result":[]}`)(w, r)
	})
	h = newHarness(t, s.URL)

	_, output := h.run(
		`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
	)

	if !strings.HasSuffix(output, `{"jsonrpc":"2.0","id":2,"result":[]}`+"\n") {
		t.Errorf("output = %q, want it to end with the reply to the new login", output)
	}
	if got := s.received()[1].Header.Get("Authorization"); got != "Bearer new-token" {
		t.Errorf("second Authorization = %q, want Bearer new-token", got)
	}
}

// Not in the PHP tests: MCP messages can be large, and a line is read
// whole however long it is (PHP's fgets has no limit either).
func TestForwardsALongLine(t *testing.T) {
	s := newServer(t, respond(200, `{"jsonrpc":"2.0","id":1,"result":[]}`))
	line := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"text":"` + strings.Repeat("a", 200<<10) + `"}}`

	code, output := newHarness(t, s.URL).run(line)

	assertOutput(t, code, output, `{"jsonrpc":"2.0","id":1,"result":[]}`+"\n")
	if sent := s.received(); len(sent) != 1 || sent[0].Body != line {
		t.Errorf("did not forward the %d-byte line as is", len(line))
	}
}

// Not in the PHP tests: like fgets, the last line counts even without a
// newline at the end of the input.
func TestForwardsTheLastLineWithoutANewline(t *testing.T) {
	s := newServer(t, respond(200, `{"jsonrpc":"2.0","id":1,"result":[]}`))
	h := newHarness(t, s.URL)
	var out bytes.Buffer

	code := mcp.Proxy(h.env(strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`), &out))

	assertOutput(t, code, out.String(), `{"jsonrpc":"2.0","id":1,"result":[]}`+"\n")
}

// Ported from "answers each line before stdin is closed": the proxy
// streams, answering a line as soon as it is read.
func TestAnswersEachLineBeforeStdinIsClosed(t *testing.T) {
	h := newHarness(t, unreachableURL)
	_ = os.Remove(h.configPath)
	stdin, writeStdin := io.Pipe()
	readStdout, stdout := io.Pipe()
	exit := make(chan int)
	go func() { exit <- mcp.Proxy(h.env(stdin, stdout)) }()

	_, _ = io.WriteString(writeStdin, `{"jsonrpc":"2.0","id":1,"method":"initialize"}`+"\n")
	answered, err := bufio.NewReader(readStdout).ReadString('\n')
	_ = writeStdin.Close()

	if want := `{"jsonrpc":"2.0","id":1,"error":{"code":-32000,"message":"Not logged in to memry. Run ` + "`memry setup`" + `."}}` + "\n"; err != nil || answered != want {
		t.Errorf("answered %q (%v) before EOF, want %q", answered, err, want)
	}
	select {
	case code := <-exit:
		if code != 0 {
			t.Errorf("exit code = %d, want 0", code)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the proxy did not stop at EOF")
	}
}

// Not in the PHP tests: once the agent stops reading the replies, the
// proxy stops too (with exit code 1) instead of forwarding more messages.
func TestStopsWhenTheOutputFails(t *testing.T) {
	s := newServer(t, respond(200, `{"jsonrpc":"2.0","id":1,"result":[]}`))
	h := newHarness(t, s.URL)
	lines := `{"jsonrpc":"2.0","id":1,"method":"tools/list"}` + "\n" + `{"jsonrpc":"2.0","id":2,"method":"tools/list"}` + "\n"

	code := mcp.Proxy(h.env(strings.NewReader(lines), failingWriter{}))

	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if n := len(s.received()); n != 1 {
		t.Errorf("sent %d requests, want 1", n)
	}
}

// Not in the PHP tests: a saved URL that is not a URL answers like an
// unreachable server, and the proxy keeps going (PHP's HTTP client throws
// on an invalid host, which stops the PHP proxy).
func TestAnswersAnInternalErrorForAnInvalidSavedURL(t *testing.T) {
	for _, url := range []string{"http://bad host", "not-a-url"} {
		h := newHarness(t, url)

		code, output := h.run(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`, `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`)

		assertOutput(t, code, output, `{"jsonrpc":"2.0","id":1,"error":{"code":-32603,"message":"Could not reach the memry server."}}`+"\n"+
			`{"jsonrpc":"2.0","id":2,"error":{"code":-32603,"message":"Could not reach the memry server."}}`+"\n")
	}
}

// Not in the PHP tests: an error answers with the request id as PHP's
// json_encode writes it back, and messages without an id (null, a batch,
// a scalar) get no error answer.
func TestErrorRepliesWriteTheIDLikePHP(t *testing.T) {
	h := newHarness(t, unreachableURL)
	_ = os.Remove(h.configPath)

	code, output := h.run(
		`{"jsonrpc":"2.0","id":"é/x","method":"a"}`,
		`{"jsonrpc":"2.0","id":2.50,"method":"a"}`,
		`{"jsonrpc":"2.0","id":null,"method":"a"}`,
		`[{"jsonrpc":"2.0","id":3,"method":"a"}]`,
		`5`,
	)

	notLoggedIn := `"error":{"code":-32000,"message":"Not logged in to memry. Run ` + "`memry setup`" + `."}}`
	assertOutput(t, code, output, `{"jsonrpc":"2.0","id":"\`+`u00e9/x",`+notLoggedIn+"\n"+`{"jsonrpc":"2.0","id":2.5,`+notLoggedIn+"\n")
}
