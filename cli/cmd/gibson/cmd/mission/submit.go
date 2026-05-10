package mission

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"buf.build/go/protovalidate"
	"github.com/spf13/cobra"
	missionv1 "github.com/zero-day-ai/sdk/api/gen/gibson/mission/v1"
	daemonv1 "github.com/zero-day-ai/sdk/api/gen/gibson/daemon/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/encoding/protojson"
	sigsyaml "sigs.k8s.io/yaml"
)

// renderToYAML marshals a MissionDefinition proto to canonical
// YAML by going proto → JSON via protojson → YAML via
// sigs.k8s.io/yaml. The resulting YAML is what the daemon's own
// parser will round-trip through to reconstruct the proto.
func renderToYAML(def *missionv1.MissionDefinition) (string, error) {
	jsonBytes, err := (protojson.MarshalOptions{}).Marshal(def)
	if err != nil {
		return "", err
	}
	yamlBytes, err := sigsyaml.JSONToYAML(jsonBytes)
	if err != nil {
		return "", err
	}
	return string(yamlBytes), nil
}

func submitCmd() *cobra.Command {
	var (
		formatHint string
		dryRun     bool
		daemonAddr string
		insecureTLS bool
		timeout    time.Duration
	)
	c := &cobra.Command{
		Use:   "submit <file>",
		Short: "Validate, render, and submit a mission to the daemon",
		Long: `End-to-end submit:
1. Load + parse the mission file (CUE / YAML / JSON detected from
   extension or --format).
2. Run protovalidate on the parsed *missionv1.MissionDefinition.
3. With --dry-run, print the rendered JSON and exit; otherwise
   call gibson.daemon.v1.DaemonService/CreateMission via gRPC.
4. Print the returned mission ID.

The daemon address is taken from --daemon (default
localhost:50002) — production use should route through the
dashboard's Server Action path, not direct gRPC; this command
is the CLI escape hatch for development and CI.

Auth: in v1 the CLI reuses the ambient identity the underlying
daemon contract requires. SPIFFE / Zitadel JWT injection lives
in a follow-up wiring task — for now the connection is direct
gRPC and assumes a development daemon.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			def, err := loadMissionFile(args[0], formatHint)
			if err != nil {
				return err
			}
			v, err := protovalidate.New()
			if err != nil {
				return fmt.Errorf("protovalidate.New: %w", err)
			}
			if err := v.Validate(def); err != nil {
				return fmt.Errorf("validate: %w", err)
			}

			if dryRun {
				marshalOpts := protojson.MarshalOptions{
					Multiline: true,
					Indent:    "  ",
				}
				out, err := marshalOpts.Marshal(def)
				if err != nil {
					return fmt.Errorf("protojson marshal: %w", err)
				}
				fmt.Fprintln(cmd.OutOrStdout(), string(out))
				return nil
			}

			ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
			defer cancel()

			opts := []grpc.DialOption{}
			if insecureTLS {
				opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
			} else {
				return errors.New("only --insecure dial is supported in v1; SPIFFE auth wiring is a follow-up task")
			}

			conn, err := grpc.NewClient(daemonAddr, opts...)
			if err != nil {
				return fmt.Errorf("dial daemon at %s: %w", daemonAddr, err)
			}
			defer func() { _ = conn.Close() }()

			// Render the parsed proto back to YAML for the
			// daemon's source_yaml field. The daemon parses this
			// internally on its end via the same protojson + YAML
			// path. mission_definition_id is left empty —
			// programmatic submit creates a fresh mission per call.
			yamlPayload, err := renderToYAML(def)
			if err != nil {
				return fmt.Errorf("render yaml: %w", err)
			}

			client := daemonv1.NewDaemonServiceClient(conn)
			resp, err := client.CreateMission(ctx, &daemonv1.CreateMissionRequest{
				Name:        def.GetName(),
				Description: def.GetDescription(),
				SourceYaml:  yamlPayload,
			})
			if err != nil {
				return fmt.Errorf("CreateMission: %w", err)
			}
			fmt.Fprintln(cmd.OutOrStdout(), resp.GetMission().GetId())
			return nil
		},
	}
	c.Flags().StringVar(&formatHint, "format", "", "Override input format detection: cue|yaml|json")
	c.Flags().BoolVar(&dryRun, "dry-run", false, "Print rendered JSON; do not contact the daemon")
	c.Flags().StringVar(&daemonAddr, "daemon", envOr("GIBSON_DAEMON_ADDR", "localhost:50002"), "Daemon gRPC address")
	c.Flags().BoolVar(&insecureTLS, "insecure", false, "Use plaintext gRPC (development only)")
	c.Flags().DurationVar(&timeout, "timeout", 30*time.Second, "Submit deadline")
	return c
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
