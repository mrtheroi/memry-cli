package usage

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"unicode/utf8"

	"github.com/mrtheroi/memry-cli/internal/client"
)

// The memry server's usage token API, all in one place. Both calls take
// the login (full access) token.
const (
	// issuePath issues a new usage token, which can only send usage
	// metrics, without revoking any: 201 {"id": 42, "token": "42|plain"}.
	issuePath = "/api/auth/usage-token"
	// revokePath revokes every usage token of the caller or, with
	// ?keep=<id>, only the ones issued before that one (a later one of
	// another client survives); 422, revoking none, when it is not one of
	// them. It answers 204.
	revokePath = "/api/auth/usage-tokens"
	keepParam  = "keep"
)

// issued is a usage token the server issued.
type issued struct {
	ID    int64  `json:"id"`
	Token string `json:"token"`
}

// issueToken asks the memry server at url for a new usage token. It
// returns it, or why it failed.
func (env Env) issueToken(url, token string) (issued, string) {
	status, payload, err := env.send(http.MethodPost, url+issuePath, token)
	if err != nil {
		return issued{}, "Could not reach the memry server at " + url + "."
	}
	var body struct {
		issued
		Message string `json:"message"`
	}
	decoded := utf8.Valid(payload) && json.Unmarshal(payload, &body) == nil
	switch {
	case status == http.StatusUnauthorized:
		return issued{}, "Your memry login is no longer valid. Run `memry setup` and try again."
	case status == http.StatusTooManyRequests:
		return issued{}, "Too many attempts, try again later."
	case !succeeded(status) && decoded && body.Message != "":
		return issued{}, body.Message
	case !succeeded(status):
		return issued{}, fmt.Sprintf("The memry server returned an unexpected error (HTTP %d).", status)
	case !decoded || body.Token == "" || body.ID == 0:
		return issued{}, "The memry server did not return a usage token."
	}
	return body.issued, ""
}

// revokeTokens revokes the usage tokens of the login token on the memry
// server at url, all of them or, with keep, the ones issued before it,
// reporting whether it did.
func (env Env) revokeTokens(url, token string, keep int64) bool {
	target := url + revokePath
	if keep != 0 {
		target += "?" + keepParam + "=" + strconv.FormatInt(keep, 10)
	}
	status, _, err := env.send(http.MethodDelete, target, token)
	return err == nil && succeeded(status)
}

// send sends a request that accepts JSON with the bearer token, returning
// the response status and body.
func (env Env) send(method, url, token string) (int, []byte, error) {
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := env.HTTP.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	payload, err := client.ReadBody(resp.Body, maxResponseBytes)
	return resp.StatusCode, payload, err
}

// succeeded reports whether status is a success: 2xx only, never a
// redirect.
func succeeded(status int) bool {
	return client.Succeeded(&http.Response{StatusCode: status})
}

// maxResponseBytes bounds the response read, so a misbehaving server
// cannot make memry buffer an unbounded body. The real ones are tiny.
const maxResponseBytes = 1 << 20
