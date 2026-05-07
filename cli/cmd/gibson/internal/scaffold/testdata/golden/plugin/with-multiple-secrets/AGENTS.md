# AGENTS.md — byte-identity-multi

This directory is a **Gibson plugin**. A plugin is a stateful gRPC
service for integrating an external system (GitLab, Slack, Shodan,
HackerOne, …) into Gibson missions. Plugins are manifest-driven: a
single `plugin.yaml` declares identity, methods, secrets, runtime
mode, lifecycle timeouts, and egress.

This file is the contract. If a doc and the SDK source disagree, **the
SDK source wins** — paths below are bare so you can grep them.

## What you implement

A `main()` that calls `plugin.Serve(ctx, opts...)` and registers
`MethodHandler`s, one per method declared in `plugin.yaml`. The full
entry surface is in `core/sdk/plugin/serve.go`:

```go
plugin.Serve(
    context.Background(),
    plugin.WithManifest("./plugin.yaml"),
    plugin.WithMethod("Echo", echoHandler),
)
```

A `MethodHandler` has signature
`func(ctx, proto.Message) (proto.Message, error)` (alias of
`dispatch.MethodHandler`). It must NOT include secret values in error
strings.

## The manifest is the contract

`plugin.yaml` (apiVersion `plugin.gibson.zero-day.ai/v1`) declares
everything the daemon needs to register and dispatch this plugin. Read
the full schema at `core/sdk/plugin/manifest/manifest.go`. The same
`Validate` function backs the local CLI validator (`gibson plugin
validate`), the SDK at startup, and the daemon at registration time.

Key spec fields:

- `metadata.name` — DNS-label, regex `^[a-z][a-z0-9-]{0,61}[a-z0-9]$`
- `spec.workload_class` — must be `plugin`
- `spec.runtime` — one of `process | pod | setec` (default `process`)
- `spec.methods[]` — declared RPC methods (name, request_proto,
  response_proto)
- `spec.secrets[]` — broker-resolved credentials (see below)
- `spec.health.startup_timeout` — default 30s
- `spec.health.liveness_interval` — default 10s
- `spec.policy.setec_required` — boolean for the strict sandbox
- `spec.egress[]` — declared network targets (informational in
  process/pod, enforced in setec)

## The secrets broker — never env vars

**Plugins do not read secrets from environment variables or config
files.** The only credential channel is the SDK's secrets broker.
Declare in `plugin.yaml`:

```yaml
spec:
  secrets:
  - name: cred:api_key            # broker-qualified name
    scope: startup                # "startup" | "per_call"
    rotation: live                # "live"    | "restart"
    required: true
```

At runtime, request the value:

```go
secret, err := plugin.ResolveSecret(ctx, "cred:api_key")
```

Implementation: `core/sdk/plugin/secrets/`. The broker is backed by
the daemon's `GetCredential` RPC; values rotate without restart when
`rotation: live`. With `rotation: restart`, the plugin process exits
with code 75 on rotation, and the platform restarts it.

## Lifecycle state machine

`core/sdk/plugin/lifecycle/lifecycle.go` defines the states:

```
Registering → ResolvingSecrets → Starting → Ready → Draining → Stopped
```

Transitions are automatic. You can hook into them:

```go
plugin.Serve(
    ctx,
    plugin.WithManifest("./plugin.yaml"),
    plugin.WithMethod("Echo", echoHandler),
    plugin.WithHooks(lifecycle.Hooks{
        OnStart: func(ctx context.Context) error { return nil },
        OnStop:  func(ctx context.Context) error { return nil },
    }),
)
```

## Exit code 75 — the rotation contract

When a `rotation: restart` secret rotates, `plugin.Serve` exits with
code 75. systemd / Kubernetes restart policies treat 75 as
"automatic-restart-please". This is **not a crash**. Don't add error
handling that conflates 75 with failure. The CLI's `gibson component
run` surfaces 75 verbatim and prints a clear note.

## Enrollment + run loop

Plugins use a different enrollment path from agents/tools: a single-
use **bootstrap token** issued by `PluginsAdminService.RegisterPlugin`,
not OAuth2 client_credentials.

1. **Mint** — your tenant-admin uses the dashboard's Register Plugin
   wizard. The dashboard calls `PluginsAdminService.RegisterPlugin`
   with this directory's `plugin.yaml`, and it returns
   `{install_id, plugin_principal_id, bootstrap_token}` (24h TTL).
2. **Enroll** — paste the bootstrap token:
   ```sh
   gibson component register --token <bootstrap-token>
   ```
   Runs the SDK's `capabilitygrant` Bootstrap → Discover → Register
   handshake and persists `~/.gibson/plugin/byte-identity-multi/host_key` (mode
   0600). Single-use; idempotent if re-run with the same install.
3. **Run** — `make build && gibson component run`. The CLI exec's the
   compiled binary, which calls `plugin.Serve(...)`. The plugin sends
   periodic heartbeats to refresh its Redis-tracked status.
4. **Verify grants** — `gibson inspect`.

## Do not

- Do **not** read secrets from env vars. The broker is your only
  credential channel.
- Do **not** commit `host_key` or anything under `~/.gibson/`.
- Do **not** include secret values in error strings, panic messages,
  or log lines.
- Do **not** run plugins outside the SDK's `plugin.Serve` lifecycle —
  the registration handshake, heartbeats, and rotation handling all
  live there.
- Do **not** edit `plugin.yaml`'s `apiVersion` or `kind`.
- Do **not** add `replace` directives or a workspace-root `go.work`.
- Do **not** treat exit code 75 as failure.

## Where to look in the SDK

| Topic                | Path                                         |
|----------------------|----------------------------------------------|
| plugin.Serve         | `core/sdk/plugin/serve.go`                   |
| Manifest schema      | `core/sdk/plugin/manifest/manifest.go`       |
| Method dispatch      | `core/sdk/plugin/dispatch/`                  |
| Secrets broker       | `core/sdk/plugin/secrets/`                   |
| Lifecycle states     | `core/sdk/plugin/lifecycle/lifecycle.go`     |
| Health server        | `core/sdk/plugin/health/`                    |
| Capability grant     | `core/sdk/capabilitygrant/`                  |
| Egress (setec mode)  | `core/sdk/plugin/egress/`                    |

## Naming convention

Per `structure.md`, plugins follow `{service}` or `{function}` —
e.g. `gitlab`, `slack`, `scope-ingestion`. The DNS-label regex
`^[a-z][a-z0-9-]{0,61}[a-z0-9]$` is enforced by `manifest.Validate`.
