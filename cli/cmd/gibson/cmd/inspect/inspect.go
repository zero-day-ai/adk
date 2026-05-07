// Package inspect implements `gibson inspect`.
//
// inspect loads the local credentials file (auto-detecting agent / tool
// / plugin from the on-disk layout), exchanges for an OAuth2 access
// token, calls the daemon's IdentityService.WhoAmI, and renders the
// caller's effective grants as either a human-friendly tree or
// proto-JSON.
//
// Spec: component-bootstrap-e2e Requirement 11.
package inspect

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/encoding/protojson"

	identitypb "github.com/zero-day-ai/sdk/api/gen/gibson/identity/v1"
)

// Command returns the `inspect` Cobra command.
func Command() *cobra.Command {
	var (
		kind     string
		name     string
		jsonOut  bool
	)
	cmd := &cobra.Command{
		Use:   "inspect",
		Short: "Show what this principal can do (calls WhoAmI on the Gibson daemon)",
		Long: `inspect loads the local credentials at ~/.gibson/<kind>/credentials,
mints an OAuth2 access token, and calls IdentityService.WhoAmI to print
the principal's effective FGA grants.

Auto-detection: when --kind is unset, inspect scans ~/.gibson/{agent,
tool,plugin}/credentials* and, when exactly one credentials file
exists, picks that. Multiple files require --kind (and --name when
multiple of the same kind exist).

Output formats:
  default  human-friendly tree with stable action labels for grep
  --json   raw WhoAmIResponse as canonical proto-JSON (for scripts)`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInspect(context.Background(), kind, name, jsonOut, cmd.OutOrStdout(), cmd.ErrOrStderr())
		},
	}
	cmd.Flags().StringVar(&kind, "kind", "", "agent | tool | plugin (auto-detected when only one exists)")
	cmd.Flags().StringVar(&name, "name", "", "Install name when multiple of the same kind exist")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Emit raw WhoAmIResponse JSON instead of the tree")
	return cmd
}

// credentialsFile is the on-disk shape — must match
// internal/enroll.CredentialsFile.
type credentialsFile struct {
	Issuer       string `json:"issuer"`
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	Audience     string `json:"audience"`
	GibsonURL    string `json:"gibson_url"`
}

func runInspect(ctx context.Context, kind, name string, jsonOut bool, out, errOut interface{}) error {
	stdout := mustWriter(out)
	stderr := mustWriter(errOut)

	credsPath, detectedKind, err := resolveCredentialsPath(kind, name)
	if err != nil {
		return err
	}

	creds, err := loadCredentials(credsPath)
	if err != nil {
		return fmt.Errorf("inspect: load credentials at %s: %w", credsPath, err)
	}

	resp, err := callWhoAmI(ctx, creds)
	if err != nil {
		return fmt.Errorf("inspect: %w", err)
	}

	if jsonOut {
		// Canonical proto-JSON for scripting.
		marshaler := protojson.MarshalOptions{Indent: "  ", UseProtoNames: true}
		b, jerr := marshaler.Marshal(resp)
		if jerr != nil {
			return fmt.Errorf("inspect: marshal json: %w", jerr)
		}
		_, _ = stdout.Write(b)
		_, _ = stdout.Write([]byte("\n"))
		return nil
	}

	renderTree(stdout, resp, detectedKind)
	preflightWarn(stderr, resp, detectedKind)
	return nil
}

// resolveCredentialsPath honours explicit --kind/--name first; falls
// back to auto-detection across the three kinds.
func resolveCredentialsPath(kind, name string) (string, string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", "", fmt.Errorf("resolve home dir: %w", err)
	}

	if kind != "" {
		path := filepath.Join(home, ".gibson", kind, credFilename(name))
		if _, ferr := os.Stat(path); ferr != nil {
			return "", "", fmt.Errorf("inspect: no credentials at %s for --kind %s --name %q", path, kind, name)
		}
		return path, kind, nil
	}

	type found struct{ path, kind string }
	var hits []found
	for _, k := range []string{"agent", "tool", "plugin"} {
		entries, err := os.ReadDir(filepath.Join(home, ".gibson", k))
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			n := e.Name()
			if n == "credentials" || strings.HasPrefix(n, "credentials.") {
				hits = append(hits, found{
					path: filepath.Join(home, ".gibson", k, n),
					kind: k,
				})
			}
		}
	}

	switch len(hits) {
	case 0:
		return "", "", errors.New("inspect: no credentials found under ~/.gibson/{agent,tool,plugin}/ — did you run `gibson <kind> enroll`?")
	case 1:
		return hits[0].path, hits[0].kind, nil
	default:
		var sb strings.Builder
		sb.WriteString("inspect: multiple credentials found; pass --kind (and --name) to disambiguate:\n")
		for _, h := range hits {
			fmt.Fprintf(&sb, "  %s\n", h.path)
		}
		return "", "", errors.New(sb.String())
	}
}

func credFilename(name string) string {
	if name == "" {
		return "credentials"
	}
	return "credentials." + name
}

func loadCredentials(path string) (credentialsFile, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return credentialsFile{}, err
	}
	var c credentialsFile
	if err := json.Unmarshal(b, &c); err != nil {
		return credentialsFile{}, fmt.Errorf("parse credentials JSON: %w", err)
	}
	if c.GibsonURL == "" || c.ClientID == "" {
		return credentialsFile{}, errors.New("credentials file is missing gibson_url or client_id")
	}
	return c, nil
}

func callWhoAmI(ctx context.Context, creds credentialsFile) (*identitypb.WhoAmIResponse, error) {
	cfg := &clientcredentials.Config{
		ClientID:     creds.ClientID,
		ClientSecret: creds.ClientSecret,
		TokenURL:     strings.TrimRight(creds.Issuer, "/") + "/oauth/v2/token",
		Scopes:       []string{"openid"},
	}
	tokenSource := cfg.TokenSource(ctx)

	dialAddr, useTLS, err := dialAddressFromURL(creds.GibsonURL)
	if err != nil {
		return nil, err
	}

	creds_ := []grpc.DialOption{
		grpc.WithPerRPCCredentials(oauthRPCCreds{src: tokenSource}),
	}
	if useTLS {
		creds_ = append(creds_, grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12})))
	} else {
		creds_ = append(creds_, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	conn, err := grpc.NewClient(dialAddr, creds_...)
	if err != nil {
		return nil, fmt.Errorf("could not dial Gibson at %s: %w", creds.GibsonURL, err)
	}
	defer conn.Close()

	client := identitypb.NewIdentityServiceClient(conn)
	resp, err := client.WhoAmI(ctx, &identitypb.WhoAmIRequest{})
	if err != nil {
		return nil, fmt.Errorf("WhoAmI failed: %w", err)
	}
	return resp, nil
}

// dialAddressFromURL extracts the host:port for grpc.Dial and reports
// whether TLS is required. Defaults: https→443, http→80.
func dialAddressFromURL(s string) (string, bool, error) {
	u, err := url.Parse(s)
	if err != nil {
		return "", false, fmt.Errorf("parse gibson_url: %w", err)
	}
	host := u.Host
	if host == "" {
		return "", false, fmt.Errorf("gibson_url has no host: %s", s)
	}
	if !strings.Contains(host, ":") {
		switch u.Scheme {
		case "http":
			host += ":80"
		default:
			host += ":443"
		}
	}
	return host, u.Scheme == "https", nil
}

// oauthRPCCreds adapts an oauth2.TokenSource to gRPC PerRPCCredentials.
type oauthRPCCreds struct{ src oauth2.TokenSource }

func (c oauthRPCCreds) GetRequestMetadata(ctx context.Context, _ ...string) (map[string]string, error) {
	tok, err := c.src.Token()
	if err != nil {
		return nil, fmt.Errorf("mint OAuth2 token: %w", err)
	}
	return map[string]string{"authorization": "Bearer " + tok.AccessToken}, nil
}
func (oauthRPCCreds) RequireTransportSecurity() bool { return false }

// renderTree prints the human-friendly view. Action labels (read,
// write, execute) are fixed strings to keep grep-based parsing stable.
func renderTree(out interface{ Write([]byte) (int, error) }, resp *identitypb.WhoAmIResponse, kind string) {
	fmt.Fprintf(stdoutWriter(out), "%s (kind=%s, principal_id=%s)\n",
		resp.GetName(), strings.ToLower(strings.TrimPrefix(resp.GetKind().String(), "PRINCIPAL_KIND_")), resp.GetPrincipalId())
	fmt.Fprintf(stdoutWriter(out), "  tenant: %s\n", resp.GetTenantId())

	fmt.Fprintln(stdoutWriter(out), "  components:")
	if len(resp.GetComponentGrants()) == 0 {
		fmt.Fprintln(stdoutWriter(out), "    (none)")
	} else {
		grants := append([]*identitypb.ComponentGrantEffective(nil), resp.GetComponentGrants()...)
		sort.Slice(grants, func(i, j int) bool { return grants[i].GetComponentRef() < grants[j].GetComponentRef() })
		maxName := 0
		for _, g := range grants {
			if l := len(g.GetComponentRef()); l > maxName {
				maxName = l
			}
		}
		for _, g := range grants {
			actions := []string{}
			if g.GetCanRead() {
				actions = append(actions, "read")
			}
			if g.GetCanConfigure() {
				actions = append(actions, "write")
			}
			if g.GetCanExecute() {
				actions = append(actions, "execute")
			}
			source := "direct"
			if len(g.GetSources()) > 0 {
				source = sourceLabel(g.GetSources()[0])
			}
			fmt.Fprintf(stdoutWriter(out), "    %-*s  %s  (%s)\n",
				maxName, g.GetComponentRef(), strings.Join(actions, " "), source)
		}
	}

	fmt.Fprintln(stdoutWriter(out), "  plugins:")
	switch {
	case kind == "agent":
		fmt.Fprintln(stdoutWriter(out), "    (none — agents do not invoke plugins directly)")
	case len(resp.GetPluginGrants()) == 0:
		fmt.Fprintln(stdoutWriter(out), "    (none)")
	default:
		for _, p := range resp.GetPluginGrants() {
			fmt.Fprintf(stdoutWriter(out), "    %s\n", p.GetPluginRef())
		}
	}

	fmt.Fprintf(stdoutWriter(out), "  active capability grants: %d\n", len(resp.GetActiveCapabilityGrants()))
}

func sourceLabel(s *identitypb.GrantSource) string {
	switch s.GetKind() {
	case identitypb.GrantSource_KIND_DIRECT:
		return "direct"
	case identitypb.GrantSource_KIND_TENANT_MEMBER:
		return fmt.Sprintf("via %s#member", s.GetSourceObject())
	case identitypb.GrantSource_KIND_TEAM_MEMBER:
		return fmt.Sprintf("via %s#member", s.GetSourceObject())
	case identitypb.GrantSource_KIND_OWNER:
		return fmt.Sprintf("via %s#owner", s.GetSourceObject())
	default:
		return "unknown"
	}
}

// preflightWarn surfaces the "this agent has effectively no grants"
// warning per Requirement 11.5.
func preflightWarn(stderr interface{ Write([]byte) (int, error) }, resp *identitypb.WhoAmIResponse, kind string) {
	if len(resp.GetComponentGrants()) > 0 {
		return
	}
	if len(resp.GetPluginGrants()) > 0 {
		return
	}
	fmt.Fprintf(stdoutWriter(stderr),
		"WARN: this %s has no direct grants and only inherits tenant-member access; check the registration step\n",
		kind)
}

// mustWriter is a small assertion that the cobra-passed writer is
// usable. We avoid importing io.Writer at the public boundary to keep
// the test surface small.
func mustWriter(v interface{}) interface{ Write([]byte) (int, error) } {
	w, ok := v.(interface{ Write([]byte) (int, error) })
	if !ok {
		// Fall back to stderr-shaped writer; main always passes a
		// real io.Writer, this branch is purely defensive.
		return os.Stderr
	}
	return w
}

func stdoutWriter(v interface{ Write([]byte) (int, error) }) interface{ Write([]byte) (int, error) } {
	return v
}

