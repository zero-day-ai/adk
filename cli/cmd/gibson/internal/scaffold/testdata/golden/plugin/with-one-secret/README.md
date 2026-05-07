# byte-identity-secret

A Gibson plugin scaffolded by `gibson component init`.

See **[AGENTS.md](./AGENTS.md)** for the full Gibson plugin contract —
manifest schema, lifecycle states, secrets-broker rules, exit-75
rotation, and the SDK source paths to grep. What follows is the
four-command quickstart.

## Quickstart

```sh
# 1. Generate Go bindings from byte-identity-secret.proto
make proto

# 2. Build
make build

# 3. Register (paste the bootstrap-token from the dashboard)
gibson component register --token <bootstrap-token>

# 4. Run (reads ~/.gibson/plugin/byte-identity-secret/host_key)
make run
```

## Container (pod runtime mode)

```sh
docker build -t byte-identity-secret:1.2.3 .
docker run --rm \
  -e GIBSON_URL=https://api.zero-day.ai \
  -e GIBSON_PLUGIN_RUNTIME=pod \
  byte-identity-secret:1.2.3
```

## Operator runbooks (internal)

- `enterprise/deploy/docs/runbooks/plugin-runtime.md`
- `enterprise/deploy/docs/runbooks/secrets-broker.md`
