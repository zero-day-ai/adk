// Command gibson is the Gibson Agent Development Kit command-line interface.
//
// It provides tooling for plugin authors, agent developers, and operators
// working with the Gibson platform:
//
//	gibson plugin init <name>   scaffold a new plugin directory
//	gibson plugin validate      validate a plugin manifest
//	gibson plugin enroll        first-time registration with the daemon
//	gibson plugin run           run a plugin locally via plugin.Serve
package main

import (
	"os"

	"github.com/zero-day-ai/adk/cmd/gibson/cmd/root"
)

func main() {
	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}
