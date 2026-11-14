# Troubleshooting

Symptom → cause → fix, in rough order of how often each one bites.

## The CLI prints nothing and exits

```
warning: run ended with no output and no recorded error
```

**Cause.** The tool result never reached the model. Almost always a translation bug of the kind this
project already fixed: Antigravity sends `functionResponse` under role `model`, and a role-based
converter turns it into an empty assistant message.

**Check.** With `DEBUG=1`, open `/tmp/agy-bridge-last-req.json` and look at the messages. You should
see a `role: "tool"` entry with a `tool_call_id`. If it is `role: "assistant"` with empty content, the
translator is dispatching on role instead of part type.

**Fix.** Upgrade, or patch `translate.BuildRequest` so `functionResponse` parts are handled before the
role branch. There is a regression test for exactly this:
`TestBuildRequestToolResultUnderModelRole`.

## `401 Invalid token` from the bridge

**Cause.** The CLI supplied a key the upstream does not recognise. `GEMINI_API_KEY` is forwarded as the
upstream credential whenever `UPSTREAM_KEY` is not set on the bridge.

**Fix.** Use a valid upstream key, or set `UPSTREAM_KEY` server-side. Check with:

```bash
curl -s http://127.0.0.1:8198/health
```

`"upstream"` tells you which gateway is being hit.

## `403 insufficient quota` / `insufficient_user_quota`

**Cause.** Not a bridge problem. The upstream account behind the key has no balance or no quota for
that model.

**Fix.** Top up, or switch `DEFAULT_MODEL` / `MODEL_MAP` to a model that key can reach. Confirm with a
direct call to the gateway, outside the bridge.

## Empty answer, `finishReason: MAX_TOKENS`

**Cause.** A reasoning model spent the whole budget thinking. With a small `maxOutputTokens` the
thinking tokens are all you get, and the visible answer is empty.

**Fix.** Raise the budget, or map the picker's model to a non-reasoning one for short tasks:

```bash
MODEL_MAP="gemini-3-flash=deepseek-v4-flash"
MAX_OUTPUT_TOKENS=32000
```

Reasoning text is surfaced as `thought: true` parts, so a client that renders thinking will show it.

## Upstream rejects the request with a `max_tokens` error

**Cause.** Antigravity asks for 65535 output tokens; some gateways cap lower and 400 the request.

**Fix.** `MAX_OUTPUT_TOKENS=8192` (or whatever the gateway allows). `0` disables the clamp entirely,
which is rarely what you want.

## The IDE shows my website instead of a response

**Cause.** The request hit the site's catch-all location instead of the bridge. Either the nginx
`location ^~ /prefix/` block is missing or misordered, or you tested during a reload while an old
worker was still serving.

**Fix.** `sudo nginx -t && sudo systemctl reload nginx`, wait a second, retry. Confirm the route
directly against the bridge first:

```bash
curl -s -X POST http://127.0.0.1:8198/v1internal:generateContent \
  -H 'Content-Type: application/json' \
  -d '{"model":"gemini-3.1-pro-preview","requestId":"r1",
       "request":{"contents":[{"role":"user","parts":[{"text":"hi"}]}]}}'
```

If that works and the public URL does not, it is an nginx problem, not a bridge problem.

## The IDE ignores `jetski.cloudCodeUrl`

**Cause.** Wrong settings file, or the IDE was not restarted.

**Fix.** The setting belongs in the **user** settings file, not workspace settings:

- Windows `%APPDATA%\Antigravity IDE\User\settings.json`
- Linux `~/.config/Antigravity IDE/User/settings.json`
- macOS `~/Library/Application Support/Antigravity IDE/User/settings.json`

Quit the IDE completely (not just close the window), reopen, then watch the bridge log — you should
see `/v1internal:…` lines.

## `loadCodeAssist` returns 200 with an empty body

**Cause.** `CLOUD_CODE_PASSTHROUGH_BASE` points somewhere that answers, but not Google.

**Fix.** Leave it at the default `https://cloudcode-pa.googleapis.com`. With a junk bearer you must get
Google's own 401; anything else means the passthrough is not reaching Google.

## Streaming arrives all at once, or hangs

**Cause.** Something is buffering. Usually `proxy_buffering on` (the nginx default), or a CDN in front.

**Fix.** `proxy_buffering off;` in the location block, `proxy_cache off;`, and keep the read timeout
above the longest expected turn. If a CDN is in front, confirm it does not buffer `text/event-stream`.

## Tool calls fail with "invalid arguments"

**Cause.** The upstream emitted partial or non-JSON argument text.

**Fix.** The bridge wraps unparseable arguments as `{"raw": "..."}` rather than dropping the call, so
the model can see its own mistake. Persistent failures usually mean the chosen model is weak at tool
calling; map the picker to a stronger one.

## Bridge starts, then dies immediately

**Cause.** Port already in use.

**Fix.** `ss -lntp | grep 8198`, or set `LISTEN=127.0.0.1:8199`.

## Everything works, but slowly

Model latency dominates; the bridge adds microseconds. Check the log line for each turn:

```
gemini-3.1-pro-preview model=gpt-4o stream 4684ms text=109 thoughts=45 tools=0
```

If `thoughts` is large, you are paying for reasoning tokens. Map the picker's fast tier to a
non-reasoning model.

## Nothing here matches

Collect: the surface (CLI or IDE), versions, the bridge log line, and the `DEBUG` dumps. Then open an
issue with [the bug template](https://github.com/codewith-aditya/agy-bridge/issues/new/choose).
Redact credentials and prompt content first.
