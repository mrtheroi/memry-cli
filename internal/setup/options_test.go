package setup_test

import "testing"

// Ported from the option checks of SetupCommandTest.php, SetupTokenTest.php
// and SetupAgentSelectionTest.php: each fails before asking or sending
// anything, and writes no config.
func TestFailsWithoutAskingOrSendingAnythingOnAnOptionError(t *testing.T) {
	s := newServer(t)
	tests := []struct {
		argv []string
		env  map[string]string
		want string
	}{
		{[]string{"--url", s.URL, "--email", "ana@example.com", "--agents", "claude-code,cursor"}, nil,
			`Unknown agent "cursor". Valid agents: claude-code, codex, opencode, antigravity, windsurf.`},
		{[]string{"--url", s.URL, "--email", "ana@example.com", "--token", "admin-token"}, nil, "Use either --email or --token, not both."},
		{[]string{"--url", s.URL, "--email", "ana@example.com", "--token"}, nil, "Use either --email or --token, not both."},
		{[]string{"--url", s.URL, "--email", "--token", "admin-token"}, nil, "Use either --email or --token, not both."},
		{[]string{"--url", "--email", "ana@example.com"}, nil, "The --url option needs a value."},
		{[]string{"--url", "--token", "admin-token"}, nil, "The --url option needs a value."},
		{[]string{"--url", s.URL, "--email"}, nil, "The --email option needs a value."},
		{[]string{"--url", s.URL, "--email", "ana@example.com", "--agents"}, nil, "The --agents option needs a value."},
		{[]string{"--url", "ftp://memry.test", "--email", "ana@example.com"}, nil, "Invalid server address given with --url. Use an http:// or https:// URL."},
		{[]string{"--url", "https://memry.test?team=1", "--token", "admin-token"}, nil, "Invalid server address given with --url. Use an http:// or https:// URL."},
		{[]string{"--token", "admin-token"}, nil, "Pass --url with --token, the address of your memry server."},
		{[]string{"--token", "-n"}, map[string]string{"MEMRY_TOKEN": "env-token"}, "Pass --url with --token, the address of your memry server."},
		{[]string{"--url", s.URL, "--token", "   "}, nil, "The token is empty."},
		{[]string{"--url", s.URL, "--token", "-n"}, nil, "Pass --token=<value> when running without interaction."},
	}
	for _, tt := range tests {
		h := newHarness(t)
		for key, value := range tt.env {
			h.env[key] = value
		}

		output, code := h.run(tt.argv)

		assertExit(t, code, 1, output)
		if want := tt.want + "\n"; output != want {
			t.Errorf("setup %q output = %q, want %q", tt.argv, output, want)
		}
		h.assertNoConfig()
	}
	if sent := s.received(); len(sent) != 0 {
		t.Errorf("sent %d requests, want none", len(sent))
	}
}

// Symfony's quiet and silent verbosities hide every line setup prints,
// errors included, and keep the exit code.
func TestPrintsNothingWhenQuiet(t *testing.T) {
	s := newServer(t)
	tests := []struct {
		argv []string
		env  map[string]string
		exit int
	}{
		{[]string{"--url", s.URL, "--token=admin-token", "-q"}, nil, 0},
		{[]string{"--url", s.URL, "--token=admin-token", "--silent"}, nil, 0},
		{[]string{"--url", s.URL, "--email", "ana@example.com", "--quiet"}, nil, 1},
		{[]string{"--url", "ftp://memry.test", "--email", "ana@example.com", "-q"}, nil, 1},
		{[]string{"--url", s.URL, "--token=admin-token"}, map[string]string{"SHELL_VERBOSITY": "-1"}, 0},
	}
	for _, tt := range tests {
		h := newHarness(t)
		for key, value := range tt.env {
			h.env[key] = value
		}

		output, code := h.run(tt.argv)

		assertExit(t, code, tt.exit, output)
		if output != "" {
			t.Errorf("setup %q printed %q, want nothing", tt.argv, output)
		}
	}
}
