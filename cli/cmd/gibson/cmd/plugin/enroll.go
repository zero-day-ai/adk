package plugin

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/zero-day-ai/adk/cmd/gibson/internal/deprecation"
	"github.com/zero-day-ai/adk/cmd/gibson/internal/enroll"
)

// enrollCmd returns the `gibson plugin enroll` Cobra command. Thin
// wrapper around enroll.EnrollPlugin; production logic lives in
// internal/enroll/plugin.go.
func enrollCmd() *cobra.Command {
	var token string

	cmd := &cobra.Command{
		Use:   "enroll",
		Short: "First-time registration of a plugin install with the Gibson daemon",
		Long: `enroll performs first-time registration of a plugin install.

Steps (delegated to internal/enroll/plugin.go):
  1. Loads plugin.yaml from the current directory.
  2. Calls capabilitygrant.Bootstrap with the provided token.
  3. Runs Discover to fetch the daemon's agent-configuration endpoint.
  4. Runs Register to exchange the token for a persistent host key.
  5. Persists the host key at ~/.gibson/plugin/<name>/host_key (mode 0600).

Idempotent: if the host key already exists for this plugin name, enroll
reports success and exits without re-registering. Re-running is safe.

The bootstrap token is single-use and audited at the daemon.

NEVER commit the host key or log the bootstrap token.

Example:
  export GIBSON_URL=https://api.zero-day.ai
  gibson plugin enroll --token eyJhbGci...`,
		RunE: func(cmd *cobra.Command, args []string) error {
			deprecation.Notify("plugin enroll", "component register --kind plugin")
			agentID, err := enroll.EnrollPlugin(context.Background(), enroll.PluginOptions{
				BootstrapToken: token,
			})
			if err != nil {
				return err
			}
			if agentID == "" {
				fmt.Println("plugin enroll: already enrolled (host key exists)")
				return nil
			}
			fmt.Printf("plugin enroll: enrolled (agent_id=%s)\n", agentID)
			return nil
		},
	}

	cmd.Flags().StringVar(&token, "token", "", "bootstrap token (required)")
	if err := cmd.MarkFlagRequired("token"); err != nil {
		panic("enroll: MarkFlagRequired(token): " + err.Error())
	}
	return cmd
}
