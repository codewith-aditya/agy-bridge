# Security policy

## Reporting a vulnerability

Email the maintainer or open a private security advisory on GitHub. Please do not file a public
issue, and do not include live credentials in any report.

Expect an acknowledgement within 72 hours. If the issue is confirmed, a fix and a coordinated
disclosure will follow.

## Threat model

`agy-bridge` is a local translation proxy. It is not an authentication service and it does not
sandbox the models it talks to. Keep the following in mind when you deploy it.

- **The bridge holds upstream credentials.** `UPSTREAM_KEY` and `CLOUD_SERVER_KEY` grant spending
  power on your gateway. Store them in an `EnvironmentFile` with `0600` permissions, not in a shell
  profile or a committed file.
- **`UPSTREAM_KEY` empty is the safe default.** Callers must present their own key, so a leaked URL
  does not leak your balance. Only set `UPSTREAM_KEY` when you intend to host for others.
- **The Cloud Code route cannot authenticate per user.** Its callers send a Google OAuth bearer,
  which is not an upstream credential. Put that route behind an unguessable path, a private network,
  or an authenticating reverse proxy.
- **Everything the model is told is visible upstream.** The full Antigravity system prompt and tool
  schema are forwarded. Point the bridge only at providers you trust with that content.
- **The passthrough forwards the caller's bearer to Google.** It does not store or log tokens. Request
  bodies are logged only with `DEBUG` enabled, and only to `/tmp`.
- **No TLS termination.** Run it on loopback behind nginx, Caddy or similar when it is reachable from
  another machine.

## Supported versions

The latest release on the default branch receives fixes. There are no long-lived branches.
