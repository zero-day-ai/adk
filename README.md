# Gibson Agent Development Kit (ADK)

The ADK is the developer-facing companion to the
[Gibson SDK](https://github.com/zero-day-ai/sdk). It ships **`gibson`**,
a single CLI that scaffolds, validates, registers, and runs Gibson
components — agents, tools, and plugins — and bakes AI-coder context
(`AGENTS.md`, `CLAUDE.md`, `prompts/`) into every scaffolded directory
so an LLM coder can be productive on first open.

The Gibson runtime contracts (interfaces, manifest types, serving
helpers) live in the SDK. The ADK owns the developer ergonomics
around them.

## Install

```sh
go install github.com/zero-day-ai/adk/cmd/gibson@latest
gibson --help
```

Requires Go 1.24+. The binary is named `gibson`.

## The 5-verb developer loop

```sh
# 1. One-time per project: pin GIBSON_URL + tenant.
gibson init --gibson-url https://api.zero-day.ai --tenant-ref tenants/acme

# 2. Scaffold a component (agent | tool | plugin).
gibson component init my-tool --kind tool
cd my-tool

# 3. Open AGENTS.md — the contract this scaffold implements.
#    Your AI coder reads this on first open.
$EDITOR AGENTS.md

# 4. Build and validate.
make proto && make build
gibson component validate

# 5. Register (paste the enroll_command from the dashboard) and run.
gibson component register --client-id <id> --client-secret - --gibson-url <url>
gibson component run
```

`gibson inspect` shows the principal's effective FGA grants once
registered.

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

Back-compat shims (deprecation banner gated by
`GIBSON_DEPRECATION_WARNINGS=1`):

```
gibson agent enroll        →  gibson component register --kind agent
gibson tool enroll         →  gibson component register --kind tool
gibson plugin init|validate|enroll|run
                           →  gibson component <verb> --kind plugin
```

## What you get when you scaffold

`gibson component init my-tool --kind tool` produces (truncated):

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
├── README.md                         # 4-command quickstart
├── AGENTS.md                         # full Gibson contract for AI coders
├── CLAUDE.md                         # Claude-Code shortcut
├── prompts/                          # add-method, add-discovery,
│   │                                 # debug-enrollment, deploy-checklist
└── .claude/settings.json             # allowlist (make/gibson/buf/go-test/kubectl-get)
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
response is reserved for `gibson.graphrag.v1.DiscoveryResult`**. Field
100 is auto-extracted by the daemon's DiscoveryProcessor and written
to the GraphRAG knowledge graph. The tool scaffold encodes this by
default.

Plugins use a manifest (`plugin.yaml`, `apiVersion
plugin.gibson.zero-day.ai/v1`) with declared methods, secrets, runtime
mode (`process | pod | setec`), and lifecycle timeouts.

The SDK is the contract; AGENTS.md in each scaffolded dir cites SDK
source paths for the full surface. The link-resolution test
(`TestAgentsMD_LinksResolveAgainstLocalSDK`) keeps the cited paths
honest.

## Workspace config

`gibson init` writes `./.gibson/workspace.yaml`:

```yaml
gibson_url: https://api.zero-day.ai
tenant_ref: tenants/acme
```

Workspace files are non-secret. They MUST NOT contain client_id /
client_secret / bootstrap_token / host_key / password / secret /
token fields — Load() rejects them. Credentials live at
`~/.gibson/<kind>/credentials` (mode 0600) or
`~/.gibson/plugin/<name>/host_key`.

The CLI does NOT call admin RPCs and does NOT mint identity. Identity
provisioning is the dashboard's job; `gibson component register` is a
paste-the-`enroll_command` consumer only.

## Build + test

```sh
make build               # → bin/gibson
make test                # unit tests (default)
make test-integration    # build-the-scaffold smoke tests; needs network + buf
make update-golden       # regenerate scaffold goldens after intentional changes
make vet
```

## Spec

The full spec for this CLI lives at
[`.spec-workflow/specs/adk-developer-workflow/`](../../.spec-workflow/specs/adk-developer-workflow/)
(requirements, design, tasks, implementation logs).
