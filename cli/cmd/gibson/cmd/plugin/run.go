package plugin

import (
	"time"

	"github.com/spf13/cobra"

	"github.com/zero-day-ai/adk/cmd/gibson/cmd/component"
	"github.com/zero-day-ai/adk/cmd/gibson/internal/deprecation"
)

// runCmd returns the back-compat `gibson plugin run` shim. It exec's
// the compiled plugin binary in the current directory under the same
// process supervisor as `gibson component run`. It does NOT call
// plugin.Serve in-process — that is the binary's job.
//
// This replaces the smoke-test-only behaviour of the pre-spec verb,
// which served health probes but could not handle method invocations.
// The new behaviour is the real run loop.
func runCmd() *cobra.Command {
	var (
		drainTimeout time.Duration
		manifestPath string // accepted for back-compat; ignored
	)
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run the compiled plugin binary (back-compat alias for `component run --kind plugin`)",
		Long: `run is a back-compat alias for ` + "`gibson component run --kind plugin`" + `.

It locates ./<metadata.name> from component.yaml in the current
directory, refuses to launch if ~/.gibson/plugin/<name>/host_key is
missing (run ` + "`gibson component register --token <bootstrap-token>`" + `
first), and supervises the binary's stdout/stderr + signals via
internal/runner.

Exit code 75 from the child indicates the SDK plugin rotation contract
(secret rotated; restart). The CLI surfaces it verbatim.

Example:
  export GIBSON_URL=https://api.zero-day.ai
  gibson plugin run`,
		RunE: func(cmd *cobra.Command, args []string) error {
			deprecation.Notify("plugin run", "component run --kind plugin")
			return component.RunForKind(".", "plugin", drainTimeout)
		},
	}
	cmd.Flags().DurationVar(&drainTimeout, "drain-timeout", 30*time.Second, "max wait between SIGTERM and SIGKILL on shutdown")
	cmd.Flags().StringVar(&manifestPath, "manifest", "./plugin.yaml", "(deprecated; ignored — kept for back-compat)")
	_ = cmd.Flags().MarkHidden("manifest")
	return cmd
}
