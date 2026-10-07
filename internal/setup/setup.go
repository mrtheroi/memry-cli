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
	"github.com/mrtheroi/memry-cli/internal/console"
	"github.com/mrtheroi/memry-cli/internal/email"
	"github.com/mrtheroi/memry-cli/internal/flags"
	"github.com/mrtheroi/memry-cli/internal/prompt"
)

// Prompter asks the questions setup needs.
type Prompter interface {
	// Ask returns the trimmed answer, empty when there is none, or an
	// error when the input ended before an answer.
	Ask(label string) (string, error)
	// Secret returns the trimmed answer, typed without being shown.
	Secret(label string) string
	// MultiSelect returns the values of the choices picked, starting
	// from defaults, or an error when the question was cancelled.
	MultiSelect(label, hint string, choices []prompt.Choice, defaults []string) ([]string, error)
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
	if message, ok := flags.UnexpectedArgument("setup", args.Rest); ok {
		if !flags.IsSilent(args, env.LookupEnv) {
			console.ErrorBlock(env.Out, message)
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
// other keys of the config, revokes the token of the previous login, then
// wires memry into the selected agents.
func (env Env) saveLogin(plan flags.Plan, token, loggedIn string) int {
	// The token of an email login was issued for this setup: unless it is
	// saved, no one could use or revoke it. A given token is the user's.
	unsaved := func() int {
		if plan.Login == flags.LoginEmail {
			env.revokeNewToken(plan.URL, token)
		}
		return failure
	}
	path := config.Path(env.getenv)
	cfg, err := config.LoadOrEmpty(path)
	if err != nil {
		env.notSaved(path, err)
		return unsaved()
	}
	previousURL, hasURL := cfg.URL()
	previousToken, hasToken := cfg.Token()
	saved, hasSaved := cfg.Agents()
	toRemove, hasToRemove := cfg.AgentsToRemove()
	selected, err := env.selectAgents(plan, saved, hasSaved)
	if err != nil {
		return unsaved()
	}
	cfg.Set("url", plan.URL)
	cfg.Set("token", token)
	cfg.Set("agents", selected)
	// An agent selected again is installed, no longer removed.
	toRemove = slices.DeleteFunc(slices.Clone(toRemove), func(key string) bool { return slices.Contains(selected, key) })
	setAgentsToRemove(cfg, toRemove)
	if err := cfg.Save(); err != nil {
		env.notSaved(path, err)
		return unsaved()
	}

	env.line(loggedIn + " Credentials saved to " + path + ".")
	if hasURL && hasToken && previousToken != token {
		env.revokePreviousToken(previousURL, previousToken)
	}
	if len(selected) == 0 {
		env.line("No agents selected; memry is not wired into any agent. Run `memry setup` again to choose some.")
	}
	wired := saved
	if hasURL && hasToken && !hasSaved && !hasToRemove {
		// Setup saved no agents up to 0.4.0, when it only wired Claude
		// Code: unwire it unless it is selected, like uninstall does.
		wired = []string{"claude-code"}
	}
	// The saved agents no longer selected are removed, and the removals
	// that failed before are retried.
	code, pending := env.wire(plan.URL, selected, append(slices.Clone(wired), toRemove...))
	if len(toRemove) > 0 || len(pending) > 0 {
		// Keep the agents memry could not be removed from, so a later
		// setup or uninstall retries the removal.
		setAgentsToRemove(cfg, pending)
		if err := cfg.Save(); err != nil {
			return env.notSaved(path, err)
		}
	}
	return code
}

// setAgentsToRemove saves the agents memry could not be removed from in
// agents_to_remove, or removes the key when there are none, so the file
// is then the one the PHP CLI writes.
func setAgentsToRemove(cfg *config.File, keys []string) {
	if len(keys) == 0 {
		cfg.Delete("agents_to_remove")
		return
	}
	cfg.Set("agents_to_remove", keys)
}

// selectAgents returns the agents given with --agents, else asks which
// agents to wire, defaulting to the saved selection without the agents
// this version does not support, else to the installed ones. The default
// is the answer when not interactive.
func (env Env) selectAgents(plan flags.Plan, saved []string, hasSaved bool) ([]string, error) {
	if plan.AgentsGiven {
		return unique(plan.Agents), nil
	}
	var choices []prompt.Choice
	installed := []string{}
	for _, agent := range env.Agents.All() {
		if agent.IsInstalled() {
			choices = append(choices, prompt.Choice{Value: agent.Key(), Label: agent.Name()})
			installed = append(installed, agent.Key())
		} else {
			choices = append(choices, prompt.Choice{Value: agent.Key(), Label: agent.Name() + " (not installed)"})
		}
	}
	defaults := installed
	if hasSaved {
		// The saved order, each agent once (PHP keeps duplicates).
		defaults = []string{}
		for _, key := range unique(saved) {
			if slices.Contains(env.Agents.Keys(), key) {
				defaults = append(defaults, key)
			}
		}
	}
	if !plan.Interactive {
		return defaults, nil
	}
	return env.Prompter.MultiSelect("Which agents do you use?", "Space to select, enter to confirm.", choices, defaults)
}

// unique returns keys without repeats, each at its first place.
func unique(keys []string) []string {
	once := []string{}
	for _, key := range keys {
		if !slices.Contains(once, key) {
			once = append(once, key)
		}
	}
	return once
}

// wire installs memry in the selected agents and removes it from the
// saved ones no longer selected. Every agent is handled, even when an
// earlier one fails; a summary line per agent ends the output. It returns
// the exit code and the deselected agents memry could not be removed
// from.
func (env Env) wire(url string, selected, saved []string) (int, []string) {
	var summary []agents.Line
	failed := false
	for _, agent := range env.Agents.Only(selected) {
		if env.report(agent.Install(url)) {
			summary = append(summary, agents.Line{Style: "info", Text: agent.Name() + ": memry is set up."})
		} else {
			failed = true
			summary = append(summary, agents.Line{Style: "error", Text: agent.Name() + ": failed; see the messages above."})
		}
	}
	var deselected []string
	for _, key := range saved {
		if !slices.Contains(selected, key) {
			deselected = append(deselected, key)
		}
	}
	var pending []string
	for _, agent := range env.Agents.Only(deselected) {
		if env.report(agent.Uninstall()) {
			summary = append(summary, agents.Line{Style: "info", Text: agent.Name() + ": memry was removed."})
		} else {
			failed = true
			pending = append(pending, agent.Key())
			summary = append(summary, agents.Line{Style: "error", Text: agent.Name() + ": failed; see the messages above."})
		}
	}
	env.line("")
	for _, line := range summary {
		env.line(line.Text)
	}
	if failed {
		return failure, pending
	}
	return success, pending
}

// report prints the lines of an agent result and returns whether it
// succeeded.
func (env Env) report(result agents.Result) bool {
	for _, line := range result.Lines {
		env.line(line.Text)
	}
	return result.Successful
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
	console.ErrorBlock(env.Out, "Aborted.")
	return false
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

// revokeNewToken revokes the token setup was just issued, when it stops
// without saving it. A 401 means it is no longer valid: as good as
// revoked.
func (env Env) revokeNewToken(url, token string) {
	resp, err := env.send(http.MethodDelete, url+"/api/auth/token", token, nil)
	if err == nil && (resp.succeeded() || resp.status == http.StatusUnauthorized) {
		env.line("Revoked the new memry token.")
	} else {
		env.line("Could not revoke the new memry token.")
	}
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
