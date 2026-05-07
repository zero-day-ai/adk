# Gibson Agent Development Kit (ADK)

The ADK exists so an **AI coding agent** — Claude Code, Cursor, or any
LLM driving your editor — can take "I want a Gibson agent that drains
Kubernetes nodes safely" and turn it into a registered, running
component without you typing the implementation.

It ships one binary, **`gibson`**, that scaffolds a complete component
directory whose entrypoint for the AI is **`AGENTS.md`** — a contract
document grounded in real Gibson SDK source paths. The agent reads
AGENTS.md on first open, picks up the contract (LLM slots, harness API,
proto field 100 = `DiscoveryResult`, manifest schema, lifecycle),
writes the implementation, and uses the same `gibson` verbs to validate
and register the result.

The Gibson runtime contracts (interfaces, manifest types, serving
helpers) live in the [SDK](https://github.com/zero-day-ai/sdk). The
ADK owns the AI-coder ergonomics around them.

## Install

```sh
go install github.com/zero-day-ai/adk/cmd/gibson@latest
gibson --help
```

Requires Go 1.24+. The binary is named `gibson`.

## What an AI-driven session looks like

```sh
# 1. One-time per workspace: pin GIBSON_URL.
gibson init --gibson-url https://api.zero-day.ai

# 2. Scaffold the component you want the AI to build.
gibson component init prom-scanner --kind tool
cd prom-scanner

# 3. Open Claude Code (or Cursor / your AI editor) here.
claude
```

What you say to Claude:

> Build a Gibson tool that probes a list of HTTPS endpoints for an
> exposed Prometheus `/metrics` route. Read AGENTS.md first for the
> contract. Populate the `Discovery` field on the response with any
> exposed services you find so they land in the GraphRAG.

What Claude does on its own — because every contract it needs is in
the directory:

1. **Reads `AGENTS.md`** — learns the `tool.Tool` interface, the
   field-100 = `gibson.graphrag.v1.DiscoveryResult` contract, the
   proto layout (`api/proto/<name>/v1/<name>.proto` + vendored SDK
   protos), the OAuth2 enrollment file path. SDK source paths are
   bare references (`core/sdk/tool/tool.go`,
   `core/sdk/api/proto/gibson/graphrag/v1/graphrag.proto`, …) so
   Claude can grep them directly.
2. **Reads `prompts/add-method.md` and `prompts/add-discovery.md`** —
   step-by-step recipes for the two changes a tool author actually
   makes. Code examples included.
3. **Edits `api/proto/prom-scanner/v1/prom-scanner.proto`** to add the
   request fields (target list, timeout) and the response fields
   (probe results) — keeping field 100 reserved for `DiscoveryResult`.
4. **Runs `make proto`** — `buf.yaml` / `buf.gen.yaml` and the
   vendored graphrag/taxonomy protos are already in the directory, so
   buf resolves everything without manual config.
5. **Implements `ExecuteProto` in `main.go`** — does the HTTPS probe,
   builds a `DiscoveryResult` with `Hosts` / `Services` entries, fills
   field 100 of the response.
6. **Runs `gibson component validate`** — local schema + proto checks
   confirm field 100 is wired correctly and `main.go` parses.
7. **Runs `make build`** — produces the binary.

You then paste the dashboard's `enroll_command` flags:

```sh
gibson component register --client-id <id> --client-secret - --gibson-url <url>
gibson component run
gibson inspect    # shows the principal's effective FGA grants
```

The `.claude/settings.json` allowlist limits Claude's shell verbs to
`make *`, `gibson *`, `buf *`, `go test ./...`, and `kubectl get *` —
no `kubectl apply`, no `helm install`, no writes against `~/.gibson/`.

## Verb surface

```
gibson init                              # workspace bootstrap
gibson component init <name> --kind …    # scaffold (agent | tool | plugin)
gibson component validate                # local schema + proto checks
gibson component register                # paste-the-enroll_command consumer
gibson component run                     # supervise the compiled binary
gibson inspect                           # who am I + my grants
gibson docs schema [component-yaml|plugin-yaml]
                                         # JSON Schema for editors / AI coders
```

The CLI **does not** call admin RPCs and **does not** mint identity.
Identity minting stays in the dashboard; `gibson component register`
consumes the `enroll_command` you paste from there.

There are no back-compat aliases (no `gibson plugin enroll` etc.).
Pre-spec callers update Makefiles and CI; the migration table is in
`CHANGELOG.md`.

## What you get when you scaffold

`gibson component init my-tool --kind tool` produces:

```
my-tool/
├── component.yaml                    # kind: tool, name, version
├── main.go                           # serve.Tool(&MyTool{})
├── api/proto/my-tool/v1/my-tool.proto   # field 100 = DiscoveryResult
├── proto/vendor/                     # vendored SDK protos (graphrag, taxonomy)
├── buf.yaml, buf.gen.yaml            # buf v2 + STANDARD lint
├── go.mod                            # pinned to SDK release
├── Makefile                          # proto/build/test/register/run/image
├── Dockerfile                        # distroless, non-root
├── README.md                         # 4-command human quickstart
├── AGENTS.md                         # ← the AI agent's contract
├── CLAUDE.md                         # Claude-Code-specific shortcut
├── prompts/                          # add-method, add-discovery,
│                                     # debug-enrollment, deploy-checklist
└── .claude/settings.json             # AI shell allowlist
```

The agent and plugin scaffolds are the same shape minus the proto
files (agent) or with a different proto layout (plugin).

## Three component shapes

| Kind   | What it is                                     | Built with              | Enrolled via            |
|--------|------------------------------------------------|-------------------------|-------------------------|
| agent  | LLM-driven gRPC service the daemon dials       | `sdk.NewAgent` + `serve.Agent` | OAuth2 client_credentials |
| tool   | Stateless gRPC tool, proto in / proto out      | `serve.Tool`            | OAuth2 client_credentials |
| plugin | Stateful integration (manifest-driven)         | `plugin.Serve`          | bootstrap-token capability-grant |

Tools follow a platform-wide rule: **proto field 100 on every tool
response is reserved for `gibson.graphrag.v1.DiscoveryResult`**. The
daemon's DiscoveryProcessor auto-extracts field 100 and writes the
entries into the GraphRAG knowledge graph — no Cypher from the tool.
The tool scaffold encodes this by default.

Plugins use a manifest (`plugin.yaml`, `apiVersion
plugin.gibson.zero-day.ai/v1`) with declared methods, secrets, runtime
mode (`process | pod | setec`), and lifecycle timeouts.

## Workspace config

`gibson init` writes `./.gibson/workspace.yaml`:

```yaml
gibson_url: https://api.zero-day.ai
```

Workspace files are non-secret. They MUST NOT contain client_id /
client_secret / bootstrap_token / host_key / password / secret /
token fields — Load() rejects them. They also do not pin a tenant —
tenant context is embedded in the credentials the dashboard issues.
Credentials live at `~/.gibson/<kind>/credentials` (mode 0600) or
`~/.gibson/plugin/<name>/host_key`.

## Build + test

```sh
make build               # → bin/gibson
make test                # unit tests (default)
make test-integration    # build-the-scaffold smoke tests; needs network + buf
make update-golden       # regenerate scaffold goldens after intentional changes
```

## Spec

The full design + tasks for this CLI lives at
[`.spec-workflow/specs/adk-developer-workflow/`](../../.spec-workflow/specs/adk-developer-workflow/)
(requirements, design, tasks, implementation logs).
