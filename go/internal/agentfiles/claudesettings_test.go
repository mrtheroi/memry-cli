package agentfiles_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mrtheroi/memry-cli/internal/agentfiles"
	"github.com/mrtheroi/memry-cli/internal/phpjson"
)

// Ported from tests/Unit/ClaudeSettingsTest.php.

const hookMarker = "hook:session-start"

func settingsPath(t *testing.T) string {
	return filepath.Join(t.TempDir(), "settings.json")
}

func hookGroup() *phpjson.Object {
	return phpjson.NewObject("matcher", "startup", "hooks", []any{
		phpjson.NewObject("type", "command", "command", "memry hook:session-start", "timeout", 10),
	})
}

func replaceHook(t *testing.T, path string) bool {
	t.Helper()
	ok, err := agentfiles.NewClaudeSettings(path).ReplaceSessionStartHook(hookGroup(), hookMarker)
	if err != nil {
		t.Fatalf("ReplaceSessionStartHook: %v", err)
	}
	return ok
}

func removeHook(t *testing.T, path string) agentfiles.Removal {
	t.Helper()
	removed, err := agentfiles.NewClaudeSettings(path).RemoveSessionStartHook(hookMarker)
	if err != nil {
		t.Fatalf("RemoveSessionStartHook: %v", err)
	}
	return removed
}

func TestClaudeSettingsKeepsNonASCIICharactersReadableWhenRewriting(t *testing.T) {
	path := settingsPath(t)
	writeFile(t, path, `{"statusLine":{"text":"Diseño ✓"}}`)

	if !replaceHook(t, path) {
		t.Error("ReplaceSessionStartHook = false, want true")
	}

	// Stronger than the PHP test, which only checks the text is still
	// there: what the PHP CLI writes, byte for byte.
	assertContents(t, path, `{
    "statusLine": {
        "text": "Diseño ✓"
    },
    "hooks": {
        "SessionStart": [
            {
                "matcher": "startup",
                "hooks": [
                    {
                        "type": "command",
                        "command": "memry hook:session-start",
                        "timeout": 10
                    }
                ]
            }
        ]
    }
}
`)
}

func TestClaudeSettingsLeavesTheSettingsUntouchedWhenTheHooksHaveAnUnexpectedShape(t *testing.T) {
	tests := map[string]string{
		"hooks is a string":            `{"hooks": "nope"}`,
		"hooks is a list":              `{"hooks": []}`,
		"SessionStart is an object":    `{"hooks": {"SessionStart": {"matcher": "startup"}}}`,
		"SessionStart is a string":     `{"hooks": {"SessionStart": "nope"}}`,
		"a group is not an object":     `{"hooks": {"SessionStart": ["nope"]}}`,
		"a group has non-list hooks":   `{"hooks": {"SessionStart": [{"hooks": "nope"}]}}`,
		"not JSON (from SetupCommand)": `{not json`,
	}
	for name, contents := range tests {
		t.Run(name, func(t *testing.T) {
			path := settingsPath(t)
			writeFile(t, path, contents)

			if replaceHook(t, path) {
				t.Error("ReplaceSessionStartHook = true, want false")
			}
			assertContents(t, path, contents)
		})
	}
}

func TestClaudeSettingsLeavesNoTemporaryFileBehindAfterWriting(t *testing.T) {
	path := settingsPath(t)
	writeFile(t, path, `{}`)

	replaceHook(t, path)

	entries, _ := os.ReadDir(filepath.Dir(path))
	if len(entries) != 1 || entries[0].Name() != "settings.json" {
		t.Errorf("dir entries = %v, want only settings.json", entries)
	}
}

func TestClaudeSettingsRemovesOnlyTheMemryHooksKeepingEveryOtherHookAndSetting(t *testing.T) {
	path := settingsPath(t)
	writeFile(t, path, `{"env":{},"hooks":{"SessionStart":[`+
		`{"matcher":"startup","hooks":[{"type":"command","command":"other-tool start"}]},`+
		`{"matcher":"startup|resume","hooks":[{"type":"command","command":"'/opt/memry/memry' hook:session-start","timeout":10},{"type":"command","command":"shared-group-tool"}]},`+
		`{"matcher":"startup","hooks":[{"type":"command","command":"memry hook:session-start","timeout":10}]}],`+
		`"Stop":[{"hooks":[{"type":"command","command":"on-stop"}]}]}}`)

	if got := removeHook(t, path); got != agentfiles.Removed {
		t.Errorf("RemoveSessionStartHook = %v, want Removed", got)
	}
	assertContents(t, path, `{
    "env": {},
    "hooks": {
        "SessionStart": [
            {
                "matcher": "startup",
                "hooks": [
                    {
                        "type": "command",
                        "command": "other-tool start"
                    }
                ]
            },
            {
                "matcher": "startup|resume",
                "hooks": [
                    {
                        "type": "command",
                        "command": "shared-group-tool"
                    }
                ]
            }
        ],
        "Stop": [
            {
                "hooks": [
                    {
                        "type": "command",
                        "command": "on-stop"
                    }
                ]
            }
        ]
    }
}
`)
}

// Ported from tests/Feature/SetupCommandTest.php ('replaces a previously
// installed memry SessionStart hook').
func TestClaudeSettingsReplacesAPreviouslyInstalledMemryHook(t *testing.T) {
	path := settingsPath(t)
	writeFile(t, path, `{"hooks":{"SessionStart":[`+
		`{"matcher":"startup","hooks":[{"type":"command","command":"other-tool context"}]},`+
		`{"matcher":"startup|resume|clear|compact","hooks":[{"type":"command","command":"'/old/memry' hook:session-start","timeout":10}]},`+
		`{"matcher":"resume","hooks":[{"type":"command","command":"'/old/memry' hook:session-start"},{"type":"command","command":"echo resumed"}]}]}}`)

	replaceHook(t, path)

	assertContents(t, path, `{
    "hooks": {
        "SessionStart": [
            {
                "matcher": "startup",
                "hooks": [
                    {
                        "type": "command",
                        "command": "other-tool context"
                    }
                ]
            },
            {
                "matcher": "resume",
                "hooks": [
                    {
                        "type": "command",
                        "command": "echo resumed"
                    }
                ]
            },
            {
                "matcher": "startup",
                "hooks": [
                    {
                        "type": "command",
                        "command": "memry hook:session-start",
                        "timeout": 10
                    }
                ]
            }
        ]
    }
}
`)
}

const memryGroupJSON = `{"matcher":"startup","hooks":[{"type":"command","command":"memry hook:session-start","timeout":10}]}`

func TestClaudeSettingsDropsTheHooksThatRemovingTheMemryHookLeavesEmpty(t *testing.T) {
	tests := map[string]struct{ hooks, want string }{
		"only the memry hook": {`{"SessionStart":[` + memryGroupJSON + `]}`, "{\n    \"model\": \"opus\"\n}\n"},
		"memry and a Stop hook": {
			`{"SessionStart":[` + memryGroupJSON + `],"Stop":[{"hooks":[{"type":"command","command":"on-stop"}]}]}`,
			"{\n    \"model\": \"opus\",\n    \"hooks\": {\n        \"Stop\": [\n            {\n                \"hooks\": [\n                    {\n                        \"type\": \"command\",\n                        \"command\": \"on-stop\"\n                    }\n                ]\n            }\n        ]\n    }\n}\n",
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			path := settingsPath(t)
			writeFile(t, path, `{"model":"opus","hooks":`+tt.hooks+`}`)

			removeHook(t, path)

			assertContents(t, path, tt.want)
		})
	}
}

func TestClaudeSettingsLeavesTheSettingsUntouchedWhenThereIsNoMemryHookToRemove(t *testing.T) {
	tests := map[string]string{
		"no hooks":                 `{"model":"opus"}`,
		"empty SessionStart":       `{"hooks":{"SessionStart":[]}}`,
		"other SessionStart hooks": `{"hooks":{"SessionStart":[{"hooks":[{"type":"command","command":"other"}]}]}}`,
	}
	for name, contents := range tests {
		t.Run(name, func(t *testing.T) {
			path := settingsPath(t)
			writeFile(t, path, contents)

			if got := removeHook(t, path); got != agentfiles.NotFound {
				t.Errorf("RemoveSessionStartHook = %v, want NotFound", got)
			}
			assertContents(t, path, contents)
		})
	}
}

func TestClaudeSettingsRemovesTheMemryHookFromAGroupItSharesWithOtherHooks(t *testing.T) {
	path := settingsPath(t)
	writeFile(t, path, `{"hooks":{"SessionStart":[{"hooks":[{"type":"command","command":"memry hook:session-start"},{"type":"command","command":"other"}]}]}}`)

	if got := removeHook(t, path); got != agentfiles.Removed {
		t.Errorf("RemoveSessionStartHook = %v, want Removed", got)
	}
	assertContents(t, path, "{\n    \"hooks\": {\n        \"SessionStart\": [\n            {\n                \"hooks\": [\n                    {\n                        \"type\": \"command\",\n                        \"command\": \"other\"\n                    }\n                ]\n            }\n        ]\n    }\n}\n")
}

func TestClaudeSettingsDoesNotCreateTheSettingsWhenRemovingAHookFromAMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing", "settings.json")

	if got := removeHook(t, path); got != agentfiles.NotFound {
		t.Errorf("RemoveSessionStartHook = %v, want NotFound", got)
	}
	if exists(t, filepath.Dir(path)) {
		t.Error("the settings directory was created")
	}
}

func TestClaudeSettingsLeavesMalformedSettingsUntouchedWhenRemovingTheMemryHook(t *testing.T) {
	tests := map[string]string{
		"not JSON":                   `{"hooks": `,
		"not an object":              `[]`,
		"SessionStart is an object":  `{"hooks": {"SessionStart": {"matcher": "startup"}}}`,
		"a group has non-list hooks": `{"hooks": {"SessionStart": [{"hooks": "memry hook:session-start"}]}}`,
	}
	for name, contents := range tests {
		t.Run(name, func(t *testing.T) {
			path := settingsPath(t)
			writeFile(t, path, contents)

			if got := removeHook(t, path); got != agentfiles.Unsafe {
				t.Errorf("RemoveSessionStartHook = %v, want Unsafe", got)
			}
			assertContents(t, path, contents)
		})
	}
}

// Divergence from PHP, whose json_encode fails on a number too large for
// a float and then overwrites the settings with a lone newline: they are
// left untouched instead.
func TestClaudeSettingsLeavesSettingsItCannotWriteBackUntouched(t *testing.T) {
	path := settingsPath(t)
	contents := `{"limit": 1e400, "hooks": {"SessionStart": [` + memryGroupJSON + `]}}`
	writeFile(t, path, contents)

	if replaceHook(t, path) {
		t.Error("ReplaceSessionStartHook = true, want false")
	}
	if got := removeHook(t, path); got != agentfiles.Unsafe {
		t.Errorf("RemoveSessionStartHook = %v, want Unsafe", got)
	}
	assertContents(t, path, contents)
}
