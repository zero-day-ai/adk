package enroll

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// stubIdP serves a minimal OIDC discovery document and an OAuth2
// token endpoint. Test calls switch the body via fields.
type stubIdP struct {
	server      *httptest.Server
	tokenStatus int
	tokenBody   string
}

func newStubIdP(t *testing.T) *stubIdP {
	t.Helper()
	s := &stubIdP{tokenStatus: http.StatusOK, tokenBody: `{"access_token":"a","token_type":"Bearer","expires_in":3600}`}
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		// Self-issuer: tests rewrite both the issuer and the token URL
		// to point back at this server.
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"issuer":"` + s.server.URL + `","audiences":["gibson-platform"]}`))
	})
	mux.HandleFunc("/oauth/v2/token", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(s.tokenStatus)
		_, _ = w.Write([]byte(s.tokenBody))
	})
	s.server = httptest.NewTLSServer(mux)
	t.Cleanup(s.server.Close)
	return s
}

// httpClientForTLS returns a client that accepts the stub server's
// self-signed cert.
func httpClientForTLS(s *httptest.Server) *http.Client {
	return &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}}
}

func writeOpts(t *testing.T, idp *stubIdP) Options {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	return Options{
		Kind:         "agent",
		ClientID:     "client-1",
		ClientSecret: "supersecret",
		GibsonURL:    idp.server.URL,
		HTTPClient:   httpClientForTLS(idp.server),
	}
}

func TestEnroll_HappyPath_Writes0600AndVerifies(t *testing.T) {
	idp := newStubIdP(t)
	opts := writeOpts(t, idp)

	path, err := Enroll(context.Background(), opts)
	if err != nil {
		t.Fatalf("Enroll: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat credentials: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Errorf("file mode = %v, want 0600", got)
	}

	b, _ := os.ReadFile(path)
	var c CredentialsFile
	if err := json.Unmarshal(b, &c); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if c.ClientID != "client-1" || c.ClientSecret != "supersecret" {
		t.Errorf("credentials mismatch: %+v", c)
	}
	if c.Audience != "gibson-platform" {
		t.Errorf("audience = %q, want gibson-platform", c.Audience)
	}
	// issuer should be set to the IdP's URL
	if u, err := url.Parse(c.Issuer); err != nil || u.Host == "" {
		t.Errorf("issuer not a valid URL: %q", c.Issuer)
	}
}

func TestEnroll_RefusesOverwriteWithoutForce(t *testing.T) {
	idp := newStubIdP(t)
	opts := writeOpts(t, idp)

	if _, err := Enroll(context.Background(), opts); err != nil {
		t.Fatalf("first Enroll: %v", err)
	}

	// Same client_id is OK (idempotent)
	if _, err := Enroll(context.Background(), opts); err != nil {
		t.Errorf("second Enroll same client_id: %v", err)
	}

	// Different client_id without --force should fail
	opts.ClientID = "client-2"
	_, err := Enroll(context.Background(), opts)
	if err == nil {
		t.Fatal("expected error overwriting with different client_id; got nil")
	}
	if !strings.Contains(err.Error(), "refusing to overwrite") {
		t.Errorf("unexpected error: %v", err)
	}

	// With --force: succeeds
	opts.Force = true
	if _, err := Enroll(context.Background(), opts); err != nil {
		t.Errorf("Enroll with --force: %v", err)
	}
}

func TestEnroll_TokenExchange401SurfacesIdPRejection(t *testing.T) {
	idp := newStubIdP(t)
	idp.tokenStatus = http.StatusUnauthorized
	idp.tokenBody = `{"error":"invalid_client"}`
	opts := writeOpts(t, idp)

	_, err := Enroll(context.Background(), opts)
	if err == nil {
		t.Fatal("expected error on 401; got nil")
	}
	if !strings.Contains(err.Error(), "rejected by IdP") {
		t.Errorf("expected IdP-rejection wording in error, got: %v", err)
	}
	// Credentials should NOT remain on disk after a failed verify.
	credPath, _ := CredentialsPath("agent", "")
	credPath = filepath.Join(os.Getenv("HOME"), ".gibson", "agent", "credentials")
	if _, err := os.Stat(credPath); err == nil {
		t.Errorf("credentials file should have been rolled back after verify failure; still at %s", credPath)
	}
}

func TestEnroll_RejectsBadKind(t *testing.T) {
	_, err := Enroll(context.Background(), Options{Kind: "plugin"})
	if err == nil {
		t.Fatal("expected error for kind=plugin (this helper is agent/tool only)")
	}
}
