package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/zero-day-ai/sdk/plugin"
	"google.golang.org/protobuf/proto"
)

func echoHandler(ctx context.Context, req proto.Message) (proto.Message, error) {
	// Stub: return the request unchanged.
	return req, nil
}

func main() {
	err := plugin.Serve(
		context.Background(),
		plugin.WithManifest("./plugin.yaml"),
		plugin.WithMethod("Echo", echoHandler),
	)
	if err != nil {
		slog.Error("plugin exited with error", "err", err)
		os.Exit(1)
	}
}
