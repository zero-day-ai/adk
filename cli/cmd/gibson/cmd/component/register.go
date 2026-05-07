package component

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/zero-day-ai/adk/cmd/gibson/internal/component"
	"github.com/zero-day-ai/adk/cmd/gibson/internal/enroll"
	"github.com/zero-day-ai/adk/cmd/gibson/internal/workspace"
)

const envClientSecret = "GIBSON_CLIENT_SECRET"

// registerCmd returns `gibson component register`. Kind-aware:
//
//   - agent / tool: paste the dashboard's --client-id / --client-secret /
//     --gibson-url. Calls enroll.Enroll, writes ~/.gibson/<kind>/credentials.
//   - plugin: paste --token (the bootstrap token from RegisterPlugin).
//     Calls enroll.EnrollPlugin, writes ~/.gibson/plugin/<name>/host_key.
//
// The kind is auto-detected from component.yaml in --dir; pass --kind
// to override.
//
// Per spec R7.9: this verb does NOT call admin RPCs. Identity minting
// is the dashboard's job; this verb only consumes the resulting
// enroll_command.
func registerCmd() *cobra.Command {
	var (
		dir          string
		kindFlag     string
		clientID     string
		clientSecret string
		gibsonURL    string
		nameFlag     string
		token        string
		force        bool
	)

	cmd := &cobra.Command{
		Use:   "register",
		Short: "Consume a dashboard-issued enroll_command (no admin RPC auto-mint)",
		Long: `register performs first-time registration of this component install.

The CLI does NOT mint identity. Run the dashboard's "Register Agent /
Tool / Plugin" wizard first; it returns an enroll_command whose flags
you paste here.

Per kind:

  agent / tool: --client-id, --client-secret, --gibson-url
                Writes ~/.gibson/<kind>/credentials (mode 0600) and
                verifies via OAuth2 client_credentials.

  plugin:       --token <bootstrap-token>
                Runs the SDK capability-grant Bootstrap → Discover →
                Register handshake and persists
                ~/.gibson/plugin/<name>/host_key (mode 0600).

The kind is auto-detected from component.yaml in --dir.

NEVER paste --client-secret into shell history. Use stdin ("-") or set
GIBSON_CLIENT_SECRET in your shell.

Examples:
  gibson component register --client-id 1234567890123456 --client-secret - --gibson-url https://api.zero-day.ai
  gibson component register --token eyJhbGci...
  gibson component register --kind plugin --token eyJhbGci...   # override kind`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRegister(cmd.InOrStdin(), registerOptions{
				Dir:          dir,
				KindFlag:     kindFlag,
				ClientID:     clientID,
				ClientSecret: clientSecret,
				GibsonURL:    gibsonURL,
				Name:         nameFlag,
				Token:        token,
				Force:        force,
			})
		},
	}

	cmd.Flags().StringVarP(&dir, "dir", "d", ".", "component directory (containing component.yaml)")
	cmd.Flags().StringVar(&kindFlag, "kind", "", "override kind (agent | tool | plugin); auto-detected from component.yaml when unset")
	cmd.Flags().StringVar(&clientID, "client-id", "", "OAuth2 client_id (agent / tool only)")
	cmd.Flags().StringVar(&clientSecret, "client-secret", "",
		`OAuth2 client_secret. Use "-" to read from stdin or set GIBSON_CLIENT_SECRET.`)
	cmd.Flags().StringVar(&gibsonURL, "gibson-url", "", "Gibson platform URL; falls back to env / workspace.")
	cmd.Flags().StringVar(&nameFlag, "name", "", "optional install name (allows multiple credentials per kind on one host)")
	cmd.Flags().StringVar(&token, "token", "", "bootstrap token (plugin only)")
	cmd.Flags().BoolVar(&force, "force", false, "overwrite existing credentials with a different client_id")
	return cmd
}

type registerOptions struct {
	Dir          string
	KindFlag     string
	ClientID     string
	ClientSecret string
	GibsonURL    string
	Name         string
	Token        string
	Force        bool
}

func runRegister(stdin io.Reader, opts registerOptions) error {
	c, _, err := loadComponent(opts.Dir)
	if err != nil {
		return err
	}
	kind := c.Kind
	if opts.KindFlag != "" {
		k := component.Kind(opts.KindFlag)
		if !k.Valid() {
			return fmt.Errorf("component register: --kind must be one of agent|tool|plugin, got %q", opts.KindFlag)
		}
		if k != c.Kind {
			return fmt.Errorf("component register: --kind=%s does not match component.yaml kind=%s", k, c.Kind)
		}
		kind = k
	}

	resolvedURL := opts.GibsonURL
	if resolvedURL == "" {
		res, rerr := workspace.Resolve("")
		if rerr == nil {
			resolvedURL = res.GibsonURL
		}
	}

	switch kind {
	case component.KindPlugin:
		return registerPlugin(c, opts, resolvedURL)
	case component.KindAgent, component.KindTool:
		return registerAgentOrTool(stdin, c, kind, opts, resolvedURL)
	}
	return fmt.Errorf("component register: unknown kind %q", kind)
}

func registerPlugin(c *component.Component, opts registerOptions, resolvedURL string) error {
	if opts.Token == "" {
		return errors.New("component register: --token <bootstrap-token> is required for plugin kind (paste from dashboard's Register Plugin wizard)")
	}
	if opts.ClientID != "" || opts.ClientSecret != "" {
		return errors.New("component register: --client-id/--client-secret are agent/tool flags; plugin uses --token")
	}

	// EnrollPlugin reads ./plugin.yaml (or spec.manifest_path) which
	// must be in opts.Dir. Restore cwd if we change it.
	if opts.Dir != "." {
		old, _ := os.Getwd()
		if err := os.Chdir(opts.Dir); err != nil {
			return fmt.Errorf("component register: chdir %s: %w", opts.Dir, err)
		}
		defer os.Chdir(old) //nolint:errcheck
	}

	agentID, err := enroll.EnrollPlugin(context.Background(), enroll.PluginOptions{
		ManifestPath:   c.EffectiveManifestPath(),
		BootstrapToken: opts.Token,
		GibsonURL:      resolvedURL,
	})
	if err != nil {
		return err
	}

	hostKeyPath, _ := enroll.PluginHostKeyPath(c.Metadata.Name)
	if agentID == "" {
		fmt.Printf("component register: already enrolled (host key at %s)\n", hostKeyPath)
	} else {
		fmt.Printf("component register: enrolled %q (agent_id=%s)\n", c.Metadata.Name, agentID)
		fmt.Printf("  host key: %s\n", hostKeyPath)
	}

	return appendStateRecord(opts.Dir, stateRecord{
		Kind:        string(component.KindPlugin),
		PrincipalID: agentID,
		GibsonURL:   resolvedURL,
		EnrolledAt:  time.Now().UTC().Format(time.RFC3339),
	})
}

func registerAgentOrTool(stdin io.Reader, c *component.Component, kind component.Kind, opts registerOptions, resolvedURL string) error {
	if opts.Token != "" {
		return errors.New("component register: --token is plugin-only; agent/tool use --client-id / --client-secret")
	}
	if strings.TrimSpace(opts.ClientID) == "" {
		return errors.New("component register: --client-id is required for agent/tool (paste from dashboard's Register wizard)")
	}
	if resolvedURL == "" {
		return errors.New("component register: --gibson-url is required (or run gibson init / set GIBSON_URL)")
	}

	secret, err := resolveSecret(stdin, opts.ClientSecret)
	if err != nil {
		return err
	}

	credPath, err := enroll.Enroll(context.Background(), enroll.Options{
		Kind:         string(kind),
		ClientID:     opts.ClientID,
		ClientSecret: secret,
		GibsonURL:    resolvedURL,
		Name:         opts.Name,
		Force:        opts.Force,
	})
	if err != nil {
		return err
	}

	fmt.Printf("component register: enrolled %s %q (credentials at %s)\n", kind, c.Metadata.Name, credPath)
	return appendStateRecord(opts.Dir, stateRecord{
		Kind:        string(kind),
		PrincipalID: opts.ClientID, // best we know from a paste-the-creds flow
		GibsonURL:   resolvedURL,
		EnrolledAt:  time.Now().UTC().Format(time.RFC3339),
	})
}

// resolveSecret implements the standard precedence:
//  1. flag value, when not "-" and non-empty
//  2. stdin, when flag value is "-"
//  3. GIBSON_CLIENT_SECRET env var
func resolveSecret(stdin io.Reader, flagValue string) (string, error) {
	switch {
	case flagValue == "-":
		return readSecretFromStdin(stdin)
	case flagValue != "":
		return flagValue, nil
	}
	if v := os.Getenv(envClientSecret); v != "" {
		return v, nil
	}
	return "", errors.New(`component register: --client-secret missing (set the flag, pass "-" for stdin, or export GIBSON_CLIENT_SECRET)`)
}

func readSecretFromStdin(r io.Reader) (string, error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 64*1024)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return "", fmt.Errorf("read stdin: %w", err)
		}
		return "", errors.New(`component register: stdin was empty (--client-secret -)`)
	}
	v := strings.TrimRight(scanner.Text(), "\r\n")
	if v == "" {
		return "", errors.New(`component register: stdin contained an empty line`)
	}
	return v, nil
}

// loadComponent reads dir/component.yaml.
func loadComponent(dir string) (*component.Component, string, error) {
	path := filepath.Join(dir, "component.yaml")
	c, err := component.Load(path)
	if err != nil {
		return nil, "", fmt.Errorf("component register: %w", err)
	}
	return c, path, nil
}

// stateRecord is one entry appended to ./.gibson/state.json on
// successful registration. Per design.md "Model 3" — developer aid only.
type stateRecord struct {
	Kind        string `json:"kind"`
	PrincipalID string `json:"principal_id,omitempty"`
	GibsonURL   string `json:"gibson_url,omitempty"`
	EnrolledAt  string `json:"enrolled_at"`
}

type stateFile struct {
	Registrations []stateRecord `json:"registrations"`
}

func appendStateRecord(dir string, rec stateRecord) error {
	path := filepath.Join(dir, ".gibson", "state.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("component register: create state dir: %w", err)
	}
	var s stateFile
	if b, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(b, &s) // tolerate corrupt/missing
	}
	s.Registrations = append(s.Registrations, rec)
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("component register: marshal state: %w", err)
	}
	return os.WriteFile(path, b, 0o644)
}
