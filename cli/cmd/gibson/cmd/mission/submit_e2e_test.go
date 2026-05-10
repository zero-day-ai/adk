// Build tag gates this suite behind `go test -tags=e2e` so a
// plain `go test ./...` doesn't try to dial a daemon.
//go:build e2e
// +build e2e

package mission

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
)

// TestSubmit_e2e_recon_template — Spec 3 Task 19.
//
// Drives the full ADK CLI submit path against a running daemon:
//   gibson mission new --from-template recon |
//   gibson mission submit -
//
// Pre-conditions:
//   GIBSON_DAEMON_ADDR  — daemon gRPC endpoint (default
//                         localhost:50002 — matches the kind
//                         deploy in enterprise/deploy/helm/gibson)
//   E2E_GIBSON_E2E      — gate; when unset, the test skips so
//                         CI lanes without a daemon succeed.
//
// The test asserts a non-empty mission ID is returned; the
// daemon's GetMission RPC is not exercised here (that's a
// follow-up integration test in the daemon suite).
func TestSubmit_e2e_recon_template(t *testing.T) {
	if os.Getenv("E2E_GIBSON_E2E") == "" {
		t.Skip("set E2E_GIBSON_E2E=1 to run `gibson mission submit` e2e against a live daemon")
	}
	daemonAddr := os.Getenv("GIBSON_DAEMON_ADDR")
	if daemonAddr == "" {
		daemonAddr = "localhost:50002"
	}

	// Stage 1: render the recon template via `mission new`.
	tplCmd := newCmd()
	tplCmd.SetArgs([]string{"--from-template", "recon"})
	tplOut := &captureWriter{}
	tplCmd.SetOut(tplOut)
	if err := tplCmd.Execute(); err != nil {
		t.Fatalf("mission new: %v", err)
	}
	if !strings.Contains(tplOut.String(), "mission") {
		t.Fatalf("mission new output looks empty: %q", tplOut.String())
	}

	// Stage 2: pipe the rendered CUE through validate to confirm
	// it satisfies the schema before we even contact the daemon.
	tmpFile := writeTmpCUE(t, tplOut.String())
	defer func() { _ = os.Remove(tmpFile) }()
	vCmd := validateCmd()
	vCmd.SetArgs([]string{tmpFile})
	vOut := &captureWriter{}
	vCmd.SetOut(vOut)
	if err := vCmd.Execute(); err != nil {
		t.Fatalf("mission validate: %v", err)
	}
	if !strings.Contains(vOut.String(), "ok") {
		t.Fatalf("mission validate output: %q", vOut.String())
	}

	// Stage 3: submit. Insecure dial only — production submits
	// route through Envoy + ext-authz in the dashboard.
	sCmd := submitCmd()
	sCmd.SetArgs([]string{
		tmpFile,
		"--insecure",
		"--daemon", daemonAddr,
		"--timeout", "30s",
	})
	sOut := &captureWriter{}
	sCmd.SetOut(sOut)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	sCmd.SetContext(ctx)
	if err := sCmd.Execute(); err != nil {
		t.Fatalf("mission submit: %v\noutput: %s", err, sOut.String())
	}
	missionID := strings.TrimSpace(sOut.String())
	if missionID == "" {
		t.Fatal("mission submit returned empty mission ID")
	}
	t.Logf("mission submitted: %s", missionID)
}

// captureWriter is a minimal io.Writer that records every Write
// call so the cobra command's output can be asserted on.
type captureWriter struct {
	buf strings.Builder
}

func (c *captureWriter) Write(p []byte) (int, error) {
	return c.buf.Write(p)
}

func (c *captureWriter) String() string { return c.buf.String() }

// Suppress "imports but not used" for cobra when the body changes.
var _ = cobra.Command{}

func writeTmpCUE(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp("", "gibson-mission-e2e-*.cue")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}
	if _, err := f.WriteString(content); err != nil {
		_ = f.Close()
		t.Fatalf("write: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	return f.Name()
}
