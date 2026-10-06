// Command memry is the memry CLI.
package main

import (
	"os"

	"github.com/mrtheroi/memry-cli/internal/commands"
)

// version is set at build time with -ldflags "-X main.version=<version>".
var version = "dev"

func main() {
	if err := commands.NewRoot(version).Execute(); err != nil {
		os.Exit(1)
	}
}
