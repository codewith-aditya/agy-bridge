# Architecture

The bridge is small on purpose: one binary, no dependencies, one translation layer. This page
explains how the pieces fit so you can change one without breaking another.

## Package layout

```
cmd/agy-bridge/        main: flags, config load, http.Server
internal/config/       env parsing, model resolution
internal/translate/    all Gemini <-> OpenAI logic, plus the tests
internal/server/       routing, handlers, upstream call, Google passthrough
```

The dependency direction is strictly one way:

```
cmd -> server -> translate
        |
        +-----> config
```

`translate` imports neither `server` nor `config`. It takes plain structs and returns plain structs,
which is why it is testable without a network, a config file or a running server.

## Request lifecycle

```
                     ┌───────────────────────────────────────────┐
 Antigravity CLI ───►│ POST /v1beta/models/{m}:streamGenerateContent
                     └───────────────┬───────────────────────────┘
                                     │  parse Gemini Request
                                     ▼
                     ┌───────────────────────────────────────────┐
                     │ translate.BuildRequest(g, model, stream)  │
                     │  systemInstruction  -> system message     │
                     │  role "model"       -> assistant          │
                     │  functionCall       -> tool_calls         │
                     │  functionResponse   -> role: tool         │
                     │  inlineData         -> image_url          │
                     │  tools[].params     -> tools[].parameters │
                     └───────────────┬───────────────────────────┘
                                     │  OpenAI chat/completions
                                     ▼
                            upstream gateway / vLLM

                     ┌───────────────────────────────────────────┐
                     │ translate.PumpStream / DecodeNonStream    │
                     │  content delta   -> parts[].text          │
                     │  reasoning delta -> parts[].thought=true  │
                     │  tool_call parts -> functionCall parts    │
                     └───────────────┬───────────────────────────┘
                                     ▼
                     data: {"candidates":[{"content":{"parts":[…]}}]}
```

The Cloud Code route runs the same pipeline; only the envelope differs (unwrap `request`, wrap each
frame as `{"response": …, "traceId": …}`).

## Routing

`internal/server.handle` dispatches on the path:

1. Any path containing `v1internal:` goes to the Cloud Code handler.
2. `…/models` returns an empty model list.
3. Everything else goes to the Gemini handler, which parses `{model}:{action}`.

Actions `generateContent`, `streamGenerateContent` and `countTokens` are implemented. Anything else
returns a Gemini-shaped 404.

## The two protocol surfaces

| | Gemini `v1beta` | Cloud Code `v1internal` |
| --- | --- | --- |
| Caller | CLI | IDE and hub |
| Credential | upstream API key | Google OAuth bearer |
| Generation path | `models/{m}:generateContent` | `v1internal:generateContent` |
| Body | `{contents, tools, generationConfig}` | `{model, project, requestId, request:{…}}` |
| Response | `{candidates, usageMetadata}` | `{response:{candidates}, traceId}` |
| Non-generation RPCs | none | forwarded to Google |

Cloud Code exists because the IDE needs an account: onboarding, the model catalog and quota are
Google-side concerns. Intercepting them would mean reimplementing an account system, so the bridge
forwards them verbatim with the caller's own bearer, and translates only generation.

## Streaming

Upstream SSE is read line by line. Each frame must start with `data:` and is JSON; `[DONE]` ends the
stream. Malformed frames are skipped rather than aborting the response, because some gateways emit
keep-alive noise.

Tool calls arrive as fragments: an `index`, an `id`, a `name`, then `arguments` split across frames.
Fragments are accumulated per index and only emitted once the stream ends, as a single
`functionCall` part. Emitting partial argument JSON would produce unparseable tool calls on the
client side.

Every emitted `functionCall` carries a placeholder `thoughtSignature`. Gemini 3 era clients expect
one when they replay the conversation, and Antigravity discards the value rather than validating it.

## Error handling

Upstream failures are re-emitted in the Gemini error envelope so the client can parse them:

```json
{"error": {"code": 403, "message": "…", "status": "PERMISSION_DENIED"}}
```

Transport-level failures (bad body, unknown action, missing key) are produced locally with the same
shape. A 401 or 403 from upstream becomes `PERMISSION_DENIED`; everything else becomes
`INVALID_ARGUMENT`.

## Concurrency

Each request is handled in its own goroutine by `net/http`. The bridge keeps no per-session state:
tool-call ids are derived from message order within a single request, so two clients interleaving
requests cannot collide. There is no cache, no queue and no background work.

The upstream HTTP client has no overall timeout because a single agent turn can legitimately run for
minutes; timeouts belong at the reverse proxy.
