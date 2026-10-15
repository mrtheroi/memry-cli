package agentfiles_test

import (
	"errors"
	"testing"

	"github.com/mrtheroi/memry-cli/internal/agentfiles"
	"github.com/mrtheroi/memry-cli/internal/phpjson"
)

func setEnv(t *testing.T, path string, pairs ...any) bool {
	t.Helper()
	settings := agentfiles.NewClaudeSettings(path)
	snapshot, err := settings.Read()
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	ok, err := settings.SetEnvFrom(snapshot, phpjson.NewObject(pairs...))
	if err != nil {
		t.Fatalf("SetEnvFrom: %v", err)
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
	settings := agentfiles.NewClaudeSettings(path)
	snapshot, err := settings.Read()
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	removed, err := settings.RemoveEnvFrom(snapshot, keys)
	if err != nil {
		t.Fatalf("RemoveEnvFrom: %v", err)
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

func TestClaudeSettingsReadReadsTheVariables(t *testing.T) {
	path := settingsPath(t)
	writeFile(t, path, `{"env":{"FOO":"bar"}}`)

	snapshot, err := agentfiles.NewClaudeSettings(path).Read()

	if err != nil || !snapshot.OK() {
		t.Fatalf("Read = %v, %v; want ok", snapshot.OK(), err)
	}
	if value, _ := snapshot.Env().Get("FOO"); value != "bar" {
		t.Errorf("FOO = %v, want bar", value)
	}
}

// A write from a snapshot happens only when the whole file is still the
// one read: any change since, even outside the env, makes it stop.
func TestClaudeSettingsSetEnvFromStopsWhenTheFileChanged(t *testing.T) {
	path := settingsPath(t)
	writeFile(t, path, `{"env":{"A":"1"}}`)
	settings := agentfiles.NewClaudeSettings(path)
	snapshot, err := settings.Read()
	if err != nil {
		t.Fatal(err)
	}
	changed := `{"env":{"A":"1"},"otelHeadersHelper":"/bin/headers.sh"}`
	writeFile(t, path, changed)

	ok, err := settings.SetEnvFrom(snapshot, phpjson.NewObject("B", "2"))

	if !errors.Is(err, agentfiles.ErrChanged) || ok {
		t.Errorf("SetEnvFrom = %v, %v; want false, ErrChanged", ok, err)
	}
	assertContents(t, path, changed)
}

func TestClaudeSettingsRemoveEnvFromStopsWhenTheFileChanged(t *testing.T) {
	path := settingsPath(t)
	writeFile(t, path, `{"env":{"A":"1"}}`)
	settings := agentfiles.NewClaudeSettings(path)
	snapshot, err := settings.Read()
	if err != nil {
		t.Fatal(err)
	}
	changed := `{"env":{"A":"2"}}`
	writeFile(t, path, changed)

	removed, err := settings.RemoveEnvFrom(snapshot, []string{"A"})

	if !errors.Is(err, agentfiles.ErrChanged) || removed != agentfiles.NotFound {
		t.Errorf("RemoveEnvFrom = %v, %v; want NotFound, ErrChanged", removed, err)
	}
	assertContents(t, path, changed)
}
