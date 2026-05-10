package mission

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	missionv1 "github.com/zero-day-ai/sdk/api/gen/gibson/mission/v1"
	"google.golang.org/protobuf/encoding/protojson"
	sigsyaml "sigs.k8s.io/yaml"
)

// loadMissionFile reads a mission file from disk and returns the
// parsed *missionv1.MissionDefinition. The format is detected from
// the file extension (.cue, .yaml, .yml, .json). Reading from
// stdin is supported via path "-"; in that case the format must be
// passed via the --format flag (default "yaml").
func loadMissionFile(path, formatHint string) (*missionv1.MissionDefinition, error) {
	src, format, err := readSource(path, formatHint)
	if err != nil {
		return nil, err
	}
	return parseMission(src, format)
}

func readSource(path, formatHint string) ([]byte, string, error) {
	if path == "-" {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return nil, "", fmt.Errorf("read stdin: %w", err)
		}
		f := formatHint
		if f == "" {
			f = "yaml"
		}
		return data, f, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, "", fmt.Errorf("read %s: %w", path, err)
	}
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(path), "."))
	switch ext {
	case "cue":
		return data, "cue", nil
	case "yaml", "yml":
		return data, "yaml", nil
	case "json":
		return data, "json", nil
	}
	if formatHint != "" {
		return data, formatHint, nil
	}
	return nil, "", fmt.Errorf("cannot infer format from %q; pass --format yaml|json|cue", path)
}

func parseMission(src []byte, format string) (*missionv1.MissionDefinition, error) {
	switch format {
	case "json":
		return parseJSON(src)
	case "yaml", "yml":
		return parseYAML(src)
	case "cue":
		return parseCUE(src)
	default:
		return nil, fmt.Errorf("unsupported format %q", format)
	}
}

func parseYAML(src []byte) (*missionv1.MissionDefinition, error) {
	jsonBytes, err := sigsyaml.YAMLToJSON(src)
	if err != nil {
		return nil, fmt.Errorf("yaml: %w", err)
	}
	return parseJSON(jsonBytes)
}

func parseJSON(src []byte) (*missionv1.MissionDefinition, error) {
	def := &missionv1.MissionDefinition{}
	if err := (protojson.UnmarshalOptions{DiscardUnknown: false}).Unmarshal(src, def); err != nil {
		return nil, fmt.Errorf("protojson: %w", err)
	}
	return def, nil
}

// parseCUE compiles a CUE document and emits proto-shaped JSON,
// then routes through parseJSON. The CUE evaluator is loaded from
// cuelang.org/go.
//
// Cue evaluation context: a single fresh CUE context, the input
// is compiled as one file, the resulting concrete value is
// marshaled as JSON. No imports are followed in v1 — the
// authoring path uses inline CUE; importing the published
// gibson/mission/v1/#MissionDefinition definition is a future
// addition that will require pulling the bundle's CUE files into
// the eval context.
func parseCUE(src []byte) (*missionv1.MissionDefinition, error) {
	ctx, instance, err := compileCUE(src)
	if err != nil {
		return nil, err
	}
	value := instance.LookupPath(cuePath(ctx, "mission"))
	if !value.Exists() {
		// Fall back to root value (whole file is a mission).
		value = instance
	}
	jsonBytes, err := value.MarshalJSON()
	if err != nil {
		return nil, fmt.Errorf("cue marshal: %w", err)
	}
	// Strip trailing newline/whitespace before handing to protojson.
	return parseJSON(bytes.TrimSpace(jsonBytes))
}
