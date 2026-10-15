package agents_test

import (
	"path/filepath"
	"testing"

	"github.com/mrtheroi/memry-cli/internal/agents"
)

func TestClaudeSettingsPath(t *testing.T) {
	sep := string(filepath.Separator)
	tests := map[string]struct {
		goos string
		vars map[string]string
		want string
	}{
		// The directory is used as given, joined with the OS separator.
		"config dir":          {"linux", map[string]string{"CLAUDE_CONFIG_DIR": "/c", "HOME": "/h"}, "/c" + sep + "settings.json"},
		"home":                {"linux", map[string]string{"HOME": "/h"}, "/h" + sep + filepath.Join(".claude", "settings.json")},
		"windows userprofile": {"windows", map[string]string{"HOME": "/decoy", "USERPROFILE": `C:\Users\ana`}, `C:\Users\ana` + sep + filepath.Join(".claude", "settings.json")},
		"windows config dir":  {"windows", map[string]string{"CLAUDE_CONFIG_DIR": `C:\Users\ana\work`}, `C:\Users\ana\work` + sep + "settings.json"},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got, _ := agents.ClaudeSettingsPath(tt.goos, func(key string) string { return tt.vars[key] })
			if got != tt.want {
				t.Errorf("ClaudeSettingsPath = %q, want %q", got, tt.want)
			}
		})
	}
}

// A relative CLAUDE_CONFIG_DIR is resolved against the working directory,
// so the path memry records means the same file from any other one.
func TestClaudeSettingsPathResolvesARelativeConfigDir(t *testing.T) {
	work := filepath.Join(t.TempDir(), "work")
	t.Cleanup(agents.SetGetwd(func() (string, error) { return work, nil }))

	got, _ := agents.ClaudeSettingsPath("linux", func(key string) string {
		return map[string]string{"CLAUDE_CONFIG_DIR": "profile"}[key]
	})

	if want := filepath.Join(work, "profile") + string(filepath.Separator) + "settings.json"; got != want {
		t.Errorf("ClaudeSettingsPath = %q, want %q", got, want)
	}
}

// CLAUDE_CONFIG_DIR is resolved the way goos does: on Windows a drive or
// UNC path is absolute, a path rooted without a drive is on the working
// directory's drive, and anything else is under the working directory; on
// Unix only a path starting with / is absolute.
func TestClaudeSettingsPathResolvesConfigDirsLikeTheOS(t *testing.T) {
	sep := string(filepath.Separator)
	cwds := map[string]string{"windows": `C:\work`, "linux": "/work"}
	tests := []struct {
		goos, dir, want string
	}{
		{"windows", `C:\x`, `C:\x`},
		{"windows", `\\server\share\x`, `\\server\share\x`},
		{"windows", `\x`, `C:\x`},
		{"windows", "/c", "C:/c"},
		{"windows", "x", filepath.Join(`C:\work`, "x")},
		{"linux", "/c", "/c"},
		{"linux", "x", filepath.Join("/work", "x")},
		// Windows paths are relative on Unix, unless the OS running the
		// test takes them as absolute (only when it is Windows).
		{"linux", `C:\x`, unixAbsolute(`C:\x`)},
		{"linux", `\\server\share\x`, unixAbsolute(`\\server\share\x`)},
		{"linux", `\x`, unixAbsolute(`\x`)},
	}
	for _, tt := range tests {
		t.Run(tt.goos+" "+tt.dir, func(t *testing.T) {
			t.Cleanup(agents.SetGetwd(func() (string, error) { return cwds[tt.goos], nil }))

			got, _ := agents.ClaudeSettingsPath(tt.goos, func(key string) string {
				return map[string]string{"CLAUDE_CONFIG_DIR": tt.dir}[key]
			})

			if want := tt.want + sep + "settings.json"; got != want {
				t.Errorf("ClaudeSettingsPath = %q, want %q", got, want)
			}
		})
	}
}

// unixAbsolute is what a Unix goos makes of dir under /work.
func unixAbsolute(dir string) string {
	if filepath.IsAbs(dir) {
		return dir
	}
	return filepath.Join("/work", dir)
}

// A Windows path relative to a drive's current directory (C:profile)
// cannot be resolved without per-drive working directories: it is refused.
func TestClaudeSettingsPathRefusesADriveRelativeConfigDir(t *testing.T) {
	_, err := agents.ClaudeSettingsPath("windows", func(key string) string {
		return map[string]string{"CLAUDE_CONFIG_DIR": "C:profile"}[key]
	})

	if err == nil || err.Error() != `CLAUDE_CONFIG_DIR C:profile is relative to a drive's current directory; set it to an absolute path, like C:\profile` {
		t.Errorf("ClaudeSettingsPath error = %v, want the drive-relative refusal", err)
	}
}
