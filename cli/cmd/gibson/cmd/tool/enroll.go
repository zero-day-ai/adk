// Package tool — enroll.go implements `gibson tool enroll`.
// Structurally identical to `gibson agent enroll` (see
// cmd/agent/enroll.go); the only difference is the on-disk path
// (~/.gibson/tool/credentials).
//
// Spec: component-bootstrap-e2e Requirement 4.
package tool

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
		Short: "Write the credentials file for this tool install and verify it",
		Long: `enroll writes the credentials file the dashboard issued and verifies
the OAuth2 client_credentials grant against the daemon's IdP.

The behaviour matches "gibson agent enroll" except the credentials
file is written to ~/.gibson/tool/credentials. See "gibson agent
enroll --help" for the secret-precedence rules and security notes.

NEVER paste --client-secret into shell history. Use stdin or env.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			deprecation.Notify("tool enroll", "component register --kind tool")
			resolved, err := resolveSecret(cmd.InOrStdin(), secret)
			if err != nil {
				return err
			}
			path, err := enroll.Enroll(context.Background(), enroll.Options{
				Kind:         "tool",
				ClientID:     clientID,
				ClientSecret: resolved,
				GibsonURL:    gibsonURL,
				Name:         name,
				Force:        force,
			})
			if err != nil {
				return err
			}
			fmt.Printf("tool enroll: enrolled (credentials at %s)\n", path)
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
		panic(fmt.Sprintf("tool enroll: MarkFlagRequired(%q): %v", name, err))
	}
}

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
	return "", errors.New("tool enroll: --client-secret missing (set the flag, pass \"-\" for stdin, or export GIBSON_CLIENT_SECRET)")
}

func readSecretFromStdin(r io.Reader) (string, error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 64*1024)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return "", fmt.Errorf("read stdin: %w", err)
		}
		return "", errors.New("tool enroll: stdin was empty (--client-secret -)")
	}
	v := strings.TrimRight(scanner.Text(), "\r\n")
	if v == "" {
		return "", errors.New("tool enroll: stdin contained an empty line")
	}
	return v, nil
}
