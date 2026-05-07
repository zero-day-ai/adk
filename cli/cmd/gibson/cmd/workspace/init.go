// Package workspace implements the `gibson init` cobra verb that
// scaffolds a workspace.yaml at the local or global path.
package workspace

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	wsi "github.com/zero-day-ai/adk/cmd/gibson/internal/workspace"
)

// Command returns the `gibson init` cobra command. The verb is named
// `init` (not `workspace init`) to match the spec — workspace
// bootstrap is a one-shot setup verb at the top level.
func Command() *cobra.Command {
	var (
		gibsonURL string
		tenantRef string
		comment   string
		global    bool
		force     bool
	)

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialise a Gibson workspace (writes .gibson/workspace.yaml)",
		Long: `init writes a workspace.yaml that pins GIBSON_URL and the active
tenant reference for this workspace, so subsequent ` + "`" + `gibson component <verb>` + "`" + `
calls do not require the flags every time.

By default the file is written to ./.gibson/workspace.yaml (per-project).
Pass --global to write to ~/.gibson/workspace.yaml (machine-wide
fallback).

The workspace file is non-secret. It MUST NOT contain client_id /
client_secret / bootstrap_token / host_key / password / secret / token
fields — Load() rejects them at parse time. Credentials live at
~/.gibson/{agent,tool,plugin}/credentials with mode 0600.

Examples:
  gibson init --gibson-url https://api.zero-day.ai --tenant-ref tenants/acme
  gibson init --global --gibson-url https://api.zero-day.ai --tenant-ref tenants/acme
  gibson init --force --gibson-url ...   # overwrite existing`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInit(cmd.OutOrStdout(), cmd.ErrOrStderr(), initOptions{
				GibsonURL: gibsonURL,
				TenantRef: tenantRef,
				Comment:   comment,
				Global:    global,
				Force:     force,
			})
		},
	}

	cmd.Flags().StringVar(&gibsonURL, "gibson-url", "", "Gibson platform URL (e.g. https://api.zero-day.ai). Required.")
	cmd.Flags().StringVar(&tenantRef, "tenant-ref", "", "Tenant reference (e.g. tenants/acme). Optional.")
	cmd.Flags().StringVar(&comment, "comment", "", "Optional free-form note.")
	cmd.Flags().BoolVar(&global, "global", false, "Write to ~/.gibson/workspace.yaml instead of ./.gibson/workspace.yaml.")
	cmd.Flags().BoolVar(&force, "force", false, "Overwrite an existing workspace file.")

	if err := cmd.MarkFlagRequired("gibson-url"); err != nil {
		panic("init: MarkFlagRequired(gibson-url): " + err.Error())
	}
	return cmd
}

type initOptions struct {
	GibsonURL string
	TenantRef string
	Comment   string
	Global    bool
	Force     bool
}

func runInit(stdout, stderr interface{ Write([]byte) (int, error) }, opts initOptions) error {
	path, err := resolvePath(opts.Global)
	if err != nil {
		return err
	}

	if !opts.Force {
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("init: %s already exists; pass --force to overwrite", path)
		} else if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("init: stat %s: %w", path, err)
		}
	}

	w := &wsi.Workspace{
		GibsonURL: opts.GibsonURL,
		TenantRef: opts.TenantRef,
		Comment:   opts.Comment,
	}
	if err := wsi.Save(path, w); err != nil {
		return err
	}

	fmt.Fprintf(stdout, "init: wrote %s\n", path)
	if opts.TenantRef == "" {
		fmt.Fprintf(stderr, "WARN: no --tenant-ref set; subcommands needing tenant context will require --tenant-ref\n")
	}
	return nil
}

func resolvePath(global bool) (string, error) {
	if global {
		return wsi.GlobalPath()
	}
	return wsi.LocalPath(), nil
}
