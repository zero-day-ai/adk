package enroll

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/zero-day-ai/sdk/capabilitygrant"
	"github.com/zero-day-ai/sdk/plugin/manifest"
)

// PluginOptions carries everything EnrollPlugin needs.
type PluginOptions struct {
	// ManifestPath is the path to plugin.yaml. Defaults to "./plugin.yaml"
	// when empty.
	ManifestPath string

	// BootstrapToken is the single-use enrolment token issued by
	// PluginsAdminService.RegisterPlugin via the dashboard.
	BootstrapToken string

	// GibsonURL overrides the GIBSON_URL env var when non-empty.
	GibsonURL string

	// CGFactory builds the capabilitygrant client. Tests inject a stub.
	// Production callers leave this nil.
	CGFactory CGClientFactory

	// HTTPClient overrides the HTTP client used for registration.
	// Tests inject a TLS-aware client for httptest.Server. Production
	// callers leave this nil.
	HTTPClient *http.Client
}

// CGClient is the subset of capabilitygrant.Client EnrollPlugin uses.
// Exposed as an interface to allow stub injection in tests.
type CGClient interface {
	Discover(ctx context.Context) error
	Register(ctx context.Context) error
	AgentID() string
}

// CGClientFactory builds a CGClient. Tests inject a fake to avoid
// dialing a real platform.
type CGClientFactory func(cfg capabilitygrant.ClientConfig) (CGClient, error)

// DefaultCGClientFactory is the production factory.
var DefaultCGClientFactory CGClientFactory = func(cfg capabilitygrant.ClientConfig) (CGClient, error) {
	return capabilitygrant.NewClient(cfg)
}

// PluginHostKeyPath returns ~/.gibson/plugin/<name>/host_key.
func PluginHostKeyPath(name string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("plugin enroll: resolve home dir: %w", err)
	}
	return filepath.Join(home, ".gibson", "plugin", name, "host_key"), nil
}

// EnrollPlugin performs first-time plugin registration via the
// SDK's capability-grant Bootstrap → Discover → Register handshake.
//
//  1. Load plugin.yaml to extract metadata.name.
//  2. If ~/.gibson/plugin/<name>/host_key already exists, return success
//     (idempotent).
//  3. Resolve GibsonURL from opts then GIBSON_URL env. Empty → error.
//  4. Build CGClient, Discover + Register, persist host key (mode 0600).
//
// Returns the daemon-issued agent_id on success.
func EnrollPlugin(ctx context.Context, opts PluginOptions) (string, error) {
	manifestPath := opts.ManifestPath
	if manifestPath == "" {
		manifestPath = "./plugin.yaml"
	}
	factory := opts.CGFactory
	if factory == nil {
		factory = DefaultCGClientFactory
	}

	m, err := manifest.Load(manifestPath)
	if err != nil {
		return "", fmt.Errorf("plugin enroll: load manifest: %w", err)
	}

	hostKeyPath, err := PluginHostKeyPath(m.Metadata.Name)
	if err != nil {
		return "", err
	}

	// Idempotency: if the host key already exists, success without
	// re-handshaking. Re-running enroll is safe.
	if _, err := os.Stat(hostKeyPath); err == nil {
		return "", nil
	}

	platformURL := opts.GibsonURL
	if platformURL == "" {
		platformURL = os.Getenv("GIBSON_URL")
	}
	if platformURL == "" {
		return "", fmt.Errorf("plugin enroll: GIBSON_URL is required (set to the Gibson platform base URL)")
	}

	if err := os.MkdirAll(filepath.Dir(hostKeyPath), 0o700); err != nil {
		return "", fmt.Errorf("plugin enroll: create host key directory: %w", err)
	}

	cfg := capabilitygrant.ClientConfig{
		PlatformURL:    platformURL,
		BootstrapToken: opts.BootstrapToken,
		HostKeyPath:    hostKeyPath,
		AgentName:      m.Metadata.Name,
		AgentMode:      "autonomous",
	}

	client, err := factory(cfg)
	if err != nil {
		return "", fmt.Errorf("plugin enroll: capabilitygrant.NewClient: %w", err)
	}

	if opts.HTTPClient != nil {
		if concrete, ok := client.(*capabilitygrant.Client); ok {
			concrete.SetHTTPClient(opts.HTTPClient)
		}
	}

	if err := client.Discover(ctx); err != nil {
		return "", fmt.Errorf("plugin enroll: discover: %w", err)
	}

	if err := client.Register(ctx); err != nil {
		return "", fmt.Errorf("plugin enroll: register: %w", err)
	}

	// Defence-in-depth: ensure 0600 even though SaveHostKey already does.
	if err := os.Chmod(hostKeyPath, 0o600); err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("plugin enroll: chmod host key: %w", err)
	}

	return client.AgentID(), nil
}
