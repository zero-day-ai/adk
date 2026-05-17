// Stub package for gibson.types.v1.
//
// The mission schema's generated CUE imports this package as:
//
//   "github.com/zero-day-ai/sdk/api/gen/gibson/types/v1:typespb"
//
// The real type definitions live at api/proto/gibson/types/v1 in the SDK,
// but the generated import path points here. This stub provides the minimum
// surface needed to compile #MissionDefinition. Only #Task is referenced
// directly; all other fields using typespb are optional and template-irrelevant.
//
// Embedded into the adk CLI binary via //go:embed in schema.go.
package v1

// #Task is the agent task descriptor. Only the scalar fields are
// concretely typed here; the TypedValue context/metadata maps are
// opened (any) so the stub compiles without pulling in commonpb.
#Task: {
	id?:   string
	goal?: string
	context?: {
		[string]: _
	}
	constraints?: #TaskConstraints
	metadata?: {
		[string]: _
	}
}

#TaskConstraints: {
	maxTurns?:  int32
	maxTokens?: int32
	allowedTools?: [...string]
	blockedTools?: [...string]
}
