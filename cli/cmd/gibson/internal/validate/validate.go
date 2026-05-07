// Package validate is the kind-aware local validator behind
// `gibson component validate`. Each kind has its own checks:
//
//   - plugin: delegates to sdk/plugin/manifest.Validate (the same
//     function the SDK and daemon call).
//   - tool:   structural component.yaml + (if buf is on PATH) `buf lint`
//             over api/proto/, plus a grep-check that the response
//             message reserves field 100 = DiscoveryResult.
//   - agent:  structural component.yaml + go/parser sanity-check on
//             main.go to catch obvious typos before `go build`.
package validate

import (
	"errors"
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/zero-day-ai/sdk/plugin/manifest"

	"github.com/zero-day-ai/adk/cmd/gibson/internal/component"
)

// Issue is a single validation finding.
type Issue struct {
	Path    string // file path or "component.yaml"
	Line    int    // 0 if N/A
	Message string
}

func (i Issue) String() string {
	if i.Line > 0 {
		return fmt.Sprintf("%s:%d: %s", i.Path, i.Line, i.Message)
	}
	return fmt.Sprintf("%s: %s", i.Path, i.Message)
}

// Report aggregates findings.
type Report struct {
	Errors   []Issue
	Warnings []Issue
}

// HasErrors reports whether the report contains at least one error.
func (r *Report) HasErrors() bool { return len(r.Errors) > 0 }

// addError appends an Issue to Errors.
func (r *Report) addError(path, msg string) {
	r.Errors = append(r.Errors, Issue{Path: path, Message: msg})
}

// addWarning appends an Issue to Warnings.
func (r *Report) addWarning(path, msg string) {
	r.Warnings = append(r.Warnings, Issue{Path: path, Message: msg})
}

// Run validates the component at dir. The kind is auto-detected from
// component.yaml; pass kind="" to use auto-detection or override.
//
// The first return value is always non-nil. The error return is set
// only for I/O issues that prevent validation from running (e.g.
// component.yaml missing). Validation findings live in the report.
func Run(dir string, kind component.Kind) (*Report, error) {
	r := &Report{}

	componentYAMLPath := filepath.Join(dir, "component.yaml")
	c, err := component.Load(componentYAMLPath)
	if err != nil {
		return r, fmt.Errorf("validate: %w", err)
	}
	if kind == "" {
		kind = c.Kind
	} else if kind != c.Kind {
		r.addError("component.yaml", fmt.Sprintf("--kind=%s does not match component.yaml kind=%s", kind, c.Kind))
		return r, nil
	}

	switch kind {
	case component.KindAgent:
		validateAgent(dir, c, r)
	case component.KindTool:
		validateTool(dir, c, r)
	case component.KindPlugin:
		validatePlugin(dir, c, r)
	default:
		r.addError("component.yaml", fmt.Sprintf("unknown kind %q", kind))
	}
	return r, nil
}

// validateAgent runs structural checks for agent kind.
func validateAgent(dir string, c *component.Component, r *Report) {
	mainGo := filepath.Join(dir, c.EffectiveMainPath(), "main.go")
	if _, err := os.Stat(mainGo); err != nil {
		r.addError(mainGo, "main.go not found")
		return
	}
	fset := token.NewFileSet()
	if _, err := parser.ParseFile(fset, mainGo, nil, parser.PackageClauseOnly|parser.ParseComments); err != nil {
		r.addError(mainGo, fmt.Sprintf("main.go parse error: %v", err))
	}
}

// validateTool runs validateAgent's checks plus proto / buf checks.
func validateTool(dir string, c *component.Component, r *Report) {
	validateAgent(dir, c, r)

	// Locate the tool's primary proto file.
	protoPath := filepath.Join(dir, "api", "proto", c.Metadata.Name, "v1", c.Metadata.Name+".proto")
	if _, err := os.Stat(protoPath); err != nil {
		r.addError(protoPath, "tool proto not found at expected path api/proto/<name>/v1/<name>.proto")
		return
	}
	checkField100(protoPath, r)

	// `buf lint` if buf is on PATH; otherwise emit a warning.
	if _, err := exec.LookPath("buf"); err != nil {
		r.addWarning("buf", "buf not found on PATH; skipping `buf lint`. Install: https://buf.build/docs/installation")
		return
	}
	cmd := exec.Command("buf", "lint")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		// buf lint emits one finding per line: <file>:<line>:<col>:<message>
		for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			if line == "" {
				continue
			}
			r.addError("buf", line)
		}
		if len(r.Errors) == 0 {
			r.addError("buf", fmt.Sprintf("buf lint failed: %v", err))
		}
	}
}

// validatePlugin delegates to the SDK manifest validator.
func validatePlugin(dir string, c *component.Component, r *Report) {
	manifestPath := filepath.Join(dir, c.EffectiveManifestPath())
	if _, err := os.Stat(manifestPath); err != nil {
		r.addError(manifestPath, "plugin.yaml not found at spec.manifest_path")
		return
	}
	_, err := manifest.Load(manifestPath)
	if err == nil {
		return
	}
	if manifest.IsValidationError(err) {
		r.addError(manifestPath, err.Error())
		return
	}
	// I/O or parse error.
	r.addError(manifestPath, fmt.Sprintf("manifest load: %v", err))
}

// field100Regex catches both `gibson.graphrag.v1.DiscoveryResult discovery = 100;`
// and the reserved-only form. We accept either as proof of contract.
var field100Regex = regexp.MustCompile(`(?m)\bgibson\.graphrag\.v1\.DiscoveryResult\s+\w+\s*=\s*100\b`)

func checkField100(protoPath string, r *Report) {
	b, err := os.ReadFile(protoPath)
	if err != nil {
		r.addError(protoPath, fmt.Sprintf("read proto: %v", err))
		return
	}
	if !field100Regex.Match(b) {
		r.addError(protoPath,
			"tool response message must declare field 100 as gibson.graphrag.v1.DiscoveryResult; see AGENTS.md")
	}
}

// ErrFailed is returned by command callers when the report contains
// errors, to drive a non-zero exit code distinct from I/O errors.
var ErrFailed = errors.New("validate: report has errors")
