package usage_test

import (
	"path/filepath"
	"reflect"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mrtheroi/memry-cli/internal/usage"
)

// Two usage commands never interleave: one started while another is in
// the middle of enabling waits for it, then gives up cleanly, and the
// first one's setup stays recorded.
func TestOverlappingEnablesNeverInterleave(t *testing.T) {
	s := newServer(t)
	h := newHarness(t)
	h.login(s.URL)
	t.Cleanup(usage.SetLockTiming(50*time.Millisecond, time.Hour))
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
	assertContains(t, second, "Another memry usage command is running; try again when it is done.")
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
