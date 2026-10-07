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
	"slices"
	"strings"
	"unicode/utf8"

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
	args := flags.Scan(env.Args)
	// Like Symfony, quiet hides messages but not these errors; only silent
	// hides them too. A malformed token such as -qfoo is never silent.
	if message, ok := unexpectedArgument(args.Rest); ok {
		if !flags.IsSilent(args, env.LookupEnv) {
			env.errorBlock(message)
		}
		return failure
	}
	if flags.IsQuiet(args, env.LookupEnv) {
		env.Out = io.Discard
	}
	plan, err := flags.Resolve(args, env.LookupEnv, env.Agents.Keys())
	if args.Token.Present {
		// Resolve has read the token; keep it away from every subprocess
		// from now on, whether this login succeeds or not. Unsetenv only
		// fails on a malformed name, which this is not.
		_ = env.Unsetenv("MEMRY_TOKEN")
	}
	var usage *flags.Error
	if errors.As(err, &usage) {
		env.line(usage.Message)
		return usage.ExitCode
	}
	login := env.loginWithEmail
	if plan.Login == flags.LoginToken {
		login = env.loginWithToken
	}
	token, loggedIn, ok := login(plan)
	if !ok {
		return failure
	}
	return env.saveLogin(plan, token, loggedIn)
}

// loginWithToken logs in with a token created by the server admin, once
// the server accepts it. Any authenticated endpoint would do; the context
// is a cheap read.
func (env Env) loginWithToken(plan flags.Plan) (token, loggedIn string, ok bool) {
	token = plan.Token
	if plan.TokenSource == flags.TokenFromPrompt {
		var err error
		if token, err = flags.CheckToken(env.Prompter.Secret("Token")); err != nil {
			env.line(err.Error())
			return "", "", false
		}
	}
	resp, err := env.send(http.MethodGet, plan.URL+"/api/context?project=memry", token, nil)
	switch {
	case err != nil:
		env.line("Could not reach the memry server at " + plan.URL + ".")
		return "", "", false
	case resp.status == http.StatusUnauthorized:
		env.line("The token was rejected by " + plan.URL + ".")
		return "", "", false
	case !resp.succeeded():
		env.failWith(resp)
		return "", "", false
	}
	return token, "Connected to " + plan.URL + ".", true
}

// loginWithEmail logs in with an email one-time code, returning the token
// and the confirmation to show, or false (after telling why) when it
// fails.
func (env Env) loginWithEmail(plan flags.Plan) (token, loggedIn string, ok bool) {
	if !plan.EmailGiven && !plan.Interactive {
		env.line("Pass --email when running without interaction.")
		return "", "", false
	}
	var address string
	if plan.EmailGiven {
		if address, ok = email.Normalize(plan.Email); !ok {
			env.line("Invalid email address given with --email.")
			return "", "", false
		}
	} else if a, err := env.askEmail(); err != nil {
		return "", "", env.aborted()
	} else {
		address = a
	}
	if _, ok := env.request(plan.URL, http.MethodPost, "/api/auth/code", "", map[string]string{"email": address}); !ok {
		return "", "", false
	}
	env.line("We sent a login code to " + address + ".")
	code, err := env.askCode(plan.Interactive)
	if err != nil {
		return "", "", env.aborted()
	}
	if code == "" {
		env.line("No login code given. Run `memry setup` interactively to enter the code from the email.")
		return "", "", false
	}
	resp, ok := env.request(plan.URL, http.MethodPost, "/api/auth/token", "", struct {
		Email string `json:"email"`
		Code  string `json:"code"`
	}{address, code})
	if !ok {
		return "", "", false
	}
	if token, ok = resp.jsonString("token"); !ok || token == "" {
		env.line("The memry server did not return a token.")
		return "", "", false
	}
	return token, "Logged in as " + address + ".", true
}

// request sends a request to the memry server at url and returns its
// successful response, or false after telling why it failed.
func (env Env) request(url, method, path, token string, data any) (response, bool) {
	resp, err := env.send(method, url+path, token, data)
	if errors.Is(err, client.ErrTooLarge) {
		env.line("The memry server at " + url + " sent a response that is too large.")
		return response{}, false
	}
	if err != nil {
		env.line("Could not reach the memry server at " + url + ".")
		return response{}, false
	}
	if !resp.succeeded() {
		env.failWith(resp)
		return response{}, false
	}
	return resp, true
}

// saveLogin saves the url, the token and the agent selection, keeping the
// other keys of the config, then revokes the token of the previous login.
func (env Env) saveLogin(plan flags.Plan, token, loggedIn string) int {
	path := config.Path(env.getenv)
	cfg, err := config.LoadOrEmpty(path)
	if err != nil {
		return env.notSaved(path, err)
	}
	previousURL, hasURL := cfg.URL()
	previousToken, hasToken := cfg.Token()
	// The saved agent selection is left as it is until the Go build wires
	// agents (phase 4): the PHP CLI reads this file too and would unwire
	// agents dropped from it.
	cfg.Set("url", plan.URL)
	cfg.Set("token", token)
	if err := cfg.Save(); err != nil {
		return env.notSaved(path, err)
	}

	env.line(loggedIn + " Credentials saved to " + path + ".")
	if hasURL && hasToken && previousToken != token {
		env.revokePreviousToken(previousURL, previousToken)
	}
	env.line("")
	env.line("Agent wiring is not implemented in the Go build yet: no agent was set up or removed, and the saved agent selection was left unchanged.")
	return success
}

// notSaved fails because the config at path could not be saved.
func (env Env) notSaved(path string, err error) int {
	env.line(fmt.Sprintf("Could not save the credentials to %s: %v.", path, err))
	return failure
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

// aborted fails like Symfony when the input ends before an answer.
func (env Env) aborted() bool {
	env.errorBlock("Aborted.")
	return false
}

// errorBlock renders an error the way Symfony does, as a padded block
// (here without its colors).
func (env Env) errorBlock(message string) {
	blank := strings.Repeat(" ", len(message)+4)
	_, _ = io.WriteString(env.Out, "\n"+blank+"\n  "+message+"  \n"+blank+"\n\n")
}

// unexpectedArgument returns the error Symfony gives for the first option
// setup does not know or the first argument (setup takes none), if any.
// --ansi and --no-ansi are global options the PHP CLI accepts.
func unexpectedArgument(rest []string) (string, bool) {
	for i, token := range rest {
		switch {
		case token == "--":
			if i+1 < len(rest) {
				return noArguments(rest[i+1]), true
			}
			return "", false
		case flags.IsGlobalOption(token):
			continue
		case strings.HasPrefix(token, "--"):
			name, _, _ := strings.Cut(token, "=")
			if slices.Contains(flags.GlobalOptions, name) {
				return `The "` + name + `" option does not accept a value.`, true
			}
			return `The "` + name + `" option does not exist.`, true
		case len(token) > 1 && token[0] == '-':
			// Like Symfony, the first character of the set that is not a
			// global shortcut.
			unknown := strings.TrimLeft(token[1:], flags.GlobalShortcuts)
			return `The "-` + unknown[:1] + `" option does not exist.`, true
		default:
			return noArguments(token), true
		}
	}
	return "", false
}

func noArguments(argument string) string {
	return `No arguments expected for "setup" command, got "` + argument + `".`
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
// Laravel's $response->json($key) when it is a string. Like PHP's
// json_decode, a body that is not valid UTF-8 is not JSON (Go's decoder
// would silently replace the bad bytes).
func (r response) jsonString(key string) (string, bool) {
	var body map[string]any
	if !utf8.Valid(r.body) || json.Unmarshal(r.body, &body) != nil {
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
	payload, err := client.ReadBody(resp.Body, maxResponseBytes)
	return response{resp.StatusCode, payload}, err
}

// maxResponseBytes bounds every response setup reads, so a misbehaving
// server cannot make it buffer an unbounded body. Real answers are tiny.
const maxResponseBytes = 1 << 20
