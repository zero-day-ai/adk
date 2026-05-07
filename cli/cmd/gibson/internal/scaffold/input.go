package scaffold

import (
	"fmt"
	"strings"
)

// Kind identifies which Gibson component shape a scaffold renders.
type Kind string

const (
	KindAgent  Kind = "agent"
	KindTool   Kind = "tool"
	KindPlugin Kind = "plugin"
)

// Valid reports whether k is one of the supported kinds.
func (k Kind) Valid() bool {
	switch k {
	case KindAgent, KindTool, KindPlugin:
		return true
	}
	return false
}

// ScaffoldInput carries everything Render needs to produce a complete
// component directory.
type ScaffoldInput struct {
	// Name is the component's DNS-label identifier (e.g. "my-scanner").
	// Must match ^[a-z][a-z0-9-]{0,61}[a-z0-9]$. Caller validates.
	Name string

	// Version is the initial semver string. Defaults to "0.1.0" when empty.
	Version string

	// Kind selects the template set: agent | tool | plugin.
	Kind Kind

	// Secrets is plugin-only. Non-nil for other kinds is a caller bug
	// caught by Render with a clear error.
	Secrets []SecretInput

	// SDKVersion pins the SDK in the rendered go.mod. Typically derived
	// from runtime/debug.ReadBuildInfo at the cobra layer.
	SDKVersion string
}

// SecretInput is a single secret declaration parsed from a --with-secret flag.
// Plugin-only.
type SecretInput struct {
	Name     string // e.g. "cred:db_password"
	Scope    string // "startup" | "per_call"
	Rotation string // "live"     | "restart"
}

// ParseSecretFlag parses a --with-secret flag value of the form
// "name=scope:rotation" into a SecretInput.
//
// Example: ParseSecretFlag("cred:api_key=startup:live")
func ParseSecretFlag(s string) (SecretInput, error) {
	eqIdx := strings.Index(s, "=")
	if eqIdx < 0 {
		return SecretInput{}, fmt.Errorf("scaffold: --with-secret %q: expected format name=scope:rotation", s)
	}
	name := strings.TrimSpace(s[:eqIdx])
	rest := strings.TrimSpace(s[eqIdx+1:])
	parts := strings.SplitN(rest, ":", 2)
	if len(parts) != 2 {
		return SecretInput{}, fmt.Errorf("scaffold: --with-secret %q: expected scope:rotation after '=', got %q", s, rest)
	}
	scope := strings.TrimSpace(parts[0])
	rotation := strings.TrimSpace(parts[1])
	if scope != "startup" && scope != "per_call" {
		return SecretInput{}, fmt.Errorf("scaffold: --with-secret %q: scope must be 'startup' or 'per_call', got %q", s, scope)
	}
	if rotation != "live" && rotation != "restart" {
		return SecretInput{}, fmt.Errorf("scaffold: --with-secret %q: rotation must be 'live' or 'restart', got %q", s, rotation)
	}
	return SecretInput{Name: name, Scope: scope, Rotation: rotation}, nil
}
