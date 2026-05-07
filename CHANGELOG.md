# ADK Changelog

## Unreleased

The ADK CLI is the canonical Gibson developer workflow tool —
scaffolds, validates, registers, and runs Gibson components, with AI-
coder context (`AGENTS.md`, `CLAUDE.md`, `prompts/`) baked into every
scaffold so an LLM coder can be productive on first open. Spec:
[`.spec-workflow/specs/adk-developer-workflow/`](../../.spec-workflow/specs/adk-developer-workflow/).

### Verbs

```
gibson init                              # workspace bootstrap
gibson component init <name> --kind agent|tool|plugin
gibson component validate
gibson component register
gibson component run
gibson docs schema [component-yaml | plugin-yaml]
gibson inspect
```

### Highlights

- **Scaffolds for all three component shapes.** Tool scaffold ships
  proto field 100 = `gibson.graphrag.v1.DiscoveryResult` plus
  `buf.yaml`, `buf.gen.yaml`, and vendored SDK protos so `make proto`
  works out of the box. Plugin scaffold has the same buf vendoring.
  Agent scaffold ships the LLM-slot + harness skeleton.

- **`AGENTS.md` per kind, grounded in real SDK source paths**, verified
  by a link-resolution test (`TestAgentsMD_LinksResolveAgainstLocalSDK`)
  that walks every `core/sdk/...` reference and asserts the file
  exists at the pinned SDK tag.

- **Workspace config (`.gibson/workspace.yaml`)** refuses
  credential-named fields and world-writable mode permissions.
  Carries no tenant identifier — tenant context is embedded in the
  credentials the dashboard issues.

- **`gibson docs schema`** emits JSON Schema (Draft 2020-12) for
  `component.yaml` and `plugin.yaml` so editors and AI coders get
  inline validation.

- **Process supervisor** (`internal/runner`) forwards SIGINT/SIGTERM
  with `--drain-timeout` and surfaces exit code 75 (the SDK plugin
  rotation contract) verbatim.

- **No admin RPCs.** `gibson component register` is a paste-the-
  `enroll_command` consumer, by design. Identity minting stays in the
  dashboard.

- **No back-compat shims.** Clean cutover: `gibson plugin <verb>` /
  `gibson agent enroll` / `gibson tool enroll` (the pre-spec verb
  forms) are gone. Update Makefiles and CI to use
  `gibson component <verb>`.

- **Integration tests behind `//go:build integration`.** Per kind:
  render scaffold, `go mod tidy`, `go build` — and for tool / plugin,
  `buf generate` + grep for `Discovery *DiscoveryResult` with proto
  tag 100 in the generated `.pb.go`. Run via
  `make test-integration`.

- **Golden-file scaffold tests for all three kinds.** Drift fails CI;
  intentional changes regenerate via `make update-golden`.

### Migration

Pre-spec scaffolds and Makefiles will need updates:

- `gibson-cli plugin enroll --token T` → `gibson component register --token T`
- `gibson-cli agent enroll …` → `gibson component register …`
- `gibson-cli tool enroll …`  → `gibson component register …`

For plugin scaffolds without `buf.yaml`, re-init via
`gibson component init <name> --kind plugin --force` (or hand-add
`buf.yaml` / `buf.gen.yaml` / `proto/vendor/`).

Requires `github.com/zero-day-ai/sdk` v1.2.0+.
