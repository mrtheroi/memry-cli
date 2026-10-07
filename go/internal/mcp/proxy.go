// Package mcp connects agents to the memry MCP server: `memry mcp`, the
// stdio proxy, and `memry mcp-headers`, Claude Code's headers helper.
package mcp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/mrtheroi/memry-cli/internal/client"
	"github.com/mrtheroi/memry-cli/internal/config"
	"github.com/mrtheroi/memry-cli/internal/phpjson"
)

// Env is what the proxy runs with.
type Env struct {
	LookupEnv func(string) (string, bool)
	In        io.Reader
	Out       io.Writer
	HTTP      *http.Client
}

// Proxy forwards each JSON-RPC line of env.In to the memry MCP server and
// prints its replies, until the input ends. It returns the exit code.
func Proxy(env Env) int {
	in := bufio.NewReader(env.In)
	for {
		line, err := in.ReadBytes('\n')
		if message := bytes.Trim(line, phpTrimmed); len(message) > 0 {
			if reply := env.forward(message); reply != nil {
				if _, err := env.Out.Write(append(reply, '\n')); err != nil {
					return 1
				}
			}
		}
		if err != nil {
			return 0
		}
	}
}

// forward sends one message to the server and returns the line to print,
// or nil when there is nothing to print.
func (env Env) forward(message []byte) []byte {
	if !phpjson.Valid(message) {
		return []byte(`{"jsonrpc":"2.0","id":null,"error":{"code":-32700,"message":"Parse error"}}`)
	}
	url, token, ok := config.Login(env.getenv)
	if !ok {
		return errorReply(message, -32000, "Not logged in to memry. Run `memry setup`.")
	}
	req, err := http.NewRequest(http.MethodPost, url+"/mcp/memory", bytes.NewReader(message))
	if err != nil {
		return unreachable(message)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("Content-Type", "application/json")
	for name, value := range protocolHeaders(message) {
		req.Header.Set(name, value)
	}
	resp, err := env.HTTP.Do(req)
	if err != nil {
		return unreachable(message)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusUnauthorized {
		return errorReply(message, -32000, "Your memry login is no longer valid. Run `memry setup`.")
	}
	// Neither a success nor an error, such as a redirect: whatever its body says.
	if !client.Succeeded(resp) && resp.StatusCode < 400 {
		return httpError(message, resp.StatusCode)
	}
	body, err := client.ReadBody(resp.Body, maxReplyBytes)
	if errors.Is(err, client.ErrTooLarge) {
		return errorReply(message, -32603, "The memry server returned a response that is too large.")
	}
	if err != nil {
		return unreachable(message)
	}
	// laravel/mcp answers JSON-RPC errors with a 4xx or 5xx status.
	if resp.StatusCode >= 400 && !isJSONRPC(body) {
		return httpError(message, resp.StatusCode)
	}
	if len(body) == 0 {
		return nil
	}
	if !phpjson.Valid(body) {
		return errorReply(message, -32603, "The memry server returned an invalid response.")
	}
	// Line breaks in valid JSON can only be whitespace between tokens.
	return bytes.Map(dropLineBreaks, body)
}

// maxReplyBytes bounds every reply the proxy reads, so a misbehaving
// server cannot make it buffer an unbounded body. MCP replies carry whole
// memories and search results, so the limit is far above setup's 1 MiB.
const maxReplyBytes = 16 << 20

func (env Env) getenv(key string) string {
	value, _ := env.LookupEnv(key)
	return value
}

// errorReply is a JSON-RPC error answering message, as PHP's json_encode
// writes it, or nil when message is a notification (it has no id), which
// gets no answer.
func errorReply(message []byte, code int, text string) []byte {
	var request map[string]json.RawMessage
	if json.Unmarshal(message, &request) != nil {
		return nil
	}
	id, ok := request["id"]
	if !ok || string(id) == "null" {
		return nil
	}
	encodedID, ok := phpjson.Reencode(id)
	if !ok {
		return nil
	}
	return fmt.Appendf(nil, `{"jsonrpc":"2.0","id":%s,"error":{"code":%d,"message":%s}}`, encodedID, code, phpjson.EncodeString(text, true))
}

// isJSONRPC reports whether body is a JSON object with a non-null
// "jsonrpc", as PHP's isset($response->json()['jsonrpc']) does.
func isJSONRPC(body []byte) bool {
	var reply map[string]json.RawMessage
	if !phpjson.Valid(body) || json.Unmarshal(body, &reply) != nil {
		return false
	}
	version, ok := reply["jsonrpc"]
	return ok && string(version) != "null"
}

// httpError answers message with the server's unexpected HTTP status.
func httpError(message []byte, status int) []byte {
	return errorReply(message, -32603, fmt.Sprintf("The memry server returned HTTP %d.", status))
}

// unreachable answers message when the server could not be reached.
func unreachable(message []byte) []byte {
	return errorReply(message, -32603, "Could not reach the memry server.")
}

func dropLineBreaks(r rune) rune {
	if r == '\r' || r == '\n' {
		return -1
	}
	return r
}

// phpTrimmed are the characters PHP's trim() strips.
const phpTrimmed = " \t\n\r\x00\x0B"

// protocolHeaders are the headers the stateless MCP protocol (2026-07-28)
// requires to mirror the request body. Requests of the older initialize
// handshake protocols carry no protocol version in their _meta and need
// none.
func protocolHeaders(message []byte) map[string]string {
	request := object(message)
	params := object(request["params"])
	version, hasVersion := str(object(params["_meta"])["io.modelcontextprotocol/protocolVersion"])
	method, hasMethod := str(request["method"])
	if !hasVersion || !hasMethod {
		return nil
	}
	var name string
	switch method {
	case "tools/call", "prompts/get":
		name, _ = str(params["name"])
	case "resources/read":
		name, _ = str(params["uri"])
	}
	headers := map[string]string{}
	for header, value := range map[string]string{"MCP-Protocol-Version": version, "Mcp-Method": method, "Mcp-Name": name} {
		// Like PHP's array_filter, which drops the falsy strings.
		if value != "" && value != "0" {
			headers[header] = value
		}
	}
	return headers
}

// object is the JSON object raw, or nil when it is not one.
func object(raw json.RawMessage) map[string]json.RawMessage {
	var members map[string]json.RawMessage
	if json.Unmarshal(raw, &members) != nil {
		return nil
	}
	return members
}

// str is the JSON string raw, or false when it is not one (null is not).
func str(raw json.RawMessage) (string, bool) {
	var s string
	return s, len(raw) > 0 && raw[0] == '"' && json.Unmarshal(raw, &s) == nil
}
