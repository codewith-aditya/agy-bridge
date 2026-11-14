# Configuration

Every setting is an environment variable. There is no config file, so a systemd unit or a container
environment is enough.

## Reference

| Variable | Default | Description |
| --- | --- | --- |
| `LISTEN` | `127.0.0.1:8198` | Bind address. Use `0.0.0.0:8198` only behind a firewall or reverse proxy. |
| `UPSTREAM_BASE` | `https://modelrouter.in/v1` | OpenAI-compatible base URL. No trailing slash; the bridge appends `/chat/completions`. |
| `UPSTREAM_KEY` | *empty* | Key used for the CLI route. Empty means the caller's `GEMINI_API_KEY` is used. |
| `CLOUD_SERVER_KEY` | *empty* | Key used for the Cloud Code route, whose callers present a Google OAuth bearer. Falls back to `UPSTREAM_KEY`, then the caller's key. |
| `DEFAULT_MODEL` | *empty* | Upstream model used for any Gemini model ID that is not in `MODEL_MAP`. |
| `MODEL_MAP` | *empty* | Comma-separated `gemini=upstream` pairs. |
| `MAX_OUTPUT_TOKENS` | `32000` | Clamp applied to `generationConfig.maxOutputTokens`. `0` disables it. |
| `CLOUD_CODE_PASSTHROUGH_BASE` | `https://cloudcode-pa.googleapis.com` | Google host for non-generation Cloud Code RPCs. |
| `DEBUG` | *empty* | Any non-empty value dumps the last request bodies to `/tmp/agy-bridge-*.json`. |

## Credential resolution

The bridge never stores credentials beyond process memory, and the two routes resolve keys
differently on purpose.

**CLI route (`/v1beta/...`)**

1. `UPSTREAM_KEY`, if set.
2. Otherwise the caller's key: `x-goog-api-key` header, then `?key=`, then `Authorization: Bearer`.

**Cloud Code route (`/v1internal:...`)**

1. `CLOUD_SERVER_KEY`, if set.
2. Otherwise `UPSTREAM_KEY`.
3. Otherwise the caller's key.

The IDE sends a Google OAuth token in `Authorization`, which no upstream will accept as an API key,
so in practice that route needs a server-side key.

### Leaving `UPSTREAM_KEY` empty

This is the recommended default for a shared endpoint. Every client presents its own upstream key in
`GEMINI_API_KEY`, spends its own balance, and a leaked URL costs you nothing. Set `UPSTREAM_KEY` only
when you are deliberately hosting for others.

## Model mapping

Antigravity asks for models by Gemini ID. The bridge resolves each one, in order:

1. an exact entry in `MODEL_MAP`,
2. `DEFAULT_MODEL`,
3. the name it was given, unchanged.

```bash
MODEL_MAP="gemini-3.1-pro-preview=glm-5.3,gemini-3-flash=deepseek-v4-flash"
DEFAULT_MODEL=glm-5.3
```

With that configuration the picker's "Gemini 3.1 Pro (High)" is served by `glm-5.3`, while "Gemini 3
Flash (Low)" is served by `deepseek-v4-flash`. Anything unrecognised falls back to `glm-5.3`.

Mapping is intentional, not cosmetic: it lets a user change model quality and latency from
Antigravity's own picker without touching the bridge.

## Output token clamp

Antigravity requests `maxOutputTokens: 65535`. Some upstreams reject anything above their own limit,
so the bridge clamps to `MAX_OUTPUT_TOKENS` by default. Raise it for long-form work, lower it to cap
spend per turn, or set `0` to pass the client's value through untouched.

## Debugging

```bash
DEBUG=1 ./bin/agy-bridge
```

With `DEBUG` set, each request overwrites:

| File | Contents |
| --- | --- |
| `/tmp/agy-bridge-last-gemini.json` | Raw Gemini `v1beta` body received from the client. |
| `/tmp/agy-bridge-last-cloud.json` | Raw Cloud Code envelope from the IDE. |
| `/tmp/agy-bridge-last-cloud-inner.json` | The unwrapped `request` object. |
| `/tmp/agy-bridge-last-req.json` | The translated OpenAI body actually sent upstream. |

These contain the full Antigravity system prompt and any prompt content, so treat them as sensitive.

## Example `.env`

`examples/agy-bridge.env.example` in the repository:

```bash
UPSTREAM_BASE=https://api.openai.com/v1
UPSTREAM_KEY=sk-...
DEFAULT_MODEL=gpt-4o-mini
MODEL_MAP=gemini-3.1-pro-preview=gpt-4o,gemini-3-flash=gpt-4o-mini
MAX_OUTPUT_TOKENS=32000
LISTEN=127.0.0.1:8198
```

`chmod 600` it, and load it with `EnvironmentFile=` (systemd) or `set -a; . ./.env; set +a` (shell).
