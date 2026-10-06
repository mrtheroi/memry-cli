package flags_test

import (
	"errors"
	"reflect"
	"testing"

	"github.com/mrtheroi/memry-cli/internal/flags"
)

var agentKeys = []string{"claude-code", "codex", "opencode", "antigravity", "windsurf"}

func resolve(argv []string, env map[string]string) (flags.Plan, error) {
	lookupEnv := func(key string) (string, bool) {
		value, ok := env[key]
		return value, ok
	}
	return flags.Resolve(flags.Scan(argv), lookupEnv, agentKeys)
}

func assertError(t *testing.T, argv []string, env map[string]string, message string) {
	t.Helper()
	plan, err := resolve(argv, env)
	var usage *flags.Error
	if !errors.As(err, &usage) {
		t.Fatalf("Resolve(%q) = %+v, %v; want the error %q", argv, plan, err, message)
	}
	if usage.Message != message || usage.ExitCode != 1 {
		t.Errorf("Resolve(%q) error = %q (exit %d), want %q (exit 1)", argv, usage.Message, usage.ExitCode, message)
	}
}

// Ported from "fails before logging in when --agents has an unknown key".
func TestResolveRejectsAnUnknownAgentFirst(t *testing.T) {
	want := `Unknown agent "cursor". Valid agents: claude-code, codex, opencode, antigravity, windsurf.`
	for _, argv := range [][]string{
		{"--url", "https://memry.test", "--email", "ana@example.com", "--agents", "claude-code,cursor"},
		{"--agents=cursor,nope", "--email", "--token"},
	} {
		assertError(t, argv, nil, want)
	}
}

// Ported from "fails without sending anything when both --email and --token
// are given", with or without values, before a valueless option is reported.
func TestResolveRejectsEmailWithToken(t *testing.T) {
	for _, argv := range [][]string{
		{"--url", "https://memry.test", "--email", "ana@example.com", "--token", "admin-token"},
		{"--url=https://memry.test", "--email=ana@example.com", "--token"},
		{"--url", "https://memry.test", "--email", "--token=admin-token"},
		{"--url", "--email", "--token"},
	} {
		assertError(t, argv, nil, "Use either --email or --token, not both.")
	}
}

// Ported from "fails without asking or sending anything when --url
// (--email, --agents) is given without a value"; they are checked in that
// order.
func TestResolveRejectsAValuelessOption(t *testing.T) {
	tests := []struct {
		argv []string
		want string
	}{
		{[]string{"--url", "--email=ana@example.com", "--agents=claude-code"}, "The --url option needs a value."},
		{[]string{"--url", "--token=admin-token", "--agents=claude-code"}, "The --url option needs a value."},
		{[]string{"--agents", "--email", "--url"}, "The --url option needs a value."},
		{[]string{"--url", "https://memry.test", "--email", "--agents=claude-code"}, "The --email option needs a value."},
		{[]string{"--agents", "--email"}, "The --email option needs a value."},
		{[]string{"--url", "https://memry.test", "--email=ana@example.com", "--agents"}, "The --agents option needs a value."},
	}
	for _, tt := range tests {
		assertError(t, tt.argv, nil, tt.want)
	}
}

// Ported from "fails without asking or sending anything when the --url
// option is not an http(s) address".
func TestResolveRejectsAURLThatIsNotAnHTTPAddress(t *testing.T) {
	urls := []string{
		"", "   ", "memry.test", "ftp://memry.test", "https://memry .test", "https://",
		"https://memry.test?team=1", "https://memry.test#top", "https://memry.test?", "https://memry.test#",
		"https://memry.test/\tx", "mailto:ana@example.com", "https:memry.test",
	}
	logins := [][]string{{"--email=ana@example.com"}, {"--token=admin-token"}}
	for _, url := range urls {
		for _, login := range logins {
			assertError(t, append([]string{"--url=" + url}, login...), nil,
				"Invalid server address given with --url. Use an http:// or https:// URL.")
		}
	}
}

// Ported from "accepts a plain http server url for self-hosted servers" and
// "strips a trailing slash from the server url".
func TestResolveAcceptsAnHTTPServerURLWithoutItsTrailingSlash(t *testing.T) {
	tests := map[string]string{
		"http://localhost:8000":                "http://localhost:8000",
		"http://192.168.1.10/":                 "http://192.168.1.10",
		"http://memry_server":                  "http://memry_server",
		"https://tools.company.internal/memry": "https://tools.company.internal/memry",
		"https://memry.test//":                 "https://memry.test",
		"HTTPS://memry.test":                   "HTTPS://memry.test",
	}
	for given, want := range tests {
		for _, argv := range [][]string{{"--url=" + given, "--token=admin-token"}, {"--url", given, "--email", "ana@example.com"}} {
			plan, err := resolve(argv, nil)
			if err != nil {
				t.Fatalf("Resolve(%q) error = %v", argv, err)
			}
			if plan.URL != want {
				t.Errorf("Resolve(%q).URL = %q, want %q", argv, plan.URL, want)
			}
		}
	}
}

// Ported from "uses the configured server url when no --url option is
// given" and "uses the memry production server by default".
func TestResolveDefaultsTheURLToMemryURLOrTheProductionServer(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
		want string
	}{
		{"production", nil, "https://api.memry.com.mx"},
		{"MEMRY_URL", map[string]string{"MEMRY_URL": "https://configured.memry.test/"}, "https://configured.memry.test"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan, err := resolve([]string{"--email=ana@example.com"}, tt.env)
			if err != nil {
				t.Fatalf("Resolve error = %v", err)
			}
			if plan.URL != tt.want {
				t.Errorf("URL = %q, want %q", plan.URL, tt.want)
			}
		})
	}
}

// Ported from "fails without asking or sending anything when --token is
// given without --url" and "still requires --url when MEMRY_TOKEN is set":
// an admin token never goes to the default server.
func TestResolveRequiresURLWithToken(t *testing.T) {
	envs := []map[string]string{nil, {"MEMRY_TOKEN": "env-token"}, {"MEMRY_URL": "https://configured.memry.test"}}
	for _, argv := range [][]string{{"--token=admin-token"}, {"--token", "admin-token"}, {"--token"}, {"--token", "-n"}} {
		for _, env := range envs {
			assertError(t, argv, env, "Pass --url with --token, the address of your memry server.")
		}
	}
}

// Ported from the MEMRY_TOKEN and --token tests of SetupTokenTest.php: the
// token comes from --token=<value>, then a non-blank MEMRY_TOKEN, then the
// prompt, trimmed.
func TestResolveTakesTheTokenFromTheFlagThenTheEnvironmentThenThePrompt(t *testing.T) {
	url := []string{"--url", "https://memry.test"}
	tests := []struct {
		name       string
		argv       []string
		env        map[string]string
		wantLogin  flags.Login
		wantToken  string
		wantSource flags.TokenSource
	}{
		{"a --token value", []string{"--token=admin-token"}, nil, flags.LoginToken, "admin-token", flags.TokenFromFlag},
		{"a trimmed --token value", []string{"--token", "  admin-token \n"}, nil, flags.LoginToken, "admin-token", flags.TokenFromFlag},
		{"a --token value over MEMRY_TOKEN", []string{"--token", "admin-token"}, map[string]string{"MEMRY_TOKEN": "env-token"}, flags.LoginToken, "admin-token", flags.TokenFromFlag},
		{"MEMRY_TOKEN", []string{"--token"}, map[string]string{"MEMRY_TOKEN": "env-token"}, flags.LoginToken, "env-token", flags.TokenFromEnv},
		{"MEMRY_TOKEN without interaction", []string{"--token", "-n"}, map[string]string{"MEMRY_TOKEN": "env-token"}, flags.LoginToken, "env-token", flags.TokenFromEnv},
		{"a trimmed MEMRY_TOKEN", []string{"--token"}, map[string]string{"MEMRY_TOKEN": "  env-token \n"}, flags.LoginToken, "env-token", flags.TokenFromEnv},
		{`a MEMRY_TOKEN of "0"`, []string{"--token", "--no-interaction"}, map[string]string{"MEMRY_TOKEN": "0"}, flags.LoginToken, "0", flags.TokenFromEnv},
		{"the prompt", []string{"--token"}, nil, flags.LoginToken, "", flags.TokenFromPrompt},
		{"the prompt over a blank MEMRY_TOKEN", []string{"--token"}, map[string]string{"MEMRY_TOKEN": "  \n"}, flags.LoginToken, "", flags.TokenFromPrompt},
		{"no token without --token, even with MEMRY_TOKEN", []string{"--email=ana@example.com"}, map[string]string{"MEMRY_TOKEN": "env-token"}, flags.LoginEmail, "", flags.NoToken},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan, err := resolve(append(url, tt.argv...), tt.env)
			if err != nil {
				t.Fatalf("Resolve error = %v", err)
			}
			if plan.Login != tt.wantLogin || plan.Token != tt.wantToken || plan.TokenSource != tt.wantSource {
				t.Errorf("Login, Token, TokenSource = %v, %q, %v; want %v, %q, %v",
					plan.Login, plan.Token, plan.TokenSource, tt.wantLogin, tt.wantToken, tt.wantSource)
			}
		})
	}
}

// Ported from "fails without sending anything when --token has no value and
// there is no interaction" and "ignores a blank MEMRY_TOKEN environment
// variable".
func TestResolveRejectsAPromptWithoutInteraction(t *testing.T) {
	for _, env := range []map[string]string{nil, {"MEMRY_TOKEN": "  \n"}} {
		for _, argv := range [][]string{{"--url=https://memry.test", "--token", "--no-interaction"}, {"-n", "--url", "https://memry.test", "--token"}} {
			assertError(t, argv, env, "Pass --token=<value> when running without interaction.")
		}
	}
}

// Ported from "fails without sending anything when the --token option is
// empty".
func TestResolveRejectsAnEmptyTokenValue(t *testing.T) {
	for _, argv := range [][]string{
		{"--url=https://memry.test", "--token="},
		{"--url=https://memry.test", "--token", ""},
		{"--url=https://memry.test", "--token=   "},
		{"--url=https://memry.test", "--token=", "-n"},
	} {
		assertError(t, argv, map[string]string{"MEMRY_TOKEN": "env-token"}, "The token is empty.")
	}
}

// Ported from "fails without sending anything when the entered token is
// empty" and "trims the token before checking and saving it".
func TestCheckTokenTrimsThePromptedToken(t *testing.T) {
	for answer, want := range map[string]string{"admin-token": "admin-token", "  admin-token \n": "admin-token", "0": "0"} {
		if got, err := flags.CheckToken(answer); got != want || err != nil {
			t.Errorf("CheckToken(%q) = %q, %v; want %q, nil", answer, got, err, want)
		}
	}
	for _, answer := range []string{"", "   ", "\t\n"} {
		_, err := flags.CheckToken(answer)
		var usage *flags.Error
		if !errors.As(err, &usage) || usage.Message != "The token is empty." || usage.ExitCode != 1 {
			t.Errorf("CheckToken(%q) error = %v, want \"The token is empty.\" (exit 1)", answer, err)
		}
	}
}

// Ported from "wires only the agents given with --agents and saves them" and
// the --agents= case of "warns but keeps the login when no agent is
// selected": an empty --agents= selects no agent.
func TestResolveCarriesTheEmailAgentsAndInteraction(t *testing.T) {
	tests := []struct {
		name string
		argv []string
		want flags.Plan
	}{
		{
			"an email login with agents",
			[]string{"--email", "ana@example.com", "--agents", " claude-code , ,codex"},
			flags.Plan{URL: flags.DefaultURL, Email: "ana@example.com", EmailGiven: true, Agents: []string{"claude-code", "codex"}, AgentsGiven: true, Interactive: true},
		},
		{
			"no agents",
			[]string{"--agents=", "-n"},
			flags.Plan{URL: flags.DefaultURL, Agents: []string{}, AgentsGiven: true},
		},
		{
			"agents to ask for",
			[]string{"--url=https://memry.test", "--token=admin-token"},
			flags.Plan{URL: "https://memry.test", Login: flags.LoginToken, Token: "admin-token", TokenSource: flags.TokenFromFlag, Interactive: true},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan, err := resolve(tt.argv, nil)
			if err != nil {
				t.Fatalf("Resolve error = %v", err)
			}
			if !reflect.DeepEqual(plan, tt.want) {
				t.Errorf("Resolve(%q) = %+v, want %+v", tt.argv, plan, tt.want)
			}
		})
	}
}
