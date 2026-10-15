package agentfiles_test

import (
	"testing"

	"github.com/mrtheroi/memry-cli/internal/agentfiles"
	"github.com/mrtheroi/memry-cli/internal/phpjson"
)

func setEnv(t *testing.T, path string, pairs ...any) bool {
	t.Helper()
	ok, err := agentfiles.NewClaudeSettings(path).SetEnv(phpjson.NewObject(pairs...))
	if err != nil {
		t.Fatalf("SetEnv: %v", err)
	}
	return ok
}

func TestClaudeSettingsSetEnvKeepsEveryOtherSettingAndVariable(t *testing.T) {
	path := settingsPath(t)
	writeFile(t, path, `{"model":"opus","env":{"FOO":"bar","OTEL_METRICS_EXPORTER":"none"}}`)

	if !setEnv(t, path, "OTEL_METRICS_EXPORTER", "otlp", "NEW", "1") {
		t.Fatal("SetEnv = false, want true")
	}

	assertContents(t, path, `{
    "model": "opus",
    "env": {
        "FOO": "bar",
        "OTEL_METRICS_EXPORTER": "otlp",
        "NEW": "1"
    }
}
`)
}

func TestClaudeSettingsSetEnvLeavesAnEnvThatIsNotAnObjectUntouched(t *testing.T) {
	path := settingsPath(t)
	writeFile(t, path, `{"env":["FOO"]}`)

	if setEnv(t, path, "NEW", "1") {
		t.Error("SetEnv = true, want false")
	}

	assertContents(t, path, `{"env":["FOO"]}`)
}

func removeEnv(t *testing.T, path string, keys ...string) agentfiles.Removal {
	t.Helper()
	removed, err := agentfiles.NewClaudeSettings(path).RemoveEnv(keys)
	if err != nil {
		t.Fatalf("RemoveEnv: %v", err)
	}
	return removed
}

func TestClaudeSettingsRemoveEnvRemovesOnlyTheGivenVariables(t *testing.T) {
	path := settingsPath(t)
	writeFile(t, path, `{"env":{"FOO":"bar","A":"1","B":"2"},"model":"opus"}`)

	if got := removeEnv(t, path, "A", "B", "MISSING"); got != agentfiles.Removed {
		t.Fatalf("RemoveEnv = %v, want Removed", got)
	}

	assertContents(t, path, `{
    "env": {
        "FOO": "bar"
    },
    "model": "opus"
}
`)
}

// SetEnv creates the env object when missing, so an env left empty is
// dropped: an empty env means the same as none.
func TestClaudeSettingsRemoveEnvDropsAnEnvLeftEmpty(t *testing.T) {
	path := settingsPath(t)
	writeFile(t, path, `{"model":"opus","env":{"A":"1"}}`)

	removeEnv(t, path, "A")

	assertContents(t, path, `{
    "model": "opus"
}
`)
}

func TestClaudeSettingsRemoveEnvWithoutAnEnvLeavesTheFileUntouched(t *testing.T) {
	for _, contents := range []string{`{"model":"opus"}`, `{"env":null}`} {
		path := settingsPath(t)
		writeFile(t, path, contents)

		if got := removeEnv(t, path, "A"); got != agentfiles.NotFound {
			t.Errorf("RemoveEnv(%s) = %v, want NotFound", contents, got)
		}

		assertContents(t, path, contents)
	}
}

func TestClaudeSettingsEnvReadsTheVariables(t *testing.T) {
	path := settingsPath(t)
	writeFile(t, path, `{"env":{"FOO":"bar"}}`)

	env, ok, err := agentfiles.NewClaudeSettings(path).Env()

	if err != nil || !ok {
		t.Fatalf("Env = %v, %v; want ok", ok, err)
	}
	if value, _ := env.Get("FOO"); value != "bar" {
		t.Errorf("FOO = %v, want bar", value)
	}
}
