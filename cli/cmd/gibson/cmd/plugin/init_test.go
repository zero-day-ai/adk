package plugin

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zero-day-ai/sdk/plugin/manifest"
)

// runInit invokes the runInit function indirectly by building and running the
// CLI binary in a subprocess. For unit-testing the init logic we call the
// internal function directly via the package's exported command path.
//
// NOTE: We test via the Cobra command to exercise flag parsing end-to-end. The
// internal runInit func is in the same package (non-_test package) to allow
// white-box testing; here we call the Cobra command directly.

func TestInitCommand_CreatesFiles(t *testing.T) {
	dir := t.TempDir()

	cmd := Command()
	cmd.SetArgs([]string{"init", "my-plugin", "--dir", dir})
	err := cmd.Execute()
	require.NoError(t, err)

	pluginDir := filepath.Join(dir, "my-plugin")
	wantFiles := []string{
		"plugin.yaml",
		"main.go",
		"my-plugin.proto",
		"Makefile",
		"Dockerfile",
		".gitignore",
		"README.md",
	}
	for _, name := range wantFiles {
		path := filepath.Join(pluginDir, name)
		_, err := os.Stat(path)
		assert.NoError(t, err, "expected file %q to exist", path)
	}
}

func TestInitCommand_ManifestValidates(t *testing.T) {
	dir := t.TempDir()

	cmd := Command()
	cmd.SetArgs([]string{
		"init", "smoketest",
		"--dir", dir,
		"--with-secret", "cred:api_key=startup:live",
	})
	require.NoError(t, cmd.Execute())

	manifestPath := filepath.Join(dir, "smoketest", "plugin.yaml")
	m, err := manifest.Load(manifestPath)
	require.NoError(t, err, "rendered plugin.yaml must pass manifest.Load+Validate")
	assert.Equal(t, "smoketest", m.Metadata.Name)
	require.Len(t, m.Spec.Secrets, 1)
	assert.Equal(t, "cred:api_key", m.Spec.Secrets[0].Name)
}

func TestInitCommand_RefusesOverwriteWithoutForce(t *testing.T) {
	dir := t.TempDir()

	// First init succeeds.
	cmd1 := Command()
	cmd1.SetArgs([]string{"init", "my-plugin", "--dir", dir})
	require.NoError(t, cmd1.Execute())

	// Second init without --force should fail.
	cmd2 := Command()
	cmd2.SetArgs([]string{"init", "my-plugin", "--dir", dir})
	err := cmd2.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

func TestInitCommand_ForceOverwrites(t *testing.T) {
	dir := t.TempDir()

	cmd1 := Command()
	cmd1.SetArgs([]string{"init", "my-plugin", "--dir", dir})
	require.NoError(t, cmd1.Execute())

	cmd2 := Command()
	cmd2.SetArgs([]string{"init", "my-plugin", "--dir", dir, "--force"})
	require.NoError(t, cmd2.Execute())
}

func TestInitCommand_InvalidName(t *testing.T) {
	dir := t.TempDir()

	tests := []string{
		"My-Plugin",   // uppercase
		"1badstart",   // starts with digit
		"-badstart",   // starts with hyphen
		"a",           // too short (single char)
		"",            // empty
	}
	for _, name := range tests {
		t.Run(name, func(t *testing.T) {
			cmd := Command()
			if name == "" {
				cmd.SetArgs([]string{"init", "--dir", dir})
			} else {
				cmd.SetArgs([]string{"init", name, "--dir", dir})
			}
			err := cmd.Execute()
			assert.Error(t, err, "expected error for invalid name %q", name)
		})
	}
}

func TestInitCommand_MainGoGoVet(t *testing.T) {
	dir := t.TempDir()

	cmd := Command()
	cmd.SetArgs([]string{"init", "vet-test", "--dir", dir})
	require.NoError(t, cmd.Execute())

	pluginDir := filepath.Join(dir, "vet-test")

	// Write a minimal go.mod pointing at the SDK replace directive.
	sdkRoot := findSDKRoot(t)
	goMod := "module github.com/zero-day-ai/vet-test\n\ngo 1.25\n\n" +
		"require github.com/zero-day-ai/sdk v0.0.0\n\n" +
		"require google.golang.org/protobuf v1.34.2\n\n" +
		"replace github.com/zero-day-ai/sdk => " + sdkRoot + "\n"
	require.NoError(t, os.WriteFile(filepath.Join(pluginDir, "go.mod"), []byte(goMod), 0644))

	tidyCmd := exec.Command("go", "mod", "tidy")
	tidyCmd.Dir = pluginDir
	if out, err := tidyCmd.CombinedOutput(); err != nil {
		t.Logf("go mod tidy output:\n%s", out)
		t.Skipf("go mod tidy failed (expected in restricted CI): %v", err)
	}

	vetCmd := exec.Command("go", "vet", "./...")
	vetCmd.Dir = pluginDir
	if out, err := vetCmd.CombinedOutput(); err != nil {
		t.Fatalf("go vet on scaffolded main.go failed:\n%s", out)
	}
}

// findSDKRoot walks up from the current working directory to locate the SDK
// module root (go.mod containing "module github.com/zero-day-ai/sdk").
func findSDKRoot(t *testing.T) string {
	t.Helper()
	// The ADK module is at opensource/adk/; the SDK is at core/sdk/ — relative
	// path is ../../core/sdk from the ADK root.
	wd, err := os.Getwd()
	require.NoError(t, err)

	// Walk up to find the ADK module root (contains go.mod with our module name).
	dir := wd
	for i := 0; i < 10; i++ {
		sdkCandidate := filepath.Join(filepath.Dir(filepath.Dir(dir)), "core", "sdk")
		if _, err := os.Stat(filepath.Join(sdkCandidate, "go.mod")); err == nil {
			return sdkCandidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	// Fallback: relative path from test location.
	t.Logf("could not locate SDK module root via walk; using relative path")
	return "../../../../core/sdk"
}
