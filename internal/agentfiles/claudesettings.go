package agentfiles

import (
	"errors"
	"io/fs"
	"os"
	"strings"

	"github.com/mrtheroi/memry-cli/internal/fsx"
	"github.com/mrtheroi/memry-cli/internal/phpjson"
)

// ClaudeSettings is Claude Code's settings.json, where memry installs its
// SessionStart hook.
type ClaudeSettings struct {
	path string
}

// NewClaudeSettings returns the settings file at path.
func NewClaudeSettings(path string) *ClaudeSettings {
	return &ClaudeSettings{path: path}
}

// Path is the file's path.
func (s *ClaudeSettings) Path() string {
	return s.path
}

// ReplaceSessionStartHook adds group to the SessionStart hooks.
func (s *ClaudeSettings) ReplaceSessionStartHook(group *phpjson.Object, marker string) (bool, error) {
	settings, ok, err := s.read()
	if !ok || err != nil {
		return false, err
	}
	// Like PHP's ??=, null hooks or SessionStart are replaced too.
	hooks, _ := settings.Get("hooks")
	if hooks == nil {
		hooks = phpjson.NewObject()
		settings.Set("hooks", hooks)
	}
	groups, _ := listOrEmpty(hooks.(*phpjson.Object), "SessionStart")
	kept, _ := withoutHooks(groups, marker)
	hooks.(*phpjson.Object).Set("SessionStart", append(kept, group))
	return true, s.write(settings)
}

// read decodes the file, or returns an empty object when there is none.
// It returns false when the file is not well formed (see wellFormed) or
// cannot be written back (PHP would write a lone newline instead).
func (s *ClaudeSettings) read() (*phpjson.Object, bool, error) {
	contents, err := os.ReadFile(s.path)
	if errors.Is(err, fs.ErrNotExist) {
		return phpjson.NewObject(), true, nil
	}
	if err != nil {
		return nil, false, err
	}
	decoded, _ := phpjson.Decode(contents)
	settings, ok := decoded.(*phpjson.Object)
	return settings, ok && wellFormed(settings) && encodable(settings), nil
}

// wellFormed reports whether the hooks of settings, if present, have the
// shape Claude Code expects: an object whose SessionStart is a list of
// groups (objects) with a list of hooks. Like PHP's ?? and isset, a null
// is the same as a missing key.
func wellFormed(settings *phpjson.Object) bool {
	hooks, _ := settings.Get("hooks")
	if hooks == nil {
		return true
	}
	events, ok := hooks.(*phpjson.Object)
	if !ok {
		return false
	}
	groups, ok := listOrEmpty(events, "SessionStart")
	if !ok {
		return false
	}
	for _, group := range groups {
		group, ok := group.(*phpjson.Object)
		if !ok {
			return false
		}
		if _, ok := listOrEmpty(group, "hooks"); !ok {
			return false
		}
	}
	return true
}

// listOrEmpty returns the list at key of o, an empty one when it is
// missing or null, or false when it is something else.
func listOrEmpty(o *phpjson.Object, key string) ([]any, bool) {
	value, _ := o.Get(key)
	if value == nil {
		return nil, true
	}
	list, ok := value.([]any)
	return list, ok
}

// write writes the settings like PHP's json_encode with pretty printing
// and unescaped slashes and Unicode.
func (s *ClaudeSettings) write(settings *phpjson.Object) error {
	return replaceWithJSON(s.path, settings)
}

// RemoveSessionStartHook removes every SessionStart hook whose command
// contains marker (and any matcher group left empty by that), keeping
// every other hook and setting. A SessionStart list and hooks object left
// empty are dropped too, as installing the hook creates them when missing.
func (s *ClaudeSettings) RemoveSessionStartHook(marker string) (Removal, error) {
	settings, ok, err := s.read()
	if err != nil {
		return NotFound, err
	}
	if !ok {
		return Unsafe, nil
	}
	hooks, _ := settings.Get("hooks")
	events, _ := hooks.(*phpjson.Object)
	if events == nil {
		return NotFound, nil
	}
	groups, _ := listOrEmpty(events, "SessionStart")
	kept, removed := withoutHooks(groups, marker)
	if !removed {
		return NotFound, nil
	}
	events.Set("SessionStart", kept)
	if len(kept) == 0 {
		events.Delete("SessionStart")
	}
	if events.Len() == 0 {
		settings.Delete("hooks")
	}
	return Removed, s.write(settings)
}

// withoutHooks returns the groups without the hooks whose command contains
// marker, dropping the groups that leaves empty, and whether it removed
// any. Like PHP's $hook->command ?? ”, a hook that is not an object, or
// whose command is not a string, has no command.
func withoutHooks(groups []any, marker string) ([]any, bool) {
	kept := []any{}
	removed := false
	for _, group := range groups {
		group := group.(*phpjson.Object)
		hooks, _ := listOrEmpty(group, "hooks")
		var others []any
		for _, hook := range hooks {
			var command any
			if hook, ok := hook.(*phpjson.Object); ok {
				command, _ = hook.Get("command")
			}
			if command, ok := command.(string); !ok || !strings.Contains(command, marker) {
				others = append(others, hook)
			}
		}
		switch {
		case len(others) == len(hooks):
			kept = append(kept, group)
		case len(others) > 0:
			group.Set("hooks", others)
			kept = append(kept, group)
		}
		removed = removed || len(others) != len(hooks)
	}
	return kept, removed
}

// SetEnv sets each of vars in the env object of the settings, creating it
// when missing, and keeps every other setting and variable. It returns
// false, leaving the file untouched, when it is not well formed or its env
// is not an object.
func (s *ClaudeSettings) SetEnv(vars *phpjson.Object) (bool, error) {
	return s.SetEnvIf(nil, vars)
}

// ErrChanged is returned by SetEnvIf when a variable no longer has the
// value expected.
var ErrChanged = errors.New("a variable changed")

// SetEnvIf is SetEnv when each variable of expected still has its value
// (nil for a missing or null one), as read when the caller decided on the
// change; otherwise it returns ErrChanged, leaving the file untouched.
func (s *ClaudeSettings) SetEnvIf(expected map[string]any, vars *phpjson.Object) (bool, error) {
	settings, env, ok, err := s.readEnv()
	if !ok || err != nil {
		return false, err
	}
	for key, want := range expected {
		var value any
		if env != nil {
			value, _ = env.Get(key)
		}
		if value != want {
			return false, ErrChanged
		}
	}
	if env == nil {
		env = phpjson.NewObject()
		settings.Set("env", env)
	}
	for key, value := range vars.All() {
		env.Set(key, value)
	}
	return true, s.writeOwnerOnly(settings)
}

// writeOwnerOnly is write for settings that hold a secret: a new file is
// readable by its owner only, and an existing one loses the permissions
// of everyone else, never gaining any.
func (s *ClaudeSettings) writeOwnerOnly(settings *phpjson.Object) error {
	encoded, err := encode(settings)
	if err != nil {
		return err
	}
	// The write keeps the mode of an existing file, so it is tightened
	// first.
	if info, err := os.Stat(s.path); err == nil {
		if err := os.Chmod(s.path, info.Mode().Perm()&0o700); err != nil {
			return err
		}
	}
	return fsx.WriteAtomic(s.path, append(encoded, '\n'), 0o600)
}

// readEnv reads the settings and their env object, nil when it is missing
// or null. It returns false when the file is not well formed or its env is
// something else.
func (s *ClaudeSettings) readEnv() (*phpjson.Object, *phpjson.Object, bool, error) {
	settings, ok, err := s.read()
	if !ok || err != nil {
		return nil, nil, false, err
	}
	value, _ := settings.Get("env")
	if value == nil {
		return settings, nil, true, nil
	}
	env, ok := value.(*phpjson.Object)
	return settings, env, ok, nil
}

// RemoveEnv removes keys from the env object of the settings, keeping
// every other setting and variable, and drops an env left empty.
func (s *ClaudeSettings) RemoveEnv(keys []string) (Removal, error) {
	settings, env, ok, err := s.readEnv()
	if err != nil {
		return NotFound, err
	}
	if !ok {
		return Unsafe, nil
	}
	if env == nil {
		return NotFound, nil
	}
	removed := false
	for _, key := range keys {
		if _, found := env.Get(key); found {
			env.Delete(key)
			removed = true
		}
	}
	if !removed {
		return NotFound, nil
	}
	// SetEnv creates the env when missing; an empty one means none.
	if env.Len() == 0 {
		settings.Delete("env")
	}
	return Removed, s.writeOwnerOnly(settings)
}

// Env returns the env object of the settings, nil when there is none. It
// returns false when the file is not well formed or its env is not an
// object.
func (s *ClaudeSettings) Env() (*phpjson.Object, bool, error) {
	_, env, ok, err := s.readEnv()
	return env, ok, err
}

// Setting returns the top-level setting key, nil when it is missing or
// null. It returns false when the file is not well formed.
func (s *ClaudeSettings) Setting(key string) (any, bool, error) {
	settings, ok, err := s.read()
	if !ok || err != nil {
		return nil, ok, err
	}
	value, _ := settings.Get(key)
	return value, true, nil
}
