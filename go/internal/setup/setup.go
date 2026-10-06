// Package setup is the `memry setup` command: it logs in to memry and
// saves the credentials.
package setup

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"

	"github.com/mrtheroi/memry-cli/internal/agents"
	"github.com/mrtheroi/memry-cli/internal/client"
	"github.com/mrtheroi/memry-cli/internal/config"
	"github.com/mrtheroi/memry-cli/internal/email"
	"github.com/mrtheroi/memry-cli/internal/flags"
)

// Prompter asks the questions setup needs.
type Prompter interface {
	// Ask returns the trimmed answer, empty when there is none, or an
	// error when the input ended before an answer.
	Ask(label string) (string, error)
	// Secret returns the trimmed answer, typed without being shown.
	Secret(label string) string
}

// Env is what setup runs with.
type Env struct {
	// Args are the arguments after `setup`.
	Args      []string
	LookupEnv func(string) (string, bool)
	Unsetenv  func(string) error
	Out       io.Writer
	Prompter  Prompter
	Agents    agents.Registry
	HTTP      *http.Client
}

// Exit codes, as in the PHP CLI.
const (
	success = 0
	failure = 1
)

// Run runs setup and returns its exit code.
func Run(env Env) int {
	plan, err := flags.Resolve(flags.Scan(env.Args), env.LookupEnv, env.Agents.Keys())
	var usage *flags.Error
	if errors.As(err, &usage) {
		env.line(usage.Message)
		return usage.ExitCode
	}
	var address string
	if !plan.EmailGiven && !plan.Interactive {
		env.line("Pass --email when running without interaction.")
		return failure
	}
	if plan.EmailGiven {
		var ok bool
		if address, ok = email.Normalize(plan.Email); !ok {
			env.line("Invalid email address given with --email.")
			return failure
		}
	} else if address, err = env.askEmail(); err != nil {
		return env.aborted()
	}
	resp, err := env.post(plan.URL+"/api/auth/code", map[string]string{"email": address})
	if err != nil {
		return env.unreachable(plan.URL)
	}
	if !resp.succeeded() {
		env.failWith(resp)
		return failure
	}
	env.line("We sent a login code to " + address + ".")
	code, err := env.askCode(plan.Interactive)
	if err != nil {
		return env.aborted()
	}
	if code == "" {
		env.line("No login code given. Run `memry setup` interactively to enter the code from the email.")
		return failure
	}
	resp, err = env.post(plan.URL+"/api/auth/token", struct {
		Email string `json:"email"`
		Code  string `json:"code"`
	}{address, code})
	if err != nil {
		return env.unreachable(plan.URL)
	}
	if !resp.succeeded() {
		env.failWith(resp)
		return failure
	}
	token, ok := resp.jsonString("token")
	if !ok || token == "" {
		env.line("The memry server did not return a token.")
		return failure
	}

	path := config.Path(env.getenv)
	cfg, err := config.LoadOrEmpty(path)
	var previousURL, previousToken string
	var hadLogin bool
	if err == nil {
		var hasURL, hasToken bool
		previousURL, hasURL = cfg.URL()
		previousToken, hasToken = cfg.Token()
		hadLogin = hasURL && hasToken
		cfg.Set("url", plan.URL)
		cfg.Set("token", token)
		cfg.Set("agents", plan.Agents)
		err = cfg.Save()
	}
	if err != nil {
		env.line(fmt.Sprintf("Could not save the credentials to %s: %v.", path, err))
		return failure
	}
	env.line("Logged in as " + address + ". Credentials saved to " + path + ".")
	if hadLogin && previousToken != token {
		env.revokePreviousToken(previousURL, previousToken)
	}
	return success
}

// askEmail asks for the email until it is a valid address.
func (env Env) askEmail() (string, error) {
	for {
		answer, err := env.Prompter.Ask("Email")
		if err != nil {
			return "", err
		}
		if address, ok := email.Normalize(answer); ok {
			return address, nil
		}
		env.line("Enter a valid email address.")
	}
}

// sixDigits is a login code.
var sixDigits = regexp.MustCompile(`^[0-9]{6}$`)

// askCode asks for the login code until it has 6 digits. It is empty when
// no answer can be read at all, as when not interactive.
func (env Env) askCode(interactive bool) (string, error) {
	if !interactive {
		return "", nil
	}
	for {
		answer, err := env.Prompter.Ask("Login code")
		if err != nil || answer == "" {
			return "", err
		}
		if code := phpTrim(answer); sixDigits.MatchString(code) {
			return code, nil
		}
		env.line("Enter the 6-digit code from the email.")
	}
}

// phpTrim trims the characters PHP's trim() does.
func phpTrim(s string) string {
	return strings.Trim(s, " \t\n\r\x00\x0B")
}

// unreachable fails because the server at url could not be reached.
func (env Env) unreachable(url string) int {
	env.line("Could not reach the memry server at " + url + ".")
	return failure
}

// aborted fails like Symfony when the input ends before an answer.
func (env Env) aborted() int {
	env.line("Aborted.")
	return failure
}

// line prints one line of output.
func (env Env) line(text string) {
	_, _ = io.WriteString(env.Out, text+"\n")
}

func (env Env) getenv(key string) string {
	value, _ := env.LookupEnv(key)
	return value
}

// response is a server response: its status and body.
type response struct {
	status int
	body   []byte
}

// succeeded reports whether the response is a success (see
// client.Succeeded): 2xx only, never a redirect.
func (r response) succeeded() bool {
	return client.Succeeded(&http.Response{StatusCode: r.status})
}

// failWith prints the server's message for a failed response.
func (env Env) failWith(r response) {
	if r.status == http.StatusTooManyRequests {
		env.line("Too many attempts, try again later.")
		return
	}
	if message, ok := r.jsonString("message"); ok {
		env.line(message)
		return
	}
	env.line(fmt.Sprintf("The memry server returned an unexpected error (HTTP %d).", r.status))
}

// jsonString returns the string at key of a JSON object body, like
// Laravel's $response->json($key) when it is a string.
func (r response) jsonString(key string) (string, bool) {
	var body map[string]any
	if json.Unmarshal(r.body, &body) != nil {
		return "", false
	}
	value, ok := body[key].(string)
	return value, ok
}

// revokePreviousToken revokes the token of the previous login on the
// server it belongs to. A failure never fails setup: the new login is
// already saved.
func (env Env) revokePreviousToken(url, token string) {
	resp, err := env.send(http.MethodDelete, url+"/api/auth/token", token, nil)
	if err == nil && resp.succeeded() {
		env.line("Revoked the previous memry token.")
	} else {
		env.line("Could not revoke the previous memry token.")
	}
}

// post sends data as JSON.
func (env Env) post(url string, data any) (response, error) {
	return env.send(http.MethodPost, url, "", data)
}

// send sends a request that accepts JSON, with the bearer token if any and
// data as a JSON body if any.
func (env Env) send(method, url, token string, data any) (response, error) {
	var body io.Reader
	if data != nil {
		payload, err := json.Marshal(data)
		if err != nil {
			return response{}, err
		}
		body = bytes.NewReader(payload)
	}
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return response{}, err
	}
	req.Header.Set("Accept", "application/json")
	if data != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := env.HTTP.Do(req)
	if err != nil {
		return response{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	payload, err := io.ReadAll(resp.Body)
	return response{resp.StatusCode, payload}, err
}
