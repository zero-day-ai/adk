package component_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/zero-day-ai/adk/cmd/gibson/internal/component"
)

func TestComponent_RoundTrip(t *testing.T) {
	c := &component.Component{
		APIVersion: component.APIVersionV1,
		Kind:       component.KindTool,
		Metadata: component.ComponentMetadata{
			Name:    "demo-tool",
			Version: "0.1.0",
		},
	}
	path := filepath.Join(t.TempDir(), "component.yaml")
	require.NoError(t, component.Save(path, c))

	loaded, err := component.Load(path)
	require.NoError(t, err)
	assert.Equal(t, c.Kind, loaded.Kind)
	assert.Equal(t, c.Metadata.Name, loaded.Metadata.Name)
}

func TestComponent_ValidateRejects(t *testing.T) {
	cases := map[string]component.Component{
		"empty kind": {
			APIVersion: component.APIVersionV1,
			Metadata:   component.ComponentMetadata{Name: "x", Version: "1"},
		},
		"bad kind": {
			APIVersion: component.APIVersionV1,
			Kind:       component.Kind("nonsense"),
			Metadata:   component.ComponentMetadata{Name: "x", Version: "1"},
		},
		"bad apiVersion": {
			APIVersion: "wrong",
			Kind:       component.KindAgent,
			Metadata:   component.ComponentMetadata{Name: "x", Version: "1"},
		},
		"name fails regex": {
			APIVersion: component.APIVersionV1,
			Kind:       component.KindAgent,
			Metadata:   component.ComponentMetadata{Name: "BadName", Version: "1"},
		},
		"missing version": {
			APIVersion: component.APIVersionV1,
			Kind:       component.KindAgent,
			Metadata:   component.ComponentMetadata{Name: "x"},
		},
		"runtime on agent": {
			APIVersion: component.APIVersionV1,
			Kind:       component.KindAgent,
			Metadata:   component.ComponentMetadata{Name: "x", Version: "1"},
			Spec:       component.ComponentSpec{Runtime: "process"},
		},
		"manifest_path on tool": {
			APIVersion: component.APIVersionV1,
			Kind:       component.KindTool,
			Metadata:   component.ComponentMetadata{Name: "x", Version: "1"},
			Spec:       component.ComponentSpec{ManifestPath: "./plugin.yaml"},
		},
		"bad runtime on plugin": {
			APIVersion: component.APIVersionV1,
			Kind:       component.KindPlugin,
			Metadata:   component.ComponentMetadata{Name: "x", Version: "1"},
			Spec:       component.ComponentSpec{Runtime: "lambda"},
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			c := c
			require.Error(t, c.Validate())
		})
	}
}

func TestComponent_LoadFromCWD_ParentWalk(t *testing.T) {
	root := t.TempDir()
	deep := filepath.Join(root, "a", "b", "c")
	require.NoError(t, os.MkdirAll(deep, 0o755))

	c := &component.Component{
		APIVersion: component.APIVersionV1,
		Kind:       component.KindAgent,
		Metadata:   component.ComponentMetadata{Name: "deep-agent", Version: "0.1.0"},
	}
	require.NoError(t, component.Save(filepath.Join(root, "component.yaml"), c))

	wd, _ := os.Getwd()
	require.NoError(t, os.Chdir(deep))
	defer os.Chdir(wd) //nolint:errcheck

	loaded, path, err := component.LoadFromCWD()
	require.NoError(t, err)
	assert.Equal(t, "deep-agent", loaded.Metadata.Name)
	assert.Contains(t, path, "component.yaml")
}

func TestComponent_LoadFromCWD_NoFile(t *testing.T) {
	wd, _ := os.Getwd()
	require.NoError(t, os.Chdir(t.TempDir()))
	defer os.Chdir(wd) //nolint:errcheck

	_, _, err := component.LoadFromCWD()
	require.Error(t, err)
}

func TestComponent_EffectiveDefaults(t *testing.T) {
	plugin := &component.Component{
		APIVersion: component.APIVersionV1,
		Kind:       component.KindPlugin,
		Metadata:   component.ComponentMetadata{Name: "p", Version: "0.1.0"},
	}
	assert.Equal(t, "./", plugin.EffectiveMainPath())
	assert.Equal(t, "./plugin.yaml", plugin.EffectiveManifestPath())
	assert.Equal(t, "process", plugin.EffectiveRuntime())

	agent := &component.Component{
		APIVersion: component.APIVersionV1,
		Kind:       component.KindAgent,
		Metadata:   component.ComponentMetadata{Name: "a", Version: "0.1.0"},
	}
	assert.Equal(t, "", agent.EffectiveRuntime())
}
