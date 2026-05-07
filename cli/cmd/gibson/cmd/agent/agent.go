// Package agent implements the `gibson agent` subcommand group.
//
// Available subcommands:
//
//	gibson agent enroll        first-time registration of an agent install
//
// Spec: component-bootstrap-e2e Requirement 3.
package agent

import "github.com/spf13/cobra"

// Command returns the root `agent` Cobra command.
func Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "agent",
		Short: "Tooling for Gibson agents",
		Long: `agent — tooling for the Gibson agent development lifecycle.

A Gibson agent is a long-running LLM-driven gRPC process that registers
with the daemon, calls tools, queries plugins, and persists memory.
The agent subcommands cover the operational lifecycle:

  enroll    write the credentials file the daemon issued and verify it`,
	}
	cmd.AddCommand(enrollCmd())
	return cmd
}
