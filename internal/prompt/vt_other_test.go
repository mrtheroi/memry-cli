//go:build !windows

package prompt_test

import (
	"bytes"
	"testing"

	"github.com/mrtheroi/memry-cli/internal/prompt"
)

// S1.8.c (pin): Unix terminals always take escape sequences; nothing is
// switched on.
func TestEnableVTIsAlwaysOnOutsideWindows(t *testing.T) {
	if !prompt.EnableVT(&bytes.Buffer{}) {
		t.Error("EnableVT = false, want true")
	}
}
