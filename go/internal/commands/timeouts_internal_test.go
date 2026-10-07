package commands

import (
	"testing"
	"time"

	"github.com/spf13/cobra"
)

// Like the PHP CLI, the proxy gives the server 30 seconds per message, and
// the hook 3 seconds (ported from "requests the context with the bearer
// token, an encoded project and a 3 second timeout"). The hook also gives
// git 3 seconds to find the repository.
func TestTimeouts(t *testing.T) {
	cmd := &cobra.Command{}
	if got := proxyEnv(cmd, "dev").HTTP.Timeout; got != 30*time.Second {
		t.Errorf("proxy timeout = %v, want 30s", got)
	}
	if got := sessionStartEnv(cmd, "dev", false).HTTP.Timeout; got != 3*time.Second {
		t.Errorf("hook timeout = %v, want 3s", got)
	}
	if got := sessionStartEnv(cmd, "dev", false).GitTimeout; got != 3*time.Second {
		t.Errorf("hook git timeout = %v, want 3s", got)
	}
}
