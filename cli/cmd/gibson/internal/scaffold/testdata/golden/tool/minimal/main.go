// Command demo-tool is a Gibson tool.
//
// See AGENTS.md for the full Gibson tool contract this binary implements,
// including the platform-wide rule that proto field 100 is reserved for
// gibson.graphrag.v1.DiscoveryResult on every tool response message.
package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/zero-day-ai/sdk/serve"
	"google.golang.org/protobuf/proto"

	pb "github.com/zero-day-ai/demo-tool/api/gen/demo-tool/v1"
)

// demo-toolTool is the tool implementation.
type demo-toolTool struct{}

func (t *demo-toolTool) Name() string              { return "demo-tool" }
func (t *demo-toolTool) Version() string           { return "0.1.0" }
func (t *demo-toolTool) Description() string       { return "demo-tool tool" }
func (t *demo-toolTool) InputMessageType() string  { return "gibson.tools.demo-tool.v1.demo-toolRequest" }
func (t *demo-toolTool) OutputMessageType() string { return "gibson.tools.demo-tool.v1.demo-toolResponse" }

// ExecuteProto is the tool's entrypoint. The daemon serialises the agent's
// request into the input proto.Message and unwraps the response.
//
// IMPORTANT: populate the Discovery field (proto field 100) with any
// entities + relationships your tool discovered. The Gibson daemon's
// DiscoveryProcessor reflects on field 100 of every tool response and
// writes the entries into the GraphRAG (Neo4j) knowledge graph
// automatically. See AGENTS.md and core/sdk/api/proto/gibson/graphrag/v1/.
func (t *demo-toolTool) ExecuteProto(ctx context.Context, in proto.Message) (proto.Message, error) {
	_ = in.(*pb.demo-toolRequest) // type-assert; replace with real fields

	// TODO: implement the tool's real behaviour and populate Discovery
	// with whatever Hosts / Ports / Services / etc. it learned.

	return &pb.demo-toolResponse{
		// Discovery: &graphragpb.DiscoveryResult{...},
	}, nil
}

func main() {
	if err := serve.Tool(&demo-toolTool{}); err != nil {
		slog.Error("serve tool", "err", err)
		os.Exit(1)
	}
}
