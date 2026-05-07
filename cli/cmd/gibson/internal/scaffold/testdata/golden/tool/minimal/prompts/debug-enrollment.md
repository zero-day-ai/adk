# Debugging demo-tool enrollment

Tool enrollment is identical to agent enrollment except the credentials
file lives at `~/.gibson/tool/credentials` (or
`~/.gibson/tool/credentials.<install>` for multi-install hosts).

`gibson component register` does three things in order; the error
message names which one failed.

## 1. OIDC discovery

The CLI fetches `${GIBSON_URL}/.well-known/openid-configuration`.
Failure: `enroll: OIDC discovery failed: ...`. Verify with
`curl -sS ${GIBSON_URL}/.well-known/openid-configuration`.

## 2. Credentials file write

Writes `~/.gibson/tool/credentials` with mode 0600. If a file already
exists with a **different** `client_id`, the CLI refuses without
`--force`. Re-running with the same `client_id` is a no-op success.

## 3. OAuth2 token exchange

Mints a `client_credentials` token against
`${issuer}/oauth/v2/token`. Failures:

- `credentials rejected by IdP at <issuer>` — wrong `client_id` or
  `client_secret`. Re-issue from the dashboard's Register Tool wizard.
- `IdP at <issuer> denied the credential (403)` — the principal is
  disabled. Tenant admin must re-enable.
- `token exchange failed against <issuer>` — connectivity / proxy
  issue.

## After register

```sh
gibson inspect
```

Auto-detects `~/.gibson/tool/credentials`, calls
`IdentityService.WhoAmI`, and prints effective component grants.
Tools usually need `can_execute` on whatever components the agent
calling them is authorised for; the dashboard's "Register Tool" flow
seeds that automatically when `component_grants` is supplied.

## Tool-specific gotcha: proto descriptor registration

The first time the tool's binary connects to the daemon, it sends its
`FileDescriptorSet` (the schema for `demo-toolRequest`/`Response`) via
`RegisterComponent`. The daemon caches it. If you change the proto
package or message names, you may need to bump the tool's version
field so the daemon doesn't try to dispatch using the cached old
descriptor.

If `gibson inspect` shows the tool registered but `gibson component
run` reports `unknown message type`, check that the proto FQ name in
your tool's `InputMessageType()` / `OutputMessageType()` matches the
package declaration in `api/proto/demo-tool/v1/demo-tool.proto`.
