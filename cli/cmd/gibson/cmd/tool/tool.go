// Package tool implements the `gibson tool` subcommand group.
//
// Available subcommands:
//
//	gibson tool enroll        first-time registration of a tool install
//
// Spec: component-bootstrap-e2e Requirement 4.
package tool

import "github.com/spf13/cobra"

// Command returns the root `tool` Cobra command.
func Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tool",
		Short: "Tooling for Gibson tools",
		Long: `tool — tooling for the Gibson tool development lifecycle.

A Gibson tool is a stateless sandboxed component (Setec microVM per call)
that an agent can call. The tool subcommands mirror the agent ones:

  enroll    write the credentials file the daemon issued and verify it`,
	}
	cmd.AddCommand(enrollCmd())
	return cmd
}
