// Package agent — enroll.go implements `gibson agent enroll`.
//
// The dashboard's CreateAgentIdentity flow returns an OAuth2 client_id +
// client_secret + gibson_url + enroll_command. The admin pastes the
// enroll_command on the agent host; this command writes the credentials
// to ~/.gibson/agent/credentials and verifies them.
//
// Spec: component-bootstrap-e2e Requirement 3.
package agent

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/zero-day-ai/adk/cmd/gibson/internal/deprecation"
	"github.com/zero-day-ai/adk/cmd/gibson/internal/enroll"
)

// envClientSecret is the env var the CLI reads when --client-secret
// is unset. Spec: component-bootstrap-e2e Requirement 3.6.
const envClientSecret = "GIBSON_CLIENT_SECRET"

func enrollCmd() *cobra.Command {
	var (
		clientID  string
		secret    string
		gibsonURL string
		name      string
		force     bool
	)

	cmd := &cobra.Command{
		Use:   "enroll",
		Short: "Write the credentials file for this agent install and verify it",
		Long: `enroll writes the credentials file the dashboard issued and verifies
the OAuth2 client_credentials grant against the daemon's IdP.

Credential precedence for --client-secret:
  1. --client-secret <value>     (visible in process listing — avoid on shared hosts)
  2. --client-secret -           (read from stdin; recommended)
  3. GIBSON_CLIENT_SECRET env    (set in the agent's systemd unit / pod env)

The credentials file is written to ~/.gibson/agent/credentials with mode
0600. If a credentials file already exists with a different client_id,
this command refuses to overwrite without --force.

After writing, the command verifies the credential by performing an
OAuth2 client_credentials token exchange against the daemon's OIDC
issuer (resolved via GIBSON_URL/.well-known/openid-configuration).
A 401 means the IdP rejected the credential; anything else is a
connectivity issue.

NEVER paste --client-secret into shell history. Use stdin or env.

Example:
  export GIBSON_URL=https://api.zero-day.ai
  export GIBSON_CLIENT_SECRET="..."
  gibson agent enroll --client-id 1234567890123456 --gibson-url $GIBSON_URL`,
		RunE: func(cmd *cobra.Command, args []string) error {
			deprecation.Notify("agent enroll", "component register --kind agent")
			resolved, err := resolveSecret(cmd.InOrStdin(), secret)
			if err != nil {
				return err
			}
			path, err := enroll.Enroll(context.Background(), enroll.Options{
				Kind:         "agent",
				ClientID:     clientID,
				ClientSecret: resolved,
				GibsonURL:    gibsonURL,
				Name:         name,
				Force:        force,
			})
			if err != nil {
				return err
			}
			fmt.Printf("agent enroll: enrolled (credentials at %s)\n", path)
			return nil
		},
	}

	cmd.Flags().StringVar(&clientID, "client-id", "", "OAuth2 client_id (required)")
	cmd.Flags().StringVar(&secret, "client-secret", "",
		`OAuth2 client_secret. Use "-" to read from stdin or set GIBSON_CLIENT_SECRET.`)
	cmd.Flags().StringVar(&gibsonURL, "gibson-url", "",
		"Gibson platform URL (e.g. https://api.zero-day.ai). Required.")
	cmd.Flags().StringVar(&name, "name", "",
		"Optional install name; allows multiple credentials per kind on one host.")
	cmd.Flags().BoolVar(&force, "force", false,
		"Overwrite an existing credentials file with a different client_id.")
	mustMark(cmd, "client-id")
	mustMark(cmd, "gibson-url")
	return cmd
}

func mustMark(cmd *cobra.Command, name string) {
	if err := cmd.MarkFlagRequired(name); err != nil {
		panic(fmt.Sprintf("agent enroll: MarkFlagRequired(%q): %v", name, err))
	}
}

// resolveSecret implements the precedence:
//  1. flag value, when not "-" and non-empty
//  2. stdin, when flag value is "-"
//  3. GIBSON_CLIENT_SECRET env var
//
// Returns an error when none yield a non-empty value.
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
	return "", errors.New("agent enroll: --client-secret missing (set the flag, pass \"-\" for stdin, or export GIBSON_CLIENT_SECRET)")
}

// readSecretFromStdin reads exactly one line (trim CR/LF) and returns
// it. Empty stdin is an error.
func readSecretFromStdin(r io.Reader) (string, error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 64*1024)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return "", fmt.Errorf("read stdin: %w", err)
		}
		return "", errors.New("agent enroll: stdin was empty (--client-secret -)")
	}
	v := strings.TrimRight(scanner.Text(), "\r\n")
	if v == "" {
		return "", errors.New("agent enroll: stdin contained an empty line")
	}
	return v, nil
}
