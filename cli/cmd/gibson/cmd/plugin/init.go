package plugin

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/spf13/cobra"

	"github.com/zero-day-ai/adk/cmd/gibson/internal/deprecation"
	"github.com/zero-day-ai/adk/cmd/gibson/internal/scaffold"
)

// reName is the allowed plugin name pattern (DNS-label-ish, 2–63 chars).
// Mirrors the pattern enforced by manifest.Validate.
var reName = regexp.MustCompile(`^[a-z][a-z0-9-]{0,61}[a-z0-9]$`)

// initCmd returns the `gibson plugin init` Cobra command.
func initCmd() *cobra.Command {
	var (
		dir         string
		withSecrets []string
		force       bool
	)

	cmd := &cobra.Command{
		Use:   "init <name>",
		Short: "Scaffold a new plugin directory from templates",
		Long: `init scaffolds a new Gibson plugin directory from embedded templates.

The generated directory contains:
  plugin.yaml   manifest at apiVersion plugin.gibson.zero-day.ai/v1
  main.go       minimal Go main calling plugin.Serve with a stub Echo handler
  <name>.proto  sample EchoRequest / EchoResponse proto definition
  Makefile      targets: proto, build, enroll, run
  Dockerfile    multi-stage CGO_ENABLED=0 build for the pod runtime mode
  .gitignore    excludes host_key and build artifacts
  README.md     four commands to enroll and run

The name must match ^[a-z][a-z0-9-]{0,61}[a-z0-9]$ (DNS-label style).

Examples:
  gibson plugin init my-scanner
  gibson plugin init my-scanner --dir ~/plugins
  gibson plugin init my-scanner --with-secret cred:shodan_key=startup:live
  gibson plugin init my-scanner --with-secret cred:token=startup:live --with-secret cred:db=per_call:restart`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			deprecation.Notify("plugin init", "component init --kind plugin")
			name := args[0]
			return runInit(name, dir, withSecrets, force)
		},
	}

	cmd.Flags().StringVarP(&dir, "dir", "d", "", "destination directory (default: current directory)")
	cmd.Flags().StringArrayVar(&withSecrets, "with-secret", nil, "declare a secret: name=scope:rotation (repeatable)")
	cmd.Flags().BoolVar(&force, "force", false, "overwrite existing files")

	return cmd
}

// runInit implements the init subcommand logic.
func runInit(name, dir string, withSecrets []string, force bool) error {
	// Validate the plugin name.
	if !reName.MatchString(name) {
		return fmt.Errorf(
			"plugin init: name %q must match ^[a-z][a-z0-9-]{0,61}[a-z0-9]$ (DNS-label style)",
			name,
		)
	}

	// Resolve the output directory.
	if dir == "" {
		var err error
		dir, err = os.Getwd()
		if err != nil {
			return fmt.Errorf("plugin init: resolve working directory: %w", err)
		}
	}
	outDir := filepath.Join(dir, name)

	// Parse --with-secret flags.
	secrets := make([]scaffold.SecretInput, 0, len(withSecrets))
	for _, raw := range withSecrets {
		si, err := scaffold.ParseSecretFlag(raw)
		if err != nil {
			return fmt.Errorf("plugin init: %w", err)
		}
		secrets = append(secrets, si)
	}

	// Render all templates.
	input := scaffold.ScaffoldInput{
		Name:    name,
		Version: "0.1.0",
		Kind:    scaffold.KindPlugin,
		Secrets: secrets,
	}
	files, err := scaffold.Render(input)
	if err != nil {
		return fmt.Errorf("plugin init: render templates: %w", err)
	}

	// Create the output directory.
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return fmt.Errorf("plugin init: create output directory %s: %w", outDir, err)
	}

	// Write each rendered file.
	for relPath, content := range files {
		dst := filepath.Join(outDir, relPath)

		// Refuse to overwrite existing files unless --force.
		if !force {
			if _, err := os.Stat(dst); err == nil {
				return fmt.Errorf(
					"plugin init: file %s already exists; use --force to overwrite",
					dst,
				)
			} else if !errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("plugin init: stat %s: %w", dst, err)
			}
		}

		// Ensure subdirectories exist (none currently, but be defensive).
		if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
			return fmt.Errorf("plugin init: create parent directory for %s: %w", dst, err)
		}

		perm := os.FileMode(0644)
		if err := os.WriteFile(dst, content, perm); err != nil {
			return fmt.Errorf("plugin init: write %s: %w", dst, err)
		}
	}

	fmt.Printf("Scaffolded plugin %q in %s\n", name, outDir)
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  cd", name)
	fmt.Println("  make build")
	fmt.Println("  make enroll TOKEN=<bootstrap-token>")
	fmt.Println("  make run")

	return nil
}
