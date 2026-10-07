// Package uninstall is `memry uninstall` and `memry delete-account`: they
// remove what `memry setup` left on this machine.
package uninstall

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"

	"github.com/mrtheroi/memry-cli/internal/agents"
	"github.com/mrtheroi/memry-cli/internal/client"
	"github.com/mrtheroi/memry-cli/internal/config"
	"github.com/mrtheroi/memry-cli/internal/console"
	"github.com/mrtheroi/memry-cli/internal/email"
	"github.com/mrtheroi/memry-cli/internal/flags"
)

// Prompter asks the questions the commands need.
type Prompter interface {
	// Ask returns the trimmed answer, or an error when the input ended
	// before an answer.
	Ask(label string) (string, error)
	// Confirm asks a yes/no question whose default is no.
	Confirm(label string) (bool, error)
}

// Env is what the commands run with.
type Env struct {
	// Args are the arguments after the command name.
	Args      []string
	LookupEnv func(string) (string, bool)
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

// Uninstall runs `memry uninstall` and returns its exit code.
func Uninstall(env Env) int {
	interactive, ok := env.start("uninstall", "--force")
	if !ok {
		return failure
	}
	if !flags.HasParameterOption(env.Args, "--force") {
		confirmed := false
		if interactive {
			var err error
			if confirmed, err = env.Prompter.Confirm("Remove memry from your agents and delete your login?"); err != nil {
				return env.aborted()
			}
		}
		if !confirmed {
			env.line("Aborted; nothing was removed.")
			return failure
		}
	}
	// Each step runs even when an earlier one fails.
	revoked := env.revokeToken()
	removed := env.removeLocalInstall()
	env.line("Run `brew uninstall memry` to remove the CLI.")
	if revoked && removed {
		return success
	}
	return failure
}

// start checks the arguments of command, which takes only the global
// options and its own value-less options, like Symfony: on an error it
// says why (unless silent) and reports false. Quiet hides the output from
// then on. It reports whether questions can be asked: not with
// --no-interaction, nor when quiet.
func (env *Env) start(command string, options ...string) (interactive, ok bool) {
	args := flags.Scan(env.Args)
	if message, bad := flags.UnexpectedArgument(command, env.Args, options...); bad {
		if !flags.IsSilent(args, env.LookupEnv) {
			console.ErrorBlock(env.Out, message)
		}
		return false, false
	}
	quiet := flags.IsQuiet(args, env.LookupEnv)
	if quiet {
		env.Out = io.Discard
	}
	return !args.NoInteraction && !quiet, true
}

// aborted fails like Symfony when the input ends before an answer.
func (env Env) aborted() int {
	console.ErrorBlock(env.Out, "Aborted.")
	return failure
}

// removeLocalInstall removes what `memry setup` left on this machine:
// memry in every agent it wired, and the config file with the login.
// Every step runs, even when an earlier one fails; it reports whether all
// of them succeeded.
func (env Env) removeLocalInstall() bool {
	saved, hasSaved := env.loadConfig().Agents()
	if !hasSaved {
		// Setup saves no agents up to 0.4.0, when it only wired Claude Code.
		saved = []string{"claude-code"}
	}
	removed := true
	for _, agent := range env.Agents.Only(saved) {
		removed = env.report(agent.Uninstall()) && removed
	}
	return env.deleteConfig() && removed
}

// report prints the lines of an agent result and returns whether it
// succeeded.
func (env Env) report(result agents.Result) bool {
	for _, line := range result.Lines {
		env.line(line.Text)
	}
	return result.Successful
}

// deleteConfig deletes the config file, and with it the login.
func (env Env) deleteConfig() bool {
	path := config.Path(env.getenv)
	if info, err := os.Stat(path); err != nil || !info.Mode().IsRegular() {
		env.line("No config file to delete.")
		return true
	}
	if err := os.Remove(path); err != nil {
		env.line("Could not delete " + path + ".")
		return false
	}
	env.line("Deleted " + path + ".")
	return true
}

// revokeToken revokes the saved token on the server it belongs to,
// reporting whether it is no longer valid. A token the server no longer
// accepts is as good as revoked.
func (env Env) revokeToken() bool {
	cfg := env.loadConfig()
	url, hasURL := cfg.URL()
	token, hasToken := cfg.Token()
	if !hasURL || !hasToken {
		env.line("Not logged in; no token to revoke.")
		return true
	}
	switch env.revoke(url, token) {
	case revoked:
		env.line("Revoked the memry token.")
	case alreadyRevoked:
		env.line("The memry token was already revoked.")
	default:
		env.line("Could not revoke the memry token.")
		return false
	}
	return true
}

// revokeResult is how revoking a token went.
type revokeResult int

const (
	revoked revokeResult = iota
	// alreadyRevoked: the server answered 401, the token was no longer
	// valid.
	alreadyRevoked
	// revokeFailed: any other answer, or the server could not be reached.
	revokeFailed
)

// revoke revokes token on the memry server at url.
func (env Env) revoke(url, token string) revokeResult {
	status, err := env.send(http.MethodDelete, url+"/api/auth/token", token, nil)
	switch {
	case err != nil:
		return revokeFailed
	case client.Succeeded(&http.Response{StatusCode: status}):
		return revoked
	case status == http.StatusUnauthorized:
		return alreadyRevoked
	}
	return revokeFailed
}

// send sends a request that accepts JSON with the bearer token, and data
// as a JSON body if any, returning the response status.
func (env Env) send(method, url, token string, data any) (int, error) {
	var body io.Reader
	if data != nil {
		payload, err := json.Marshal(data)
		if err != nil {
			return 0, err
		}
		body = bytes.NewReader(payload)
	}
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Accept", "application/json")
	if data != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := env.HTTP.Do(req)
	if err != nil {
		return 0, err
	}
	_ = resp.Body.Close()
	return resp.StatusCode, nil
}

// loadConfig reads the config file. Like the PHP CLI, a file that is
// missing or cannot be read as a JSON object is an empty config.
func (env Env) loadConfig() *config.File {
	path := config.Path(env.getenv)
	cfg, err := config.LoadOrEmpty(path)
	if err != nil {
		cfg = &config.File{}
	}
	return cfg
}

// line prints one line of output.
func (env Env) line(text string) {
	_, _ = io.WriteString(env.Out, text+"\n")
}

func (env Env) getenv(key string) string {
	value, _ := env.LookupEnv(key)
	return value
}

// DeleteAccount runs `memry delete-account` and returns its exit code.
func DeleteAccount(env Env) int {
	interactive, ok := env.start("delete-account")
	if !ok {
		return failure
	}
	cfg := env.loadConfig()
	url, hasURL := cfg.URL()
	token, hasToken := cfg.Token()
	if !hasURL || !hasToken {
		env.line("You are not logged in to memry.")
		return failure
	}
	env.line("This permanently deletes your memry account and ALL its memories on the server. It cannot be undone.")
	address, err := env.askEmail(interactive)
	if err != nil {
		return env.aborted()
	}
	if address == "" {
		env.line("Aborted; nothing was deleted.")
		return failure
	}
	if message := env.deleteAccount(url, token, address); message != "" {
		env.line(message)
		return failure
	}
	env.line("Deleted your memry account and all its memories.")
	// The token was deleted with the account, so there is nothing to revoke.
	removed := env.removeLocalInstall()
	env.line("Run `brew uninstall memry` to remove the CLI.")
	if removed {
		return success
	}
	return failure
}

// askEmail asks for the account email until it is a valid address. It is
// empty when the answer is left empty, as it is without interaction.
func (env Env) askEmail(interactive bool) (string, error) {
	if !interactive {
		return "", nil
	}
	for {
		answer, err := env.Prompter.Ask("Type your account email to confirm")
		if err != nil || answer == "" {
			return "", err
		}
		if address, ok := email.Normalize(answer); ok {
			return address, nil
		}
		env.line("Enter a valid email address.")
	}
}

// deleteAccount deletes the account on the server, returning why it
// failed, or nothing once it is deleted.
func (env Env) deleteAccount(url, token, address string) string {
	status, err := env.send(http.MethodDelete, url+"/api/account", token, map[string]string{"email": address})
	switch {
	case err != nil:
	case client.Succeeded(&http.Response{StatusCode: status}):
		return ""
	case status == http.StatusUnprocessableEntity:
		return "The email does not match your memry account."
	case status == http.StatusUnauthorized:
		return "Your memry login is no longer valid. Run `memry setup` and try again."
	}
	return "Could not delete your memry account. Try again later."
}
