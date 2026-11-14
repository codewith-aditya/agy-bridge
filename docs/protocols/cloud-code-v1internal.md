# Cloud Code protocol (`v1internal`)

The Antigravity IDE does not use the public Gemini API. It is a VS Code derivative that talks to
Google's internal Cloud Code service at `cloudcode-pa.googleapis.com`, using RPC-style paths named
`v1internal:*`.

## The switch

The IDE ships an override for that base URL. In `resources/app/out/main.js`:

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

So one user setting reroutes the whole client:

```json
{ "jetski.cloudCodeUrl": "http://127.0.0.1:8198" }
```

The bundled hub / language server is the same Go binary as the CLI and carries parallel knobs:
`CLOUD_CODE_URL` (environment), `--cloud_code_endpoint=` (flag), plus `AGY_LLM_GATEWAY_URL`,
`AICODE_ENDPOINT_URL` and `USER_INFO_OVERRIDE_URL`.

## RPC surface

| RPC | Handled by |
| --- | --- |
| `v1internal:generateContent` | translated to upstream |
| `v1internal:streamGenerateContent` | translated to upstream |
| `v1internal:countTokens` | local estimate |
| `v1internal:loadCodeAssist` | forwarded to Google |
| `v1internal:onboardUser` | forwarded to Google |
| `v1internal:fetchAvailableModels` | forwarded to Google |
| `v1internal:fetchUserInfo` | forwarded to Google |
| `v1internal:fetchAdminControls` | forwarded to Google |
| anything else `v1internal:*` | forwarded to Google |

Forwarding keeps the IDE's own Google sign-in, native model picker, quota display and account tier
working. Reimplementing those would mean reimplementing an account system, which is out of scope.

## Envelope

Request:

```json
{
  "model": "gemini-3.1-pro-preview",
  "project": "cloudaicompanion-…",
  "requestId": "…",
  "request": {
    "contents": [{"role": "user", "parts": [{"text": "hello"}]}],
    "generationConfig": {"maxOutputTokens": 65535}
  }
}
```

Response:

```json
{
  "response": {
    "candidates": [{
      "content": {"role": "model", "parts": [{"text": "hi"}]},
      "finishReason": "STOP",
      "index": 0
    }],
    "modelVersion": "gemini-3.1-pro-preview"
  },
  "traceId": "…"
}
```

Streaming frames are the same envelope, one per SSE event:

```
data: {"response":{"candidates":[{"content":{"role":"model","parts":[{"text":"hi"}]},"index":0}]},"traceId":"…"}

```

The inner `request` object is exactly a Gemini `GenerateContentRequest`, so the same translator is
reused. Only the wrapper changes.

## Credentials

The IDE authenticates with a Google OAuth access token in `Authorization: Bearer`. No upstream will
accept that as an API key, so this route needs a server-side credential:

```bash
CLOUD_SERVER_KEY=sk-...
```

Because every IDE user then spends the operator's balance, put this route behind an unguessable path,
a VPN, or an authenticating reverse proxy. See [Deployment](../deployment.md).

## Verifying

Generation:

```bash
curl -s -X POST "$BASE/v1internal:generateContent" -H 'Content-Type: application/json' \
  -d '{"model":"gemini-3.1-pro-preview","requestId":"r1",
       "request":{"contents":[{"role":"user","parts":[{"text":"Reply exactly: OK"}]}],
                  "generationConfig":{"maxOutputTokens":512}}}'
```

Passthrough. A junk token must come back with **Google's own** 401, not a local stub:

```bash
curl -s -o /dev/null -w '%{http_code}\n' -X POST "$BASE/v1internal:loadCodeAssist" \
  -H 'Authorization: Bearer ya29.fake' -H 'Content-Type: application/json' -d '{}'
```

```json
{"error":{"code":401,"message":"Request had invalid authentication credentials.…","status":"UNAUTHENTICATED"}}
```

If you get a 200 or a local error instead, the passthrough is misconfigured — check
`CLOUD_CODE_PASSTHROUGH_BASE`.

## What this cannot do

- Skip Google sign-in. Onboarding and the catalog are Google-side.
- Add custom models to the native picker. The picker lists what the upstream catalog returns; the
  bridge maps names behind the scenes rather than injecting new entries.
- Preserve Google-side features that depend on the real API (grounding, cached content, safety
  settings). Those fields are dropped.
