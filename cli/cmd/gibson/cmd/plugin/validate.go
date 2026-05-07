package plugin

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
	"github.com/zero-day-ai/sdk/plugin/manifest"

	"github.com/zero-day-ai/adk/cmd/gibson/internal/deprecation"
)

// validateCmd returns the `gibson plugin validate` Cobra command.
func validateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "validate [path]",
		Short: "Schema-validate a plugin manifest",
		Long: `validate loads and validates a plugin.yaml manifest against the
plugin.gibson.zero-day.ai/v1 schema.

On success: prints "manifest valid" and exits with code 0.
On failure: prints each validation violation with field names and, where
available, YAML line numbers; exits with code 2.

No daemon connectivity is required — validation is entirely local.

Examples:
  gibson plugin validate
  gibson plugin validate ./my-plugin/plugin.yaml`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			deprecation.Notify("plugin validate", "component validate")
			path := "./plugin.yaml"
			if len(args) > 0 {
				path = args[0]
			}
			return runValidate(path, os.Stdout, os.Stderr, osExiter)
		},
	}
	return cmd
}

// exiter is the function called to exit the process. Replaced in tests.
type exiter func(code int)

// osExiter calls os.Exit. This is the production implementation.
func osExiter(code int) { os.Exit(code) }

// validateLoadManifest is the manifest loader used by runValidate.
// Exposed as a variable so tests can verify it's called with the right path.
var validateLoadManifest = manifest.Load

// runValidate loads and validates the manifest at path.
//
// On a validation error it writes a structured message to errW and calls
// exit(2), so callers (CI, Makefile) can distinguish schema errors from I/O
// errors. The exit function is injectable for testing.
func runValidate(path string, outW, errW io.Writer, exit exiter) error {
	m, err := validateLoadManifest(path)
	if err != nil {
		if manifest.IsValidationError(err) {
			// Validation error: structured output to stderr + exit 2.
			fmt.Fprintf(errW, "manifest invalid: %v\n", err)
			exit(2)
			return nil // unreachable in production; reached in tests with fake exiter
		}
		// I/O or parse error.
		return fmt.Errorf("plugin validate: %w", err)
	}

	fmt.Fprintf(outW, "manifest valid: %s v%s (%d method(s))\n",
		m.Metadata.Name, m.Metadata.Version, len(m.Spec.Methods))
	return nil
}
