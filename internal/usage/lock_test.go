package usage_test

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mrtheroi/memry-cli/internal/agents"
	"github.com/mrtheroi/memry-cli/internal/setup"
	"github.com/mrtheroi/memry-cli/internal/uninstall"
	"github.com/mrtheroi/memry-cli/internal/usage"
)

// Two usage commands never interleave: one started while another is in
// the middle of enabling waits for it, then gives up cleanly, and the
// first one's setup stays recorded.
func TestOverlappingEnablesNeverInterleave(t *testing.T) {
	s := newServer(t)
	h := newHarness(t)
	h.login(s.URL)
	t.Cleanup(usage.SetLockWait(50 * time.Millisecond))
	var second string
	var secondCode int
	var started atomic.Bool
	s.onIssue = func() {
		if !started.Swap(true) {
			second, secondCode = h.run(usage.Enable)
		}
	}

	output, code := h.run(usage.Enable)

	assertExit(t, code, 0, output)
	assertExit(t, secondCode, 1, second)
	assertContains(t, second, "Another memry command is changing the memry config. If none is running (for example after a crash), delete ")
	if got := s.activeTokens(); !reflect.DeepEqual(got, []int{7}) {
		t.Errorf("active tokens = %v, want only the first enable's", got)
	}
	h.run(usage.Status)
	assertContains(t, readText(t, h.configPath), `"token_id": 7`)
}

// The lock lives in the user's own cache directory, which other users
// cannot write to, not in a shared temporary directory.
func TestTheLockIsInTheUsersCacheDirectory(t *testing.T) {
	s := newServer(t)
	h := newHarness(t)
	h.login(s.URL)
	var locks []string
	s.onIssue = func() {
		locks, _ = filepath.Glob(filepath.Join(h.cacheDir, "memry", "memry-usage-*.lock"))
	}

	output, code := h.run(usage.Enable)

	assertExit(t, code, 0, output)
	if len(locks) != 1 {
		t.Errorf("locks under %s while enabling = %v, want one", h.cacheDir, locks)
	}
}

// `memry setup` takes the same lock for its config writes: a setup that
// overlaps an enable waits for it, then gives up cleanly, and neither
// overwrites the other's config.
func TestSetupDuringEnableWaitsForTheLock(t *testing.T) {
	s := newServer(t)
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{}`)
	}))
	t.Cleanup(other.Close)
	h := newHarness(t)
	h.login(s.URL)
	t.Cleanup(usage.SetLockWait(50 * time.Millisecond))
	var out bytes.Buffer
	setupCode := -1
	var started atomic.Bool
	s.onIssue = func() {
		if started.Swap(true) {
			return
		}
		setupCode = setup.Run(setup.Env{
			Args: []string{"--url", other.URL, "--token=other-token", "--agents=", "-n"},
			LookupEnv: func(key string) (string, bool) {
				value, ok := h.env[key]
				return value, ok
			},
			Unsetenv: func(string) error { return nil },
			Out:      &out,
			Agents:   agents.NewSupported(),
			HTTP:     h.client(),
			CacheDir: h.cacheDir,
		})
	}

	output, code := h.run(usage.Enable)

	assertExit(t, code, 0, output)
	assertExit(t, setupCode, 1, out.String())
	assertContains(t, out.String(), "Another memry command is changing the memry config. If none is running (for example after a crash), delete ")
	cfg := readText(t, h.configPath)
	assertContains(t, cfg, `"url": "`+s.URL+`"`)
	assertContains(t, cfg, `"token_id": 7`)
}

// Uninstall takes the same lock for its config writes: started while an
// enable is in flight, it gives up with the shared lock message instead of
// deleting the config the enable is about to record its setup in.
func TestUninstallDuringEnableWaitsForTheLock(t *testing.T) {
	s := newServer(t)
	h := newHarness(t)
	h.login(s.URL)
	t.Cleanup(usage.SetLockWait(50 * time.Millisecond))
	var out bytes.Buffer
	uninstallCode := -1
	var started atomic.Bool
	s.onIssue = func() {
		if started.Swap(true) {
			return
		}
		uninstallCode = uninstall.Uninstall(uninstall.Env{
			Args: []string{"--force"},
			LookupEnv: func(key string) (string, bool) {
				value, ok := h.env[key]
				return value, ok
			},
			Out:      &out,
			Agents:   agents.NewSupported(),
			HTTP:     h.client(),
			CacheDir: h.cacheDir,
		})
	}

	output, code := h.run(usage.Enable)

	assertExit(t, code, 0, output)
	assertExit(t, uninstallCode, 1, out.String())
	assertContains(t, out.String(), "Another memry command is changing the memry config. If none is running (for example after a crash), delete ")
	cfg := readText(t, h.configPath)
	assertContains(t, cfg, `"token": "1|login-token"`)
	assertContains(t, cfg, `"token_id": 7`)
}

// Without a usage record there is nothing to reconcile or clean up: status
// and disable answer without the lock, so a cache directory that cannot
// hold it does not matter.
func TestStatusAndDisableWithoutARecordNeedNoLock(t *testing.T) {
	for name, want := range map[string]string{"status": "Usage metrics are off.", "disable": "Usage metrics are not on; nothing to remove."} {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t)
			h.login("https://memry.example.com")
			h.cacheDir = filepath.Join(t.TempDir(), "not-a-directory")
			writeRaw(t, h.cacheDir, "a file")
			command := usage.Status
			if name == "disable" {
				command = usage.Disable
			}

			output, code := h.run(command)

			assertExit(t, code, 0, output)
			assertContains(t, output, want)
		})
	}
}
