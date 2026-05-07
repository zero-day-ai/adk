package enroll_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zero-day-ai/sdk/capabilitygrant"

	"github.com/zero-day-ai/adk/cmd/gibson/internal/enroll"
)

// buildMockPlatform builds a minimal httptest.TLS server that serves the
// capabilitygrant discovery and registration endpoints.
func buildMockPlatform(t *testing.T) *httptest.Server {
	t.Helper()

	mux := http.NewServeMux()
	var srvURL string

	mux.HandleFunc("/.well-known/agent-configuration", func(w http.ResponseWriter, r *http.Request) {
		doc := map[string]any{
			"protocol_version": "1.0",
			"provider_name":    "Test Platform",
			"issuer":           r.Host,
			"default_location": "us-east-1",
			"supported_modes":  []string{"delegated", "autonomous"},
			"endpoints": map[string]string{
				"register":   srvURL + "/agent-auth/register",
				"execute":    "/agent-auth/execute",
				"list":       "/agent-auth/agents",
				"status":     "/agent-auth/status",
				"revoke":     "/agent-auth/revoke",
				"introspect": "/agent-auth/introspect",
			},
			"jwks_uri": "/agent-auth/.well-known/jwks.json",
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(doc)
	})

	mux.HandleFunc("/agent-auth/register", func(w http.ResponseWriter, r *http.Request) {
		var reqBody struct {
			HostID string `json:"host_id"`
		}
		_ = json.NewDecoder(r.Body).Decode(&reqBody)
		agentID := "test-agent-id"
		if reqBody.HostID != "" {
			agentID = "agent-" + reqBody.HostID[:minInt(8, len(reqBody.HostID))]
		}
		resp := map[string]any{
			"agent_id":        agentID,
			"capabilities":    []any{},
			"component_scope": "component:" + agentID,
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(resp)
	})

	srv := httptest.NewTLSServer(mux)
	srvURL = srv.URL
	t.Cleanup(srv.Close)
	return srv
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// stubCGClient is a test double for enroll.CGClient that records calls.
type stubCGClient struct {
	discoverCalled int
	registerCalled int
	agentIDVal     string
	discoverErr    error
	registerErr    error
}

func (s *stubCGClient) Discover(_ context.Context) error {
	s.discoverCalled++
	return s.discoverErr
}

func (s *stubCGClient) Register(_ context.Context) error {
	s.registerCalled++
	return s.registerErr
}

func (s *stubCGClient) AgentID() string { return s.agentIDVal }

// writeValidManifest writes a minimal valid plugin.yaml to dir.
// protoName is the name used in proto FQN (must be all lowercase letters,
// no hyphens). The plugin name is separate and may contain hyphens.
func writeValidManifest(t *testing.T, dir, pluginName, protoName string) string {
	t.Helper()
	content := `apiVersion: plugin.gibson.zero-day.ai/v1
kind: Plugin
metadata:
  name: ` + pluginName + `
  version: 0.1.0
spec:
  workload_class: plugin
  methods:
  - name: Echo
    request_proto: gibson.plugins.` + protoName + `.v1.EchoRequest
    response_proto: gibson.plugins.` + protoName + `.v1.EchoResponse
`
	path := filepath.Join(dir, "plugin.yaml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	return path
}

func TestEnrollPlugin_HostKeyWrittenWithCorrectMode(t *testing.T) {
	srv := buildMockPlatform(t)

	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	manifestDir := t.TempDir()
	manifestPath := writeValidManifest(t, manifestDir, "enroll-test", "enrolltest")

	factory := func(cfg capabilitygrant.ClientConfig) (enroll.CGClient, error) {
		client, err := capabilitygrant.NewClient(cfg)
		if err != nil {
			return nil, err
		}
		client.SetHTTPClient(srv.Client())
		client.PatchDiscoveryRegisterURL(srv.URL + "/agent-auth/register")
		return client, nil
	}

	_, err := enroll.EnrollPlugin(context.Background(), enroll.PluginOptions{
		ManifestPath:   manifestPath,
		BootstrapToken: "test-bootstrap-token",
		GibsonURL:      srv.URL,
		CGFactory:      factory,
	})
	require.NoError(t, err)

	expectedKeyPath := filepath.Join(homeDir, ".gibson", "plugin", "enroll-test", "host_key")
	info, err := os.Stat(expectedKeyPath)
	require.NoError(t, err, "host key must exist at %s", expectedKeyPath)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm(), "host key must have mode 0600")
}

func TestEnrollPlugin_IdempotentRerun(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	manifestDir := t.TempDir()
	manifestPath := writeValidManifest(t, manifestDir, "idempotent-test", "idempotenttest")

	stub := &stubCGClient{agentIDVal: "stub-agent"}
	factory := func(cfg capabilitygrant.ClientConfig) (enroll.CGClient, error) {
		return stub, nil
	}

	keyPath := filepath.Join(homeDir, ".gibson", "plugin", "idempotent-test", "host_key")
	require.NoError(t, os.MkdirAll(filepath.Dir(keyPath), 0o700))
	require.NoError(t, os.WriteFile(keyPath, []byte("fake-key"), 0o600))

	_, err := enroll.EnrollPlugin(context.Background(), enroll.PluginOptions{
		ManifestPath:   manifestPath,
		BootstrapToken: "token",
		GibsonURL:      "https://localhost",
		CGFactory:      factory,
	})
	require.NoError(t, err)

	assert.Equal(t, 0, stub.discoverCalled, "Discover must not be called when already enrolled")
	assert.Equal(t, 0, stub.registerCalled, "Register must not be called when already enrolled")
}

func TestEnrollPlugin_MissingGibsonURL(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("GIBSON_URL", "")

	manifestDir := t.TempDir()
	manifestPath := writeValidManifest(t, manifestDir, "url-test", "urltest")

	stub := &stubCGClient{agentIDVal: "stub-agent"}
	factory := func(cfg capabilitygrant.ClientConfig) (enroll.CGClient, error) {
		return stub, nil
	}

	_, err := enroll.EnrollPlugin(context.Background(), enroll.PluginOptions{
		ManifestPath:   manifestPath,
		BootstrapToken: "token",
		CGFactory:      factory,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "GIBSON_URL")
}

func TestEnrollPlugin_MissingManifest(t *testing.T) {
	stub := &stubCGClient{}
	factory := func(cfg capabilitygrant.ClientConfig) (enroll.CGClient, error) {
		return stub, nil
	}

	_, err := enroll.EnrollPlugin(context.Background(), enroll.PluginOptions{
		ManifestPath:   "/nonexistent/plugin.yaml",
		BootstrapToken: "token",
		GibsonURL:      "https://localhost",
		CGFactory:      factory,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "plugin enroll")
}
