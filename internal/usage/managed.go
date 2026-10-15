package usage

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/mrtheroi/memry-cli/internal/phpjson"
)

// managedBlockers says which telemetry settings of Claude Code's managed
// settings files in dir stop memry from turning on usage metrics: managed
// settings win over the user's, and a managed OTLP setting removes memry's
// endpoint at startup. The files are managed-settings.json and the *.json
// drop-ins of managed-settings.d, which Claude Code merges. One that exists
// but cannot be read leaves the destination unknown.
func managedBlockers(dir, endpoint string) []string {
	if dir == "" {
		return nil
	}
	lines := managedFileBlockers(filepath.Join(dir, "managed-settings.json"), endpoint)
	dropIns := filepath.Join(dir, "managed-settings.d")
	entries, err := os.ReadDir(dropIns)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return append(lines, unknownManaged(dropIns))
	}
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(name, ".") && strings.HasSuffix(name, ".json") && !entry.IsDir() {
			lines = append(lines, managedFileBlockers(filepath.Join(dropIns, name), endpoint)...)
		}
	}
	return lines
}

// unknownManaged says the managed settings at path cannot be read.
func unknownManaged(path string) string {
	return "Could not read the managed settings " + path + ", so memry cannot determine where Claude Code sends metrics."
}

// managedFileBlockers says which telemetry settings the managed settings
// file at path sets that are not allowed next to memry's export to
// endpoint (see isTelemetry and allowed).
func managedFileBlockers(path, endpoint string) []string {
	unknown := []string{unknownManaged(path)}
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return unknown
	}
	decoded, _ := phpjson.Decode(data)
	settings, ok := decoded.(*phpjson.Object)
	if !ok {
		return unknown
	}
	var lines []string
	// A policy helper computes the managed settings at startup; memry
	// never runs it, so what they set cannot be determined.
	if helper, _ := settings.Get("policyHelper"); helper != nil {
		lines = append(lines, "policyHelper is set in the managed settings "+path+", so memry cannot determine where Claude Code sends metrics.")
	}
	if helper, _ := settings.Get(helperKey); helper != nil {
		lines = append(lines, helperKey+" is set in the managed settings "+path+".")
	}
	value, _ := settings.Get("env")
	if value == nil {
		return lines
	}
	env, ok := value.(*phpjson.Object)
	if !ok {
		return unknown
	}
	for key, value := range env.All() {
		if isTelemetry(key) && !allowed(key, value, endpoint) {
			lines = append(lines, key+" is set in the managed settings "+path+".")
		}
	}
	return lines
}

// ManagedSettingsDir is the system directory of Claude Code's managed
// settings files on goos: managed-settings.json and managed-settings.d
// (https://code.claude.com/docs/en/managed-settings). MDM profiles, the
// registry and server-managed settings are out of memry's sight.
func ManagedSettingsDir(goos string) string {
	switch goos {
	case "darwin":
		return "/Library/Application Support/ClaudeCode"
	case "windows":
		return `C:\Program Files\ClaudeCode`
	}
	return "/etc/claude-code"
}
