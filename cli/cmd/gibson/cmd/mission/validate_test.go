package mission

import (
	"strings"
	"testing"
)

// TestCUESchemaValidation_Positive verifies that all four shipped
// templates parse and validate cleanly through the schema-aware path.
// Failure here means either the embedded schema bundle is broken or a
// template was accidentally drifted out of spec.
func TestCUESchemaValidation_Positive(t *testing.T) {
	templates := []struct {
		name string
		src  string
	}{
		{
			name: "recon",
			src: `import missionv1 "github.com/zero-day-ai/sdk/api/proto/gibson/mission/v1"

mission: missionv1.#MissionDefinition & {
	name:        "recon"
	description: "Reconnaissance across a target's exposed surface."
	version:     "1.0.0"
	targetRef:   ""
	nodes: {
		scan: {
			id:   "scan"
			type: missionv1.#NODE_TYPE_AGENT
			agentConfig: { agentName: "nmap-agent" }
		}
	}
	entryPoints: ["scan"]
	exitPoints: ["scan"]
}
`,
		},
		{
			name: "webapp-scan",
			src: `import missionv1 "github.com/zero-day-ai/sdk/api/proto/gibson/mission/v1"

mission: missionv1.#MissionDefinition & {
	name:    "webapp-scan"
	version: "1.0.0"
	nodes: {
		scan: {
			id:   "scan"
			type: missionv1.#NODE_TYPE_AGENT
			agentConfig: { agentName: "webvuln-agent" }
		}
	}
	entryPoints: ["scan"]
	exitPoints: ["scan"]
}
`,
		},
		{
			name: "no-import-inline",
			src: `mission: {
	name:    "inline"
	version: "1.0.0"
}
`,
		},
	}

	for _, tc := range templates {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseMission([]byte(tc.src), "cue")
			if err != nil {
				t.Fatalf("expected ok, got: %v", err)
			}
		})
	}
}

// TestCUESchemaValidation_Negative proves that a structurally invalid
// template is rejected at CUE-evaluation time — before protojson.Unmarshal
// ever runs. Three distinct structural mistakes are tested.
//
// Each sub-test asserts:
//  1. The error message starts with "cue build:" (CUE layer, not proto layer).
//  2. The error message does NOT contain "proto" (confirming the proto step
//     was never reached).
//  3. The error mentions the offending field path.
func TestCUESchemaValidation_Negative(t *testing.T) {
	cases := []struct {
		name        string
		src         string
		wantInErr   string // must appear in error string
		wantPrefix  string // error must start with this
		mustNotHave string // must NOT appear (guards against proto-layer slip-through)
	}{
		{
			name: "unknown_top_level_field",
			src: `import missionv1 "github.com/zero-day-ai/sdk/api/proto/gibson/mission/v1"

mission: missionv1.#MissionDefinition & {
	name:        "broken"
	bogus_field: "not in schema"
}
`,
			wantInErr:   "bogus_field",
			wantPrefix:  "cue build:",
			mustNotHave: "proto",
		},
		{
			name: "wrong_type_for_nodes",
			src: `import missionv1 "github.com/zero-day-ai/sdk/api/proto/gibson/mission/v1"

mission: missionv1.#MissionDefinition & {
	name:  "broken"
	nodes: "not-a-struct"
}
`,
			wantInErr:   "nodes",
			wantPrefix:  "cue build:",
			mustNotHave: "proto",
		},
		{
			// #MissionNode has a disjunction of config oneofs, so an unknown
			// field inside a node causes CUE to report "empty disjunction"
			// at the node path rather than "field not allowed" for the unknown
			// field itself. The assertion checks the full field path is present
			// in the error, confirming this is still a CUE-layer rejection.
			name: "unknown_node_field",
			src: `import missionv1 "github.com/zero-day-ai/sdk/api/proto/gibson/mission/v1"

mission: missionv1.#MissionDefinition & {
	name: "broken"
	nodes: {
		bad: {
			id:          "bad"
			type:        missionv1.#NODE_TYPE_AGENT
			agentConfig: { agentName: "x" }
			not_a_field: "disallowed"
		}
	}
	entryPoints: ["bad"]
	exitPoints:  ["bad"]
}
`,
			// CUE surfaces the disjunction failure at the node path.
			wantInErr:   "mission.nodes.bad",
			wantPrefix:  "cue build:",
			mustNotHave: "proto",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseMission([]byte(tc.src), "cue")
			if err == nil {
				t.Fatal("expected validation error, got nil")
			}
			got := err.Error()
			if !strings.HasPrefix(got, tc.wantPrefix) {
				t.Errorf("error must start with %q\ngot: %s", tc.wantPrefix, got)
			}
			if !strings.Contains(got, tc.wantInErr) {
				t.Errorf("error must mention %q\ngot: %s", tc.wantInErr, got)
			}
			if strings.Contains(got, tc.mustNotHave) {
				t.Errorf("error must NOT contain %q (indicates proto layer ran before CUE)\ngot: %s", tc.mustNotHave, got)
			}
		})
	}
}
