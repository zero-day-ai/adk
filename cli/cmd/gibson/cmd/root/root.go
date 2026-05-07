// Package root defines the gibson root Cobra command and wires all
// subcommands together.
package root

import (
	"github.com/spf13/cobra"

	"github.com/zero-day-ai/adk/cmd/gibson/cmd/component"
	"github.com/zero-day-ai/adk/cmd/gibson/cmd/docs"
	"github.com/zero-day-ai/adk/cmd/gibson/cmd/inspect"
	wscmd "github.com/zero-day-ai/adk/cmd/gibson/cmd/workspace"
)

var rootCmd = &cobra.Command{
	Use:   "gibson",
	Short: "Gibson Agent Development Kit CLI",
	Long: `gibson — tooling for the Gibson agent / tool / plugin
development lifecycle.

Subcommands:
  init       initialise a Gibson workspace (.gibson/workspace.yaml)
  component  scaffold, validate, register, run components (agent | tool | plugin)
  docs       emit machine-readable docs (JSON Schemas, etc.)
  inspect    show what this principal can do (calls WhoAmI)`,
	SilenceUsage: true,
}

func init() {
	rootCmd.AddCommand(wscmd.Command())
	rootCmd.AddCommand(component.Command())
	rootCmd.AddCommand(docs.Command())
	rootCmd.AddCommand(inspect.Command())
}

// Execute runs the root command. Called from main.
func Execute() error {
	return rootCmd.Execute()
}
