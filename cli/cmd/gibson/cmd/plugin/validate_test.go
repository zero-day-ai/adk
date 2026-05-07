package plugin

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeExiter records the exit code instead of calling os.Exit.
func fakeExiter(code int) {
	// No-op; captured via closed-over variable in the caller.
}

// captureExiter returns an exiter that stores the code in *got.
func captureExiter(got *int) exiter {
	return func(code int) {
		*got = code
	}
}

func TestValidateCommand_ValidManifest(t *testing.T) {
	dir := t.TempDir()

	validManifest := `apiVersion: plugin.gibson.zero-day.ai/v1
kind: Plugin
metadata:
  name: my-plugin
  version: 0.1.0
spec:
  workload_class: plugin
  methods:
  - name: Echo
    request_proto: gibson.plugins.myplugin.v1.EchoRequest
    response_proto: gibson.plugins.myplugin.v1.EchoResponse
`
	manifestPath := filepath.Join(dir, "plugin.yaml")
	require.NoError(t, os.WriteFile(manifestPath, []byte(validManifest), 0644))

	var exitCode int
	var outBuf, errBuf bytes.Buffer
	err := runValidate(manifestPath, &outBuf, &errBuf, captureExiter(&exitCode))
	require.NoError(t, err)
	assert.Equal(t, 0, exitCode, "valid manifest must not trigger exit")
	assert.Contains(t, outBuf.String(), "manifest valid")
	assert.Contains(t, outBuf.String(), "my-plugin")
}

func TestValidateCommand_InvalidManifest_ExitsWithCode2(t *testing.T) {
	dir := t.TempDir()

	invalidManifest := `apiVersion: wrong.version/v1
kind: Plugin
metadata:
  name: bad-plugin
  version: 0.1.0
spec:
  workload_class: plugin
  methods:
  - name: Echo
    request_proto: gibson.plugins.badplugin.v1.EchoRequest
    response_proto: gibson.plugins.badplugin.v1.EchoResponse
`
	manifestPath := filepath.Join(dir, "plugin.yaml")
	require.NoError(t, os.WriteFile(manifestPath, []byte(invalidManifest), 0644))

	exitCode := -1
	var outBuf, errBuf bytes.Buffer
	err := runValidate(manifestPath, &outBuf, &errBuf, captureExiter(&exitCode))
	require.NoError(t, err, "runValidate should not return an error for validation failures (calls exit(2) instead)")
	assert.Equal(t, 2, exitCode, "invalid manifest must trigger exit(2)")
	assert.Contains(t, errBuf.String(), "manifest invalid")
	assert.Contains(t, errBuf.String(), "apiVersion")
}

func TestValidateCommand_MultipleViolations(t *testing.T) {
	dir := t.TempDir()

	badManifest := `apiVersion: bad/v1
kind: Plugin
metadata:
  name: x
  version: not-semver
spec:
  workload_class: plugin
  methods: []
`
	manifestPath := filepath.Join(dir, "plugin.yaml")
	require.NoError(t, os.WriteFile(manifestPath, []byte(badManifest), 0644))

	exitCode := -1
	var outBuf, errBuf bytes.Buffer
	err := runValidate(manifestPath, &outBuf, &errBuf, captureExiter(&exitCode))
	require.NoError(t, err)
	assert.Equal(t, 2, exitCode)
	errMsg := errBuf.String()
	// Multiple violations should all appear in the error output.
	assert.Contains(t, errMsg, "apiVersion")
}

func TestValidateCommand_MissingFile(t *testing.T) {
	var outBuf, errBuf bytes.Buffer
	exitCode := -1
	err := runValidate("/nonexistent/plugin.yaml", &outBuf, &errBuf, captureExiter(&exitCode))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "plugin validate")
	assert.Equal(t, -1, exitCode, "I/O errors must not call exit(2)")
}

func TestValidateCommand_CobraIntegration_ValidManifest(t *testing.T) {
	dir := t.TempDir()

	validManifest := `apiVersion: plugin.gibson.zero-day.ai/v1
kind: Plugin
metadata:
  name: cobra-test
  version: 0.2.0
spec:
  workload_class: plugin
  methods:
  - name: Ping
    request_proto: gibson.plugins.cobratest.v1.PingRequest
    response_proto: gibson.plugins.cobratest.v1.PingResponse
`
	manifestPath := filepath.Join(dir, "plugin.yaml")
	require.NoError(t, os.WriteFile(manifestPath, []byte(validManifest), 0644))

	cmd := Command()
	cmd.SetArgs([]string{"validate", manifestPath})
	err := cmd.Execute()
	assert.NoError(t, err)
}
