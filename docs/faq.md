# FAQ

## Does this change Antigravity's model, or just add a tool?

It changes the model. The bridge speaks the exact wire protocols Antigravity already uses, so the
client believes it is talking to Google and the whole agent loop — tools, streaming, multi-turn — runs
against your upstream. MCP is a different thing entirely: an MCP server can only add tools to the
host, never change the host's model.

## Is this an official Google feature?

No. `GOOGLE_GEMINI_BASE_URL` is mentioned in the CLI's own changelog string but not documented on the
docs site, and `jetski.cloudCodeUrl` is an internal setting. Either can change in a future release.
That is why the bridge is a translation layer and not a patched binary — when the client changes, only
the bridge needs updating.

## Does it work with the IDE, or only the CLI?

Both. The CLI uses Gemini `v1beta`; the IDE and its hub use Cloud Code `v1internal`. Both surfaces are
implemented in the same binary. See [Getting started](getting-started.md).

## Do I still need a Google account?

For the CLI, no: `modelProvider: "gemini"` plus an API key runs it without signing in.

For the IDE, yes. Sign-in, onboarding, the model catalog and quota calls are Google-side, so the
bridge forwards them and only translates generation. The account can be a free one.

## Where does the Gemini system prompt go?

Every byte of it is forwarded upstream, because the model needs it to behave like Antigravity. That
means your provider sees the full prompt, the tool schema and your conversation. Point the bridge only
at providers you trust with that content.

## Can several people share one bridge?

Yes. Leave `UPSTREAM_KEY` empty and each caller supplies their own upstream key in `GEMINI_API_KEY`.
The bridge then never spends your money.

The Cloud Code route is the exception: IDE callers present a Google OAuth token, which is not an
upstream credential, so that route needs `CLOUD_SERVER_KEY` and should be kept behind an unguessable
path or a private network.

## Can I add my own models to the IDE's picker?

Not by injecting entries — the picker is fed by Google's catalog. What you can do is remap: any model
the user selects resolves through `MODEL_MAP` to whatever you want it to hit. From the user's point of
view the picker works, it just happens to be serving your models.

## Which models work best?

Anything competent at tool calling. A few datapoints from testing:

| Model family | Notes |
| --- | --- |
| GLM 5.x | Reliable tool calls, verbose reasoning; slower per turn. |
| DeepSeek V4 Flash | Fast and correct on tool calls; good default for the "flash" tier. |
| Qwen 3.x Max / 397B | Solid tool calls, good long-context behaviour. |
| MiniMax M3 | Works; verify latency for your region. |

Small or heavily quantised models tend to drop tool calls or emit truncated JSON. The bridge tolerates
truncated arguments (`{"raw": "..."}`) but the model's behaviour will still look broken.

## Does it support images?

Inline base64 images (`inlineData`) are converted to `image_url` data URIs, so vision-capable upstream
models work. Links and uploaded files that Antigravity sends as paths are not fetched.

## Does it support streaming?

Yes, both routes. Reasoning deltas are forwarded as `thought: true` parts.

## Why is the answer sometimes slower than the official client?

Reasoning models spend tokens thinking before answering. Watch the log line: a large `thoughts` count
explains the delay. Map the picker's fast tier to a non-reasoning model if latency matters more than
quality.

## Is there a rate limit?

Not in the bridge. It has no cache, no queue and no throttling; limits come from your upstream and from
whatever reverse proxy you put in front. Add rate limiting at the proxy if the endpoint is shared.

## Can I run it on Windows?

Yes. `make cross` builds `agy-bridge-windows-amd64.exe`. Point the CLI or IDE at
`http://127.0.0.1:8198` and keep it running in a terminal, or register it as a service with
`sc create` / NSSM.

## How do I upgrade?

Replace the binary and restart. There is no state, no database and no migration. Config is read from
the environment at start.

## Does it phone home?

No. The only outbound requests are the upstream chat/completions call and, on the Cloud Code route,
the forwarded Google RPCs. Nothing else, no telemetry.

## Can I contribute?

Yes — see [CONTRIBUTING.md](../CONTRIBUTING.md). The highest-value contributions are new protocol
quirks with a captured payload and a test, and compatibility notes for new Antigravity releases.
