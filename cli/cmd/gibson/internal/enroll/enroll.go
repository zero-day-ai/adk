// Package enroll implements the shared enrollment logic backing
// `gibson agent enroll` and `gibson tool enroll`.
//
// The two CLI surfaces are structurally identical — they both write a
// JSON credentials file at `~/.gibson/<kind>/credentials` (mode 0600),
// then verify the credentials by performing an OAuth2 client_credentials
// token exchange against the daemon's OIDC issuer. This file
// contains both flows; the per-kind cobra commands are thin wrappers
// that delegate here.
//
// Spec: component-bootstrap-e2e Requirements 3 and 4.
package enroll

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/oauth2/clientcredentials"
)

// VerifyTimeout bounds the OAuth2 token-exchange verification step.
// Beyond this the operator should investigate the network path or IdP
// reachability rather than wait.
const VerifyTimeout = 30 * time.Second

// Options carries everything Enroll needs to write a credentials file
// and verify it. Kind is "agent" or "tool"; other values are an error.
type Options struct {
	// Kind is "agent" or "tool". Drives the on-disk path.
	Kind string

	// ClientID is the OAuth2 client_id from the dashboard's
	// CreateAgentIdentity response.
	ClientID string

	// ClientSecret is the matching client_secret. NEVER log this.
	ClientSecret string

	// GibsonURL is the daemon's public Envoy URL. Used to bootstrap
	// the issuer/audience values via OIDC discovery and as the dial
	// target for ongoing RPCs.
	GibsonURL string

	// Name disambiguates multi-credential installs. When empty, the
	// canonical credentials path is used; when set, ".<name>" is
	// appended to the filename.
	Name string

	// Force overrides the idempotency check that refuses to overwrite
	// a credentials file with a different client_id.
	Force bool

	// HTTPClient is an injection point for tests. Production paths
	// pass nil and the helper uses http.DefaultClient.
	HTTPClient *http.Client
}

// CredentialsFile is the on-disk shape of `~/.gibson/<kind>/credentials`.
// It mirrors what core/sdk/auth/oidc.LoadAgentCredentials expects
// (verified at runtime when the SDK reads it).
type CredentialsFile struct {
	Issuer       string `json:"issuer"`
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	Audience     string `json:"audience"`
	GibsonURL    string `json:"gibson_url"`
}

// CredentialsPath returns the on-disk path for this kind/name pair.
// The canonical form is `~/.gibson/<kind>/credentials`; appending a
// non-empty name yields `~/.gibson/<kind>/credentials.<name>` for
// multi-install hosts.
func CredentialsPath(kind, name string) (string, error) {
	if kind != "agent" && kind != "tool" {
		return "", fmt.Errorf("enroll: kind must be \"agent\" or \"tool\" (got %q)", kind)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("enroll: resolve home dir: %w", err)
	}
	filename := "credentials"
	if name != "" {
		filename = "credentials." + name
	}
	return filepath.Join(home, ".gibson", kind, filename), nil
}

// Enroll writes the credentials file and verifies via OAuth2. Returns
// the resolved on-disk path on success.
func Enroll(ctx context.Context, opts Options) (string, error) {
	if err := validate(opts); err != nil {
		return "", err
	}

	credPath, err := CredentialsPath(opts.Kind, opts.Name)
	if err != nil {
		return "", err
	}

	if err := idempotencyCheck(credPath, opts); err != nil {
		return "", err
	}

	issuer, audience, err := discoverOIDC(ctx, opts.GibsonURL, opts.HTTPClient)
	if err != nil {
		return "", fmt.Errorf("enroll: OIDC discovery failed: %w", err)
	}

	creds := CredentialsFile{
		Issuer:       issuer,
		ClientID:     opts.ClientID,
		ClientSecret: opts.ClientSecret,
		Audience:     audience,
		GibsonURL:    opts.GibsonURL,
	}

	if err := os.MkdirAll(filepath.Dir(credPath), 0700); err != nil {
		return "", fmt.Errorf("enroll: create credentials dir: %w", err)
	}
	if err := writeCredentialsFile(credPath, creds); err != nil {
		return "", err
	}

	if err := verifyTokenExchange(ctx, creds, opts.HTTPClient); err != nil {
		// Roll back the partial credential file on verification failure
		// — half-enrolled state is worse than no-enrolled state.
		_ = os.Remove(credPath)
		return "", err
	}

	return credPath, nil
}

func validate(opts Options) error {
	switch opts.Kind {
	case "agent", "tool":
	default:
		return fmt.Errorf("enroll: kind must be \"agent\" or \"tool\" (got %q)", opts.Kind)
	}
	if strings.TrimSpace(opts.ClientID) == "" {
		return errors.New("enroll: --client-id is required")
	}
	if strings.TrimSpace(opts.ClientSecret) == "" {
		return errors.New("enroll: --client-secret is required (use --client-secret -, GIBSON_CLIENT_SECRET, or pass directly)")
	}
	if strings.TrimSpace(opts.GibsonURL) == "" {
		return errors.New("enroll: --gibson-url is required (e.g. https://api.zero-day.ai)")
	}
	return nil
}

// idempotencyCheck refuses to overwrite a credentials file whose
// client_id differs from the new one. Same-id rewrites are a no-op
// (we still proceed with the write to update issuer/audience if they
// drifted). Non-existent file is fine — it's the common first-run
// case.
func idempotencyCheck(credPath string, opts Options) error {
	existing, err := readCredentialsFile(credPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		// Permission errors / corrupt file land here. Surface them but
		// don't proceed — the user needs to look at the file.
		return fmt.Errorf("enroll: read existing credentials: %w", err)
	}
	if existing.ClientID == opts.ClientID {
		return nil
	}
	if !opts.Force {
		return fmt.Errorf(
			"enroll: refusing to overwrite credentials at %s with a different client_id (existing=%s, new=%s); pass --force to overwrite",
			credPath, existing.ClientID, opts.ClientID)
	}
	return nil
}

func readCredentialsFile(path string) (CredentialsFile, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return CredentialsFile{}, err
	}
	var c CredentialsFile
	if err := json.Unmarshal(b, &c); err != nil {
		return CredentialsFile{}, fmt.Errorf("parse credentials JSON: %w", err)
	}
	return c, nil
}

func writeCredentialsFile(path string, c CredentialsFile) error {
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("enroll: marshal credentials: %w", err)
	}
	return os.WriteFile(path, b, 0600)
}

// discoverOIDC fetches the daemon's OIDC discovery document at
// `<gibson-url>/.well-known/openid-configuration` and returns the
// issuer + audience values. Audience defaults to "gibson-platform" when
// the discovery document does not embed a custom audience claim.
func discoverOIDC(ctx context.Context, gibsonURL string, httpClient *http.Client) (string, string, error) {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	url := strings.TrimRight(gibsonURL, "/") + "/.well-known/openid-configuration"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", "", fmt.Errorf("build discovery request: %w", err)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("fetch %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("discovery returned HTTP %d from %s", resp.StatusCode, url)
	}
	var doc struct {
		Issuer   string   `json:"issuer"`
		Audience []string `json:"audiences,omitempty"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return "", "", fmt.Errorf("decode discovery: %w", err)
	}
	if doc.Issuer == "" {
		return "", "", fmt.Errorf("discovery document at %s has empty issuer", url)
	}
	audience := "gibson-platform"
	if len(doc.Audience) > 0 {
		audience = doc.Audience[0]
	}
	return doc.Issuer, audience, nil
}

// verifyTokenExchange performs the OAuth2 client_credentials grant
// against the resolved issuer's token endpoint. Success means the
// credentials are accepted by the IdP and an access token can be
// minted; failure prints the IdP's reason without leaking the secret.
func verifyTokenExchange(ctx context.Context, creds CredentialsFile, httpClient *http.Client) error {
	cfg := &clientcredentials.Config{
		ClientID:     creds.ClientID,
		ClientSecret: creds.ClientSecret,
		TokenURL:     strings.TrimRight(creds.Issuer, "/") + "/oauth/v2/token",
		Scopes:       []string{"openid"},
	}
	if httpClient != nil {
		ctx = oauth2WithClient(ctx, httpClient)
	}
	verifyCtx, cancel := context.WithTimeout(ctx, VerifyTimeout)
	defer cancel()

	if _, err := cfg.Token(verifyCtx); err != nil {
		// Detect 401/403 specifically so the operator knows it's a
		// credential issue versus a connectivity issue. Standard
		// oauth2 errors include "invalid_client" / "401 Unauthorized"
		// in the message; we surface a friendlier wrapper.
		msg := err.Error()
		switch {
		case strings.Contains(msg, "401") || strings.Contains(msg, "invalid_client"):
			return fmt.Errorf("enroll: credentials rejected by IdP at %s — verify client_id/client_secret", creds.Issuer)
		case strings.Contains(msg, "403"):
			return fmt.Errorf("enroll: IdP at %s denied the credential (403); the service account may be disabled", creds.Issuer)
		default:
			return fmt.Errorf("enroll: token exchange failed against %s: %w", creds.Issuer, err)
		}
	}
	return nil
}
