# Documentation

`agy-bridge` translates the Gemini wire formats that Google Antigravity speaks into an
OpenAI-compatible `/chat/completions` call. These pages cover everything from a first run to the
internals of the two protocols.

| Page | What it covers |
| --- | --- |
| [Getting started](getting-started.md) | Install, first run, wiring up the CLI and the IDE. |
| [Configuration](configuration.md) | Every environment variable, model mapping, key strategy. |
| [Architecture](architecture.md) | Package layout and the request lifecycle. |
| [Gemini v1beta protocol](protocols/gemini-v1beta.md) | What the CLI sends and what the bridge returns. |
| [Cloud Code protocol](protocols/cloud-code-v1internal.md) | The IDE envelope, passthrough, and the hidden setting. |
| [Deployment](deployment.md) | systemd, nginx, TLS, and hardening a shared endpoint. |
| [Troubleshooting](troubleshooting.md) | Symptom → cause → fix for the failures people actually hit. |
| [FAQ](faq.md) | Short answers to recurring questions. |
| [Internals](ANTIGRAVITY-internals.md) | Reverse-engineering notes from the shipped binaries. |

## Conventions in these docs

- `$BASE` is the bridge URL, for example `http://127.0.0.1:8198` or `https://example.com/agy`.
- `$KEY` is an upstream credential. Never commit one.
- Everything is copy-pasteable; placeholders are obvious and never real secrets.
