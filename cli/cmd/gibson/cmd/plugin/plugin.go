// Package plugin implements the `gibson plugin` subcommand group.
//
// Available subcommands:
//
//	gibson plugin init <name>   scaffold a new plugin skeleton
//	gibson plugin validate      validate a plugin.yaml manifest
//	gibson plugin enroll        first-time registration with the daemon
//	gibson plugin run           run a plugin via plugin.Serve
package plugin

import "github.com/spf13/cobra"

// Command returns the root `plugin` Cobra command that groups all plugin
// subcommands. It is registered with the CLI's root command.
func Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "plugin",
		Short: "Scaffold, validate, enroll, and run Gibson plugins",
		Long: `plugin — tooling for the Gibson plugin development lifecycle.

A Gibson plugin is a stateful gRPC service that runs alongside the daemon,
consumes credentials via the secrets broker, and serves declared methods to
tools. The plugin subcommands cover the full developer lifecycle:

  init      scaffold a new plugin directory from templates
  validate  schema-validate a plugin.yaml manifest
  enroll    first-time registration with the Gibson daemon
  run       run a plugin locally (thin wrapper around plugin.Serve)`,
	}

	cmd.AddCommand(initCmd())
	cmd.AddCommand(validateCmd())
	cmd.AddCommand(enrollCmd())
	cmd.AddCommand(runCmd())

	return cmd
}
