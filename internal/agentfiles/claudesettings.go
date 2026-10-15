package agentfiles

import (
	"bytes"
	"errors"
	"fmt"
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

// ReplaceSessionStartHook adds group to the SessionStart hooks. Like every
// write to the settings, it writes only while the file is still the one it
// read (see edit).
func (s *ClaudeSettings) ReplaceSessionStartHook(group *phpjson.Object, marker string) (bool, error) {
	written := false
	err := s.edit(func(settings *phpjson.Object) bool {
		// Like PHP's ??=, null hooks or SessionStart are replaced too.
		hooks, _ := settings.Get("hooks")
		if hooks == nil {
			hooks = phpjson.NewObject()
			settings.Set("hooks", hooks)
		}
		groups, _ := listOrEmpty(hooks.(*phpjson.Object), "SessionStart")
		kept, _ := withoutHooks(groups, marker)
		hooks.(*phpjson.Object).Set("SessionStart", append(kept, group))
		written = true
		return true
	})
	if errors.Is(err, errUnsafe) {
		return false, nil
	}
	return written && err == nil, err
}

// editAttempts is how many times edit reads the settings and applies its
// change again when another program changed them meanwhile.
const editAttempts = 3

// errUnsafe means the settings are not well formed: edit left them alone.
var errUnsafe = errors.New("not well formed")

// edit applies change to the settings as read and writes them back, with
// their permissions, only while the file is still the one it read: a file
// changed meanwhile is read again and the change applied again, as it is
// computed from what is read, so another program's change is never
// overwritten. A change that returns false writes nothing. Settings that
// are not well formed return errUnsafe, untouched.
func (s *ClaudeSettings) edit(change func(settings *phpjson.Object) bool) error {
	for attempt := 1; ; attempt++ {
		snapshot, err := s.Read()
		if err != nil {
			return err
		}
		if snapshot.settings == nil {
			return errUnsafe
		}
		if !change(snapshot.settings) {
			return nil
		}
		err = s.replaceFrom(snapshot, snapshot.settings, false)
		if errors.Is(err, ErrChanged) && attempt == editAttempts {
			return fmt.Errorf("%s kept changing while memry was writing it; try again: %w", s.path, err)
		}
		if !errors.Is(err, ErrChanged) {
			return err
		}
	}
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

// RemoveSessionStartHook removes every SessionStart hook whose command
// contains marker (and any matcher group left empty by that), keeping
// every other hook and setting. A SessionStart list and hooks object left
// empty are dropped too, as installing the hook creates them when missing.
// Like every write to the settings, it writes only while the file is still
// the one it read (see edit).
func (s *ClaudeSettings) RemoveSessionStartHook(marker string) (Removal, error) {
	removal := NotFound
	err := s.edit(func(settings *phpjson.Object) bool {
		removal = NotFound
		hooks, _ := settings.Get("hooks")
		events, _ := hooks.(*phpjson.Object)
		if events == nil {
			return false
		}
		groups, _ := listOrEmpty(events, "SessionStart")
		kept, removed := withoutHooks(groups, marker)
		if !removed {
			return false
		}
		events.Set("SessionStart", kept)
		if len(kept) == 0 {
			events.Delete("SessionStart")
		}
		if events.Len() == 0 {
			settings.Delete("hooks")
		}
		removal = Removed
		return true
	})
	switch {
	case errors.Is(err, errUnsafe):
		return Unsafe, nil
	case err != nil:
		return NotFound, err
	}
	return removal, nil
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

// Snapshot is the settings file as read once: its bytes, which a write
// from the snapshot compares the file with, and what they hold.
type Snapshot struct {
	raw      []byte
	exists   bool
	settings *phpjson.Object
	env      *phpjson.Object
	ok       bool
}

// OK reports whether the settings are well formed with an env, if any,
// that is an object: whether memry can edit them safely.
func (snap Snapshot) OK() bool {
	return snap.ok
}

// Env is the env object of the settings, nil when there is none.
func (snap Snapshot) Env() *phpjson.Object {
	return snap.env
}

// Setting is the top-level setting key, nil when it is missing or null.
func (snap Snapshot) Setting(key string) any {
	if snap.settings == nil {
		return nil
	}
	value, _ := snap.settings.Get(key)
	return value
}

// Read reads the settings file, or empty settings when there is none.
func (s *ClaudeSettings) Read() (Snapshot, error) {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, fs.ErrNotExist) {
		return Snapshot{settings: phpjson.NewObject(), ok: true}, nil
	}
	if err != nil {
		return Snapshot{}, err
	}
	return snapshotOf(data), nil
}

// snapshotOf decodes the bytes of an existing settings file.
func snapshotOf(data []byte) Snapshot {
	snap := Snapshot{raw: data, exists: true}
	decoded, _ := phpjson.Decode(data)
	settings, ok := decoded.(*phpjson.Object)
	if !ok || !wellFormed(settings) || !encodable(settings) {
		return snap
	}
	snap.settings = settings
	value, _ := settings.Get("env")
	if value == nil {
		snap.ok = true
		return snap
	}
	snap.env, snap.ok = value.(*phpjson.Object)
	return snap
}

// ErrChanged is returned by a write from a snapshot when the file is no
// longer the one the snapshot read.
var ErrChanged = errors.New("the file changed")

// reread reads the file again, returning ErrChanged unless it is exactly
// the one snapshot read, byte for byte. Its objects are new, so changing
// them leaves snapshot as it is.
func (s *ClaudeSettings) reread(snapshot Snapshot) (Snapshot, error) {
	fresh, err := s.Read()
	if err != nil {
		return Snapshot{}, err
	}
	if fresh.exists != snapshot.exists || !bytes.Equal(fresh.raw, snapshot.raw) {
		return Snapshot{}, ErrChanged
	}
	return fresh, nil
}

// SetEnvFrom sets each of vars in the env object of the settings, creating
// it when missing, and keeps every other setting and variable, but only
// when the file is still exactly snapshot (else ErrChanged). It returns
// false, leaving the file untouched, when it is not well formed or its env
// is not an object.
func (s *ClaudeSettings) SetEnvFrom(snapshot Snapshot, vars *phpjson.Object) (bool, error) {
	fresh, err := s.reread(snapshot)
	if err != nil || !fresh.ok {
		return false, err
	}
	if fresh.env == nil {
		fresh.env = phpjson.NewObject()
		fresh.settings.Set("env", fresh.env)
	}
	for key, value := range vars.All() {
		fresh.env.Set(key, value)
	}
	if err := s.replaceFrom(snapshot, fresh.settings, true); err != nil {
		return false, err
	}
	return true, nil
}

// RemoveEnvFrom removes keys from the env object of the settings, keeping
// every other setting and variable, and drops an env left empty (SetEnvFrom
// creates it when missing; an empty one means none), but only when the
// file is still exactly snapshot (else ErrChanged).
func (s *ClaudeSettings) RemoveEnvFrom(snapshot Snapshot, keys []string) (Removal, error) {
	fresh, err := s.reread(snapshot)
	if err != nil {
		return NotFound, err
	}
	if !fresh.ok {
		return Unsafe, nil
	}
	if fresh.env == nil {
		return NotFound, nil
	}
	removed := false
	for _, key := range keys {
		if _, found := fresh.env.Get(key); found {
			fresh.env.Delete(key)
			removed = true
		}
	}
	if !removed {
		return NotFound, nil
	}
	if fresh.env.Len() == 0 {
		fresh.settings.Delete("env")
	}
	if err := s.replaceFrom(snapshot, fresh.settings, true); err != nil {
		return NotFound, err
	}
	return Removed, nil
}

// beforeReplace runs once a write from a snapshot has read the file again
// and is about to replace it; a seam for the tests.
var beforeReplace = func() {}

// replaceFrom writes settings over the file, the only way memry writes the
// settings: only while the file is still exactly snapshot, compared again
// right before every rename attempt (see fsx.WriteAtomicIf), else
// ErrChanged; another program writing it at that same instant still wins,
// the last writer. With ownerOnly, for settings that hold a secret, a new
// file is readable by its owner only, and an existing one loses the
// permissions of everyone else, never gaining any; otherwise the file
// keeps its permissions (a new one gets the default ones).
func (s *ClaudeSettings) replaceFrom(snapshot Snapshot, settings *phpjson.Object, ownerOnly bool) error {
	encoded, err := encode(settings)
	if err != nil {
		return err
	}
	data := append(encoded, '\n')
	check := func() error {
		_, err := s.reread(snapshot)
		return err
	}
	beforeReplace()
	if !ownerOnly {
		return fsx.ReplaceIf(s.path, data, check)
	}
	// The write keeps the mode of an existing file, so it is tightened
	// first.
	if info, err := os.Stat(s.path); err == nil {
		if err := os.Chmod(s.path, info.Mode().Perm()&0o700); err != nil {
			return err
		}
	}
	return fsx.WriteAtomicIf(s.path, data, 0o600, check)
}
