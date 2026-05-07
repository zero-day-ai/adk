# demo-agent

A Gibson agent scaffolded by `gibson component init`.

See **[AGENTS.md](./AGENTS.md)** for the full Gibson agent contract this
scaffold implements (LLM slots, harness API, lifecycle, do-not-do
list). What follows is the four-command quickstart.

## Quickstart

```sh
# 1. Build
make build

# 2. Register (paste the enroll_command's flags from the dashboard)
gibson component register \
  --client-id <id> \
  --client-secret - \
  --gibson-url <url>

# 3. Run (reads creds from ~/.gibson/agent/credentials)
make run
```

## Container

```sh
make image
docker run --rm \
  -e GIBSON_URL=https://api.zero-day.ai \
  demo-agent:0.1.0
```
