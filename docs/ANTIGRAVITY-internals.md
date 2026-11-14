# Antigravity internals

Notes from inspecting the Antigravity CLI (v1.5.9) and IDE (v2.7.0) binaries. Everything here was
observed directly, either by reading strings out of the shipped binaries or by pointing the client at
a logging server and recording what it sent.

## The CLI speaks the public Gemini API

`agy` is a Go binary built on `google-genai-sdk`. With `modelProvider: "gemini"` in
`~/.gemini/config/settings.json` and a `GEMINI_API_KEY` in the environment, it calls:

```
POST {base}/v1beta/models/{model}:streamGenerateContent?alt=sse
POST {base}/v1beta/models/{model}:generateContent
POST {base}/v1beta/models/{model}:countTokens
```

The credential travels in the `x-goog-api-key` header. The base URL comes from
`GOOGLE_GEMINI_BASE_URL`, which is documented in the changelog string embedded in the binary:

> Added support for `GEMINI_API_KEY`, so the CLI can run against the Gemini API directly without
> signing in. Set `modelProvider: "gemini"` in `settings.json`, export `GEMINI_API_KEY`, and point
> `GOOGLE_GEMINI_BASE_URL` at a custom endpoint if you need one.

A default session sends a ~38 KB system prompt and fourteen tool declarations: `read_file`,
`run_command`, `manage_task`, `send_message`, `schedule`, `invoke_subagent`, `define_subagent`,
`manage_subagents`, `write_file`, `edit_file`, `search_web`, `run_workflow`, `swarm`, `ask_question`.

Tool schemas arrive as `parametersJsonSchema`, which is plain JSON Schema (draft 2020-12) and maps
directly onto OpenAI's `tools[].function.parameters`.

### Two quirks worth knowing

**Tool results are tagged `model`.** The follow-up turn that carries a tool result looks like:

```json
{"role": "model", "parts": [{"functionResponse": {"name": "run_command", "response": {}}}]}
```

The Gemini specification uses `user` for function responses. A converter that switches on role turns
these into empty assistant messages, the upstream rejects or ignores them, and the run ends with
"run ended with no output and no recorded error". Switch on part type instead.

**No call ids.** `functionResponse` has a name but no id, while OpenAI tool messages need a
`tool_call_id`. Synthesise one when emitting `tool_calls` (`call_<n>_<name>`) and match responses
back FIFO per function name.

`functionCall` parts also carry a `thoughtSignature` on the way back in, so the bridge attaches a
placeholder signature to every call it emits; the client round-trips it and does not validate it.

## The IDE speaks Cloud Code

The IDE is a VS Code derivative. It does not use `v1beta`; it uses Google's internal Cloud Code API
against `cloudcode-pa.googleapis.com`, with RPCs named `v1internal:*`:

```
v1internal:loadCodeAssist        v1internal:onboardUser
v1internal:fetchAvailableModels  v1internal:fetchUserInfo
v1internal:generateContent       v1internal:streamGenerateContent
v1internal:countTokens           v1internal:fetchAdminControls
```

The envelope is a wrapper around a normal GenerateContent request:

```json
{"model": "...", "project": "...", "requestId": "...", "request": {"contents": []}}
{"response": {"candidates": []}, "traceId": "..."}
```

### The endpoint override

`resources/app/out/main.js` builds the base URL like this (minified, abridged):

```js
cloudCodeUrlOverride: this._configurationService.getValue("jetski.cloudCodeUrl")

function NBo({isDevMode, isGoogleInternal, isGcpTos, cloudCodeUrlOverride}) {
  return cloudCodeUrlOverride ||
    (isGoogleInternal && isDevMode ? "https://autopush-cloudcode-pa.sandbox.googleapis.com"
   : isGoogleInternal           ? "https://preprod-daily-cloudcode-pa.sandbox.googleapis.com"
   : (isGcpTos)                 ? "https://cloudcode-pa.googleapis.com"
   :                              "https://daily-cloudcode-pa.googleapis.com")
}
```

So a single user setting reroutes the entire Cloud Code client, with no patching:

```json
{ "jetski.cloudCodeUrl": "http://127.0.0.1:8198" }
```

The bundled hub / language server is the same Go binary as the CLI, and carries parallel knobs:
`CLOUD_CODE_URL`, `--cloud_code_endpoint=`, `AGY_LLM_GATEWAY_URL`, `AICODE_ENDPOINT_URL` and
`USER_INFO_OVERRIDE_URL`. Its log strings name them explicitly, for example
`[CLIAuthProvider] UpdateEndpointURL called with CLOUD_CODE_URL: %q`.

### What to intercept and what to leave alone

Sign-in, onboarding and the model catalog are tied to a real Google account. Forwarding those RPCs to
Google with the caller's own bearer keeps the native picker and quota display working, and only
generation needs translating. A junk bearer must come back with Google's own 401; that is the check
that proves the request was really forwarded rather than stubbed locally.

Because IDE callers present a Google OAuth token rather than an upstream key, the Cloud Code route
needs a server-side credential, which is exactly why it belongs behind an unguessable path or a
private network, unlike the CLI route where each caller can bring their own key.

## Verifying without a GUI

Protocol-level checks are enough to know the translation is right:

```bash
# CLI shape
curl -s -X POST "$BASE/v1beta/models/gemini-3.1-pro-preview:generateContent" \
  -H "X-Goog-Api-Key: $KEY" -H 'Content-Type: application/json' \
  -d '{"contents":[{"role":"user","parts":[{"text":"Reply exactly: OK"}]}],
       "generationConfig":{"maxOutputTokens":512}}'

# IDE shape
curl -s -X POST "$BASE/v1internal:generateContent" -H 'Content-Type: application/json' \
  -d '{"model":"gemini-3.1-pro-preview","requestId":"r1",
       "request":{"contents":[{"role":"user","parts":[{"text":"Reply exactly: OK"}]}],
                  "generationConfig":{"maxOutputTokens":512}}}'

# Passthrough (expect Google's 401, not a local stub)
curl -s -o /dev/null -w '%{http_code}\n' -X POST "$BASE/v1internal:loadCodeAssist" \
  -H 'Authorization: Bearer ya29.fake' -H 'Content-Type: application/json' -d '{}'
```

In-app behaviour still needs a real client on a machine that can sign in to Google.
