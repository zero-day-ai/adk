package mission

import (
	"fmt"

	"github.com/spf13/cobra"
	"google.golang.org/protobuf/encoding/protojson"
	sigsyaml "sigs.k8s.io/yaml"
)

func renderCmd() *cobra.Command {
	var (
		formatHint string
		outFormat  string
	)
	c := &cobra.Command{
		Use:   "render <file>",
		Short: "Compile a mission file to proto-shaped JSON or YAML",
		Long: `Render the input mission file as the canonical proto-shaped JSON
the daemon expects. With --out-format yaml, JSON is converted to YAML
via sigs.k8s.io/yaml (round-trip-equivalent).

Useful for:
- Reviewing what the daemon will see before submitting.
- Diffing two mission files semantically (compile both then diff JSON).
- Piping into other tooling (` + "`gibson mission render m.cue | jq …`" + `).

Output is deterministic: protojson uses stable proto field ordering;
sigs.k8s.io/yaml uses lexical key ordering.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			def, err := loadMissionFile(args[0], formatHint)
			if err != nil {
				return err
			}
			marshalOpts := protojson.MarshalOptions{
				Multiline: true,
				Indent:    "  ",
			}
			jsonBytes, err := marshalOpts.Marshal(def)
			if err != nil {
				return fmt.Errorf("protojson marshal: %w", err)
			}
			switch outFormat {
			case "json", "":
				fmt.Fprintln(cmd.OutOrStdout(), string(jsonBytes))
			case "yaml":
				yamlBytes, err := sigsyaml.JSONToYAML(jsonBytes)
				if err != nil {
					return fmt.Errorf("yaml convert: %w", err)
				}
				fmt.Fprint(cmd.OutOrStdout(), string(yamlBytes))
			default:
				return fmt.Errorf("unsupported --out-format %q (use json or yaml)", outFormat)
			}
			return nil
		},
	}
	c.Flags().StringVar(&formatHint, "format", "", "Override input format detection: cue|yaml|json")
	c.Flags().StringVar(&outFormat, "out-format", "json", "Output format: json|yaml")
	return c
}
