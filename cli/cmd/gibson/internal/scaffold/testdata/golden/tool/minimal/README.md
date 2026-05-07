# demo-tool

A Gibson tool scaffolded by `gibson component init`.

See **[AGENTS.md](./AGENTS.md)** for the Gibson tool contract — including
the platform rule that **proto field 100 on every tool response message
is reserved for `gibson.graphrag.v1.DiscoveryResult`**, which the daemon
auto-extracts into the GraphRAG knowledge graph. What follows is the
five-command quickstart.

## Quickstart

```sh
# 1. Generate Go bindings from api/proto/demo-tool/v1/demo-tool.proto
make proto

# 2. Build
make build

# 3. Register (paste the enroll_command's flags from the dashboard)
gibson component register \
  --client-id <id> \
  --client-secret - \
  --gibson-url <url>

# 4. Run (reads creds from ~/.gibson/tool/credentials)
make run
```

## Container

```sh
make image
docker run --rm \
  -e GIBSON_URL=https://api.zero-day.ai \
  demo-tool:0.1.0
```

## Proto layout

```
api/proto/demo-tool/v1/demo-tool.proto      # your tool's request/response
api/gen/                                    # generated Go (gitignored)
proto/vendor/gibson/graphrag/v1/            # vendored DiscoveryResult
proto/vendor/taxonomy/v1/                   # vendored taxonomy enums
buf.yaml, buf.gen.yaml                      # buf v2 configuration
```
