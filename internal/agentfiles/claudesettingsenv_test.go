package agentfiles_test

import (
	"errors"
	"strconv"
	"strings"
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

// The file is compared again right before the rename, after the new
// contents were written aside, so an edit made while they were being
// written is not overwritten either.
func TestClaudeSettingsSetEnvFromComparesAgainRightBeforeReplacing(t *testing.T) {
	path := settingsPath(t)
	writeFile(t, path, `{"env":{"A":"1"}}`)
	settings := agentfiles.NewClaudeSettings(path)
	snapshot, err := settings.Read()
	if err != nil {
		t.Fatal(err)
	}
	edited := `{"env":{"A":"2"}}`
	t.Cleanup(agentfiles.SetBeforeReplace(func() { writeFile(t, path, edited) }))

	ok, err := settings.SetEnvFrom(snapshot, phpjson.NewObject("B", "2"))

	if !errors.Is(err, agentfiles.ErrChanged) || ok {
		t.Errorf("SetEnvFrom = %v, %v; want false, ErrChanged", ok, err)
	}
	assertContents(t, path, edited)
}

// Installing the hook never overwrites what another command wrote since
// the hook write read the file, like the telemetry variables a concurrent
// usage enable added: it reads again and keeps them.
func TestClaudeSettingsHookWriteKeepsAConcurrentChange(t *testing.T) {
	path := settingsPath(t)
	writeFile(t, path, `{"model":"opus"}`)
	changed := false
	t.Cleanup(agentfiles.SetBeforeReplace(func() {
		if !changed {
			changed = true
			writeFile(t, path, `{"model":"opus","env":{"OTEL_METRICS_EXPORTER":"otlp"}}`)
		}
	}))

	if !replaceHook(t, path) {
		t.Fatal("ReplaceSessionStartHook = false, want true")
	}

	contents := readFile(t, path)
	for _, want := range []string{`"OTEL_METRICS_EXPORTER": "otlp"`, `hook:session-start`} {
		if !strings.Contains(contents, want) {
			t.Errorf("settings = %s, want %s kept", contents, want)
		}
	}
}

func TestClaudeSettingsHookRemovalKeepsAConcurrentChange(t *testing.T) {
	path := settingsPath(t)
	replaceHook(t, path)
	changed := false
	t.Cleanup(agentfiles.SetBeforeReplace(func() {
		if !changed {
			changed = true
			contents := strings.Replace(readFile(t, path), `"hooks"`, `"env": {"OTEL_METRICS_EXPORTER": "otlp"}, "hooks"`, 1)
			writeFile(t, path, contents)
		}
	}))

	if got := removeHook(t, path); got != agentfiles.Removed {
		t.Fatalf("RemoveSessionStartHook = %v, want Removed", got)
	}

	contents := readFile(t, path)
	if !strings.Contains(contents, `"OTEL_METRICS_EXPORTER": "otlp"`) || strings.Contains(contents, "hook:session-start") {
		t.Errorf("settings = %s, want the hook removed and the change kept", contents)
	}
}

// A file that keeps changing is left to the other program, with an error
// asking to try again.
func TestClaudeSettingsHookWriteGivesUpOnAFileThatKeepsChanging(t *testing.T) {
	path := settingsPath(t)
	writeFile(t, path, `{}`)
	n := 0
	t.Cleanup(agentfiles.SetBeforeReplace(func() {
		n++
		writeFile(t, path, `{"n":`+strconv.Itoa(n)+`}`)
	}))

	ok, err := agentfiles.NewClaudeSettings(path).ReplaceSessionStartHook(hookGroup(), hookMarker)

	if ok || !errors.Is(err, agentfiles.ErrChanged) || !strings.Contains(err.Error(), "try again") {
		t.Errorf("ReplaceSessionStartHook = %v, %v; want false and a changed error", ok, err)
	}
	if got := readFile(t, path); strings.Contains(got, "hook:session-start") {
		t.Errorf("settings = %s, want the other program's", got)
	}
}
