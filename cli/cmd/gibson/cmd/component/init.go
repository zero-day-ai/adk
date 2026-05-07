package component

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime/debug"

	"github.com/spf13/cobra"

	"github.com/zero-day-ai/adk/cmd/gibson/internal/scaffold"
)

// nameRegex enforces the DNS-label-style name regex used by the
// scaffold and SDK manifest validator.
var nameRegex = regexp.MustCompile(`^[a-z][a-z0-9-]{0,61}[a-z0-9]$`)

// initCmd returns the `gibson component init` cobra command.
func initCmd() *cobra.Command {
	var (
		kind        string
		dir         string
		withSecrets []string
		force       bool
	)

	cmd := &cobra.Command{
		Use:   "init <name>",
		Short: "Scaffold a new component directory (agent | tool | plugin)",
		Long: `init scaffolds a new Gibson component directory from embedded
templates. The kind selects which template set to render and what file
shape the new directory has:

  agent:  component.yaml + main.go (sdk.NewAgent + sdk.ServeAgent) +
          go.mod + Makefile + Dockerfile + README + AGENTS.md +
          CLAUDE.md + prompts/ + .claude/settings.json

  tool:   like agent, plus api/proto/<name>/v1/<name>.proto with
          field 100 = gibson.graphrag.v1.DiscoveryResult, buf.yaml,
          buf.gen.yaml, and proto/vendor/ for the SDK protos

  plugin: plugin.yaml manifest + main.go (plugin.Serve + Echo handler)
          + .proto + Makefile + Dockerfile + README + AGENTS.md +
          CLAUDE.md + prompts/ + .claude/settings.json

The name must match ^[a-z][a-z0-9-]{0,61}[a-z0-9]$ (DNS-label style).

--with-secret is plugin-only. agent and tool kinds do not declare
broker secrets.

Examples:
  gibson component init my-agent --kind agent
  gibson component init my-scanner --kind tool
  gibson component init my-plugin --kind plugin --with-secret cred:api_key=startup:live
  gibson component init my-plugin --kind plugin --dir ~/projects --force`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInit(args[0], kind, dir, withSecrets, force)
		},
	}

	cmd.Flags().StringVar(&kind, "kind", "", "component kind: agent | tool | plugin (required)")
	cmd.Flags().StringVarP(&dir, "dir", "d", "", "destination directory (default: current directory)")
	cmd.Flags().StringArrayVar(&withSecrets, "with-secret", nil, "plugin-only: declare a secret name=scope:rotation (repeatable)")
	cmd.Flags().BoolVar(&force, "force", false, "overwrite existing files")
	if err := cmd.MarkFlagRequired("kind"); err != nil {
		panic("component init: MarkFlagRequired(kind): " + err.Error())
	}
	return cmd
}

// runInit is the public-from-this-package implementation. Exported as
// RunInit so the back-compat plugin shim can delegate without
// duplicating logic.
func runInit(name, kindStr, dir string, withSecrets []string, force bool) error {
	if !nameRegex.MatchString(name) {
		return fmt.Errorf("component init: name %q must match ^[a-z][a-z0-9-]{0,61}[a-z0-9]$ (DNS-label style)", name)
	}
	k := scaffold.Kind(kindStr)
	if !k.Valid() {
		return fmt.Errorf("component init: --kind must be one of agent|tool|plugin, got %q", kindStr)
	}

	if dir == "" {
		var err error
		dir, err = os.Getwd()
		if err != nil {
			return fmt.Errorf("component init: resolve working directory: %w", err)
		}
	}
	outDir := filepath.Join(dir, name)

	secrets := make([]scaffold.SecretInput, 0, len(withSecrets))
	for _, raw := range withSecrets {
		si, err := scaffold.ParseSecretFlag(raw)
		if err != nil {
			return fmt.Errorf("component init: %w", err)
		}
		secrets = append(secrets, si)
	}

	files, err := scaffold.Render(scaffold.ScaffoldInput{
		Name:       name,
		Version:    "0.1.0",
		Kind:       k,
		Secrets:    secrets,
		SDKVersion: resolveSDKVersion(),
	})
	if err != nil {
		return fmt.Errorf("component init: render templates: %w", err)
	}

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("component init: create output directory %s: %w", outDir, err)
	}

	for relPath, content := range files {
		dst := filepath.Join(outDir, relPath)
		if !force {
			if _, err := os.Stat(dst); err == nil {
				return fmt.Errorf("component init: file %s already exists; use --force to overwrite", dst)
			} else if !errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("component init: stat %s: %w", dst, err)
			}
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return fmt.Errorf("component init: create parent directory for %s: %w", dst, err)
		}
		if err := os.WriteFile(dst, content, 0o644); err != nil {
			return fmt.Errorf("component init: write %s: %w", dst, err)
		}
	}

	fmt.Printf("Scaffolded %s %q in %s\n", k, name, outDir)
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Printf("  cd %s\n", name)
	fmt.Println("  cat AGENTS.md            # the contract this scaffold implements")
	if k == scaffold.KindTool || k == scaffold.KindPlugin {
		fmt.Println("  make proto               # generate Go bindings")
	}
	fmt.Println("  make build")
	if k == scaffold.KindPlugin {
		fmt.Println("  gibson component register --token <bootstrap-token>")
	} else {
		fmt.Println("  gibson component register --client-id <id> --client-secret - --gibson-url <url>")
	}
	fmt.Println("  gibson component run")
	return nil
}

// RunInit is an exported entry-point for the back-compat plugin shim
// (cmd/plugin/init.go) so it can dispatch to component init without
// importing private symbols.
func RunInit(name, kindStr, dir string, withSecrets []string, force bool) error {
	return runInit(name, kindStr, dir, withSecrets, force)
}

// resolveSDKVersion reads the SDK version from go.mod via
// runtime/debug.ReadBuildInfo. Falls back to "v1.2.0" (the version that
// introduced post-scaffold-removal SDK) if BuildInfo is unavailable
// (e.g. during `go run`). Pinning at compile time guarantees the
// scaffold's go.mod points at a version of the SDK the ADK has tested
// against.
func resolveSDKVersion() string {
	const fallback = "v1.2.0"
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return fallback
	}
	for _, d := range info.Deps {
		if d.Path == "github.com/zero-day-ai/sdk" {
			if d.Replace != nil && d.Replace.Version != "" {
				return d.Replace.Version
			}
			if d.Version != "" {
				return d.Version
			}
		}
	}
	return fallback
}
