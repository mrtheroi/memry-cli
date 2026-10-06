package flags

import (
	"math"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// DefaultURL is the memry server setup uses without --url or MEMRY_URL.
const DefaultURL = "https://api.memry.com.mx"

// Error is a setup option error: the message to show and the exit code.
type Error struct {
	Message  string
	ExitCode int
}

func (e *Error) Error() string {
	return e.Message
}

// usageError is an option error; each one exits with code 1, as in PHP.
func usageError(message string) *Error {
	return &Error{Message: message, ExitCode: 1}
}

func fail(message string) (Plan, error) {
	return Plan{}, usageError(message)
}

// Plan is what setup does, resolved from its options.
type Plan struct {
	// URL is the memry server, without a trailing slash.
	URL string
	// Login is how setup logs in.
	Login Login
	// Token is the trimmed, non-empty token of a token login, unless it
	// comes from the prompt.
	Token       string
	TokenSource TokenSource
	// Email is the --email value of an email login, as given.
	Email      string
	EmailGiven bool
	// Agents are the keys given with --agents, which may be none.
	Agents      []string
	AgentsGiven bool
	// Interactive is false with --no-interaction, or when Quiet.
	Interactive bool
	// Quiet hides every line setup prints (see IsQuiet).
	Quiet bool
}

// Login is how setup logs in.
type Login int

const (
	// LoginEmail logs in with an email one-time code.
	LoginEmail Login = iota
	// LoginToken logs in with a token created by the server admin.
	LoginToken
)

// TokenSource is where the token of a token login comes from.
type TokenSource int

const (
	// NoToken: an email login has no token to read.
	NoToken TokenSource = iota
	// TokenFromFlag: --token=<value>.
	TokenFromFlag
	// TokenFromEnv: MEMRY_TOKEN, when --token has no value.
	TokenFromEnv
	// TokenFromPrompt: a hidden prompt, which setup asks and checks with
	// CheckToken.
	TokenFromPrompt
)

// Resolve checks the setup options and turns them into a plan, in the
// order of the PHP CLI, before anything is asked or sent.
func Resolve(args Args, lookupEnv func(string) (string, bool), agentKeys []string) (Plan, error) {
	var agents []string
	if args.Agents.HasValue {
		agents = splitKeys(args.Agents.Value)
		for _, key := range agents {
			if !slices.Contains(agentKeys, key) {
				return fail(`Unknown agent "` + key + `". Valid agents: ` + strings.Join(agentKeys, ", ") + ".")
			}
		}
	}
	if args.Email.Present && args.Token.Present {
		return fail("Use either --email or --token, not both.")
	}
	// A valueless --token is fine: it means asking for the token.
	for _, option := range []struct {
		name string
		Option
	}{{"url", args.URL}, {"email", args.Email}, {"agents", args.Agents}} {
		if option.Present && !option.HasValue {
			return fail("The --" + option.name + " option needs a value.")
		}
	}
	if args.URL.HasValue && !isServerURL(args.URL.Value) {
		return fail("Invalid server address given with --url. Use an http:// or https:// URL.")
	}
	plan := Plan{
		URL:         serverURL(args.URL, lookupEnv),
		Email:       args.Email.Value,
		EmailGiven:  args.Email.HasValue,
		Agents:      agents,
		AgentsGiven: args.Agents.HasValue,
	}
	plan.Quiet = IsQuiet(args, lookupEnv)
	plan.Interactive = !args.NoInteraction && !plan.Quiet
	if !args.Token.Present {
		return plan, nil
	}
	if !args.URL.Present {
		return fail("Pass --url with --token, the address of your memry server.")
	}
	token, source, err := tokenSource(args, plan.Interactive, lookupEnv)
	if err != nil {
		return Plan{}, err
	}
	plan.Login, plan.Token, plan.TokenSource = LoginToken, token, source
	return plan, nil
}

// IsQuiet reports whether the verbosity is quiet or silent, which hides
// every line setup prints, errors included: -q, --quiet or --silent, else
// a quiet SHELL_VERBOSITY unless a -v option is given. Setup needs it even
// when Resolve fails.
func IsQuiet(args Args, lookupEnv func(string) (string, bool)) bool {
	return args.Quiet || (!args.Verbose && isQuietVerbosity(lookupEnv))
}

// isQuietVerbosity reports whether SHELL_VERBOSITY is quiet (-1) or silent
// (-2), read like PHP's (int) cast. Symfony takes any other value as the
// normal verbosity.
func isQuietVerbosity(lookupEnv func(string) (string, bool)) bool {
	value, _ := lookupEnv("SHELL_VERBOSITY")
	n := phpInt(value)
	return n == -1 || n == -2
}

// leadingNumber is the numeric prefix PHP's (int) cast reads.
var leadingNumber = regexp.MustCompile(`^[ \t\n\r\v\f]*[+-]?(\d+(\.\d*)?|\.\d+)([eE][+-]?\d+)?`)

// phpInt converts s like PHP's (int) cast: its leading number, truncated,
// or 0.
func phpInt(s string) int {
	number, err := strconv.ParseFloat(strings.TrimLeft(leadingNumber.FindString(s), " \t\n\r\v\f"), 64)
	if err != nil || math.IsInf(number, 0) {
		return 0
	}
	return int(number)
}

// serverURL is the --url value, else MEMRY_URL, else the production
// server, without a trailing slash.
func serverURL(option Option, lookupEnv func(string) (string, bool)) string {
	value := option.Value
	if !option.HasValue {
		value = DefaultURL
		if fromEnv, ok := lookupEnv("MEMRY_URL"); ok {
			value = fromEnv
		}
	}
	return strings.TrimRight(value, "/")
}

// tokenSource finds the token of a token login: --token=<value>, else a
// non-blank MEMRY_TOKEN, else the prompt when there is interaction.
// MEMRY_TOKEN is only read here, once --token is given; setup must then
// remove it from the environment its subprocesses inherit.
func tokenSource(args Args, interactive bool, lookupEnv func(string) (string, bool)) (string, TokenSource, error) {
	fromEnv, _ := lookupEnv("MEMRY_TOKEN")
	switch {
	case args.Token.HasValue:
		token, err := CheckToken(args.Token.Value)
		return token, TokenFromFlag, err
	case phpTrim(fromEnv) != "":
		return phpTrim(fromEnv), TokenFromEnv, nil
	case !interactive:
		return "", NoToken, usageError("Pass --token=<value> when running without interaction.")
	default:
		return "", TokenFromPrompt, nil
	}
}

// isServerURL reports whether url is an http(s) address that API paths can
// be appended to: a host, a valid port if any, no whitespace, no query and
// no fragment.
func isServerURL(raw string) bool {
	if strings.ContainsAny(raw, " \t\n\v\f\r?#") {
		return false
	}
	parsed, err := url.Parse(raw)
	return err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Hostname() != "" &&
		isPort(parsed.Port())
}

// isPort reports whether port is empty or a port PHP's parse_url accepts:
// at most five digits (url.Parse has checked they are digits), and here
// also within 1-65535.
func isPort(port string) bool {
	if port == "" {
		return true
	}
	n, err := strconv.Atoi(port)
	return err == nil && len(port) <= 5 && n >= 1 && n <= 65535
}

// splitKeys returns the agent keys of a comma-separated list, trimmed,
// without empty ones.
func splitKeys(list string) []string {
	keys := []string{}
	for _, key := range strings.Split(list, ",") {
		if key = phpTrim(key); key != "" {
			keys = append(keys, key)
		}
	}
	return keys
}

// phpTrim trims the characters PHP's trim() does, and only those.
func phpTrim(s string) string {
	return strings.Trim(s, " \t\n\r\x00\x0B")
}

// CheckToken trims a given or prompted token and rejects an empty one.
func CheckToken(token string) (string, error) {
	if token = phpTrim(token); token == "" {
		return "", usageError("The token is empty.")
	}
	return token, nil
}
