# ADK Changelog

## Unreleased

The ADK CLI is now the canonical Gibson developer workflow tool —
scaffolds, validates, registers, and runs Gibson components, with AI-
coder context (`AGENTS.md`, `CLAUDE.md`, `prompts/`) baked into every
scaffold. Spec:
[`.spec-workflow/specs/adk-developer-workflow/`](../../.spec-workflow/specs/adk-developer-workflow/).

### Changed

- **Binary renamed `gibson-cli` → `gibson`.** Install via
  `go install github.com/zero-day-ai/adk/cmd/gibson@latest`. There is
  no `gibson-cli` symlink or alias; update Makefiles, shell history,
  and CI steps. The daemon binary of the same name lives only inside
  container images (built from `core/gibson`), so the local-binary
  name is free to use.

- **Plugin scaffold's `make proto` works out of the box.** The plugin
  scaffold now ships `buf.yaml`, `buf.gen.yaml`, and vendored copies
  of `gibson/graphrag/v1/graphrag.proto` and `taxonomy/v1/taxonomy.proto`
  under `proto/vendor/`. The Makefile's `register` target replaces
  `enroll`; `run` delegates to `gibson component run` instead of the
  old smoke-test-only mode.

- **`gibson plugin run` is no longer a smoke test.** It exec's the
  compiled plugin binary under `internal/runner` (the same supervisor
  as `gibson component run`), forwards SIGINT/SIGTERM with a
  configurable drain timeout, and surfaces exit code 75 (the SDK
  plugin rotation contract) verbatim. The `--manifest` flag is
  preserved as a hidden no-op.

### Added

- **`gibson init`** — workspace bootstrap. Writes
  `./.gibson/workspace.yaml` (or `~/.gibson/workspace.yaml` with
  `--global`) pinning `GIBSON_URL` and the active tenant. Workspace
  files are non-secret; `Load()` rejects credential-named fields.

- **`gibson component <verb>`** — kind-aware unified verb group:
  - `init <name> --kind {agent | tool | plugin}` — scaffold a
    complete, buildable, AI-coder-ready directory. Replaces (and
    aliases from) the per-kind sprawl.
  - `validate` — local checks: `component.yaml` shape, `main.go`
    parses, plugin manifest via SDK validator, tool proto contains
    field 100 = `DiscoveryResult`, `buf lint` when buf is on PATH.
  - `register` — paste-the-`enroll_command` consumer. Agent/tool:
    OAuth2 client_credentials → `~/.gibson/<kind>/credentials`.
    Plugin: bootstrap-token capability-grant →
    `~/.gibson/plugin/<name>/host_key`. **Does NOT call admin RPCs.**
  - `run` — `internal/runner` process supervisor: SIGINT/SIGTERM
    forwarded with `--drain-timeout` (default 30s), child exit code
    surfaced verbatim including 75 = plugin rotation.

- **Agent and tool scaffolds at parity with plugin.** All three kinds
  emit `component.yaml`, `main.go`, `Makefile`, `Dockerfile`,
  `.gitignore`, `README.md`, `AGENTS.md` (kind-specific contract grounded
  in SDK source paths), `CLAUDE.md`, `prompts/*.md`,
  `.claude/settings.json`. Tool scaffold additionally ships the
  field-100 proto, `buf.yaml`, `buf.gen.yaml`, and vendored SDK protos.

- **AI-coder asset bundle in every scaffold.** `AGENTS.md` cites real
  SDK file paths; the `prompts/` directory has kind-specific task
  briefs (`add-method`, `debug-enrollment`, `deploy-checklist`, plus
  `add-discovery` for tools); `.claude/settings.json` allowlists
  `make`, `gibson`, `buf`, `go test`, `kubectl get` and explicitly
  denies `kubectl apply/patch/delete` and `helm install/upgrade`.

- **`gibson docs schema [component-yaml | plugin-yaml]`** — emits
  JSON Schema (Draft 2020-12) for editor / AI-coder validation.
  `--output <dir>` writes both schemas to disk.

- **Back-compat aliases with opt-in deprecation banners.**
  `gibson plugin {init,validate,enroll,run}`,
  `gibson agent enroll`, and `gibson tool enroll` continue to work.
  Set `GIBSON_DEPRECATION_WARNINGS=1` to print one stderr line per
  invocation pointing at the new verb form.

- **Integration tests behind `//go:build integration`.** Per kind:
  render scaffold, `go mod tidy`, `go build` — and for tool / plugin,
  `buf generate` + grep `Discovery *DiscoveryResult` with proto tag
  100 in the generated `.pb.go`. Run via `make test-integration`.

- **`AGENTS.md → SDK source path` link-resolution test.** Every
  `core/sdk/...` path cited in any rendered `AGENTS.md` is
  asserted to exist on disk. Catches drift between scaffold prose
  and SDK reality.

- **Golden-file scaffold tests for all three kinds.** Drift fails CI;
  intentional changes regenerate via `make update-golden`.

### Removed

- **`gibson-cli` binary name.** Hard cutover; no alias.

- **`github.com/zero-day-ai/sdk/plugin/scaffold` (in the SDK).** The
  scaffold package moved to
  `github.com/zero-day-ai/adk/cmd/gibson/internal/scaffold` so
  developer ergonomics can iterate without forcing an SDK release tag.
  Requires SDK v1.2.0+; the ADK is the only known consumer.

### Migration notes

For operators with shell history wired to the old verbs:
- `gibson-cli plugin enroll --token T` → `gibson component register --token T`
  (or `gibson plugin enroll --token T` keeps working with deprecation
  banner opt-in).
- Pre-spec plugin scaffolds without `buf.yaml` need either a manual
  buf config or a re-init via `gibson component init`.
- The "ServeAgent is not yet implemented" comment block in
  `core/sdk/examples/minimal-agent` is gone; it always was implemented.
