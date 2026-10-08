package uninstall_test

import (
	"testing"

	"github.com/mrtheroi/memry-cli/internal/uninstall"
)

func TestUninstall_NoHomeExitsNonZero(t *testing.T) {
	h, s := newHarness(t), newServer(t)
	delete(h.env, "MEMRY_CONFIG")
	delete(h.env, "HOME")

	output, code := h.run(uninstall.Uninstall, force)

	assertExit(t, code, 1, output)
	assertContains(t, output, "Could not find your home directory; set USERPROFILE or HOME.\n")
	if len(h.claude.ran) != 0 || len(s.received()) != 0 {
		t.Errorf("ran %v and sent %v, want nothing done", h.claude.ran, s.received())
	}
}

func TestDeleteAccount_NoHomeExitsNonZero(t *testing.T) {
	h := newHarness(t)
	delete(h.env, "MEMRY_CONFIG")
	delete(h.env, "HOME")

	output, code := h.run(uninstall.DeleteAccount, nil)

	assertExit(t, code, 1, output)
	assertContains(t, output, "Could not find your home directory; set USERPROFILE or HOME.\n")
}
