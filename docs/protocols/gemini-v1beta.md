# Gemini `v1beta` protocol

This is the surface the Antigravity CLI speaks. It is the public Gemini API shape, so any Gemini
client can talk to the bridge, not just Antigravity.

## Endpoints

```
POST {base}/v1beta/models/{model}:generateContent
POST {base}/v1beta/models/{model}:streamGenerateContent?alt=sse
POST {base}/v1beta/models/{model}:countTokens
```

Authentication is `x-goog-api-key: <key>` (also accepted as `?key=` or `Authorization: Bearer`).

## Request

```json
{
  "contents": [
    {"role": "user", "parts": [{"text": "list files in this directory"}]}
  ],
  "systemInstruction": {"parts": [{"text": "You are Antigravity…"}]},
  "tools": [{
    "functionDeclarations": [{
      "name": "run_command",
      "description": "Run a shell command",
      "parametersJsonSchema": {
        "type": "object",
        "properties": {"CommandLine": {"type": "string"}},
        "required": ["CommandLine"]
      }
    }]
  }],
  "generationConfig": {
    "maxOutputTokens": 65535,
    "thinkingConfig": {"includeThoughts": true, "thinkingBudget": 1024}
  }
}
```

Notes on each field:

| Field | Bridge behaviour |
| --- | --- |
| `systemInstruction` | Concatenated into one system message. |
| `contents[].role` | `user` and `model` map to user and assistant. Anything else is treated as user. |
| `parts[].text` | Prompt or answer text. `thought: true` parts are dropped from the prompt. |
| `parts[].functionCall` | Becomes an assistant `tool_calls` entry with a synthesised id. |
| `parts[].functionResponse` | Becomes a `role: tool` message. **Sent under role `model` by Antigravity.** |
| `parts[].inlineData` | Becomes an `image_url` data URI. |
| `tools[].functionDeclarations` | `parametersJsonSchema` is forwarded as OpenAI `parameters` verbatim. |
| `generationConfig.maxOutputTokens` | Clamped by `MAX_OUTPUT_TOKENS`. |
| `generationConfig.temperature` / `topP` | Forwarded. |
| `generationConfig.stopSequences` | Forwarded as `stop`. |
| `generationConfig.responseMimeType` | `application/json` becomes `response_format: json_object`. |
| `thinkingConfig` | Recorded but not translated; reasoning arrives from the upstream's own field. |

## Response

```json
{
  "candidates": [{
    "content": {"role": "model", "parts": [{"text": "There are three files."}]},
    "finishReason": "STOP",
    "index": 0
  }],
  "usageMetadata": {"promptTokenCount": 17, "candidatesTokenCount": 48, "totalTokenCount": 65},
  "modelVersion": "gemini-3.1-pro-preview"
}
```

A tool call looks like:

```json
{
  "candidates": [{
    "content": {
      "role": "model",
      "parts": [{
        "functionCall": {"name": "run_command", "args": {"CommandLine": "ls"}},
        "thoughtSignature": "WlpaWlpa…"
      }]
    },
    "finishReason": "STOP",
    "index": 0
  }],
  "modelVersion": "gemini-3.1-pro-preview"
}
```

`finishReason` is `STOP`, `MAX_TOKENS` or `SAFETY`.

## Streaming

`streamGenerateContent?alt=sse` returns `text/event-stream`. Each frame is a complete
`GenerateContentResponse`:

```
data: {"candidates":[{"content":{"role":"model","parts":[{"text":"There "}]},"index":0}]}

data: {"candidates":[{"content":{"role":"model","parts":[{"text":"are three files."}]},"index":0}]}

data: {"candidates":[{"content":{"role":"model","parts":[]},"finishReason":"STOP","index":0}],
       "usageMetadata":{"promptTokenCount":17,"candidatesTokenCount":48,"totalTokenCount":65}}

```

Reasoning deltas are interleaved as `{"text": "…", "thought": true}` parts. Tool calls are emitted as
one frame after the text, never split.

## countTokens

`countTokens` returns `{"totalTokens": <estimate>}` where the estimate is the request body length
divided by four. It is deliberately crude: Antigravity uses it for budgeting hints, not for billing.

## Testing by hand

```bash
curl -s -X POST "$BASE/v1beta/models/gemini-3.1-pro-preview:generateContent" \
  -H "x-goog-api-key: $KEY" -H 'Content-Type: application/json' \
  -d '{"contents":[{"role":"user","parts":[{"text":"Reply exactly: OK"}]}],
       "generationConfig":{"maxOutputTokens":512}}'
```

```bash
curl -sN -X POST "$BASE/v1beta/models/gemini-3.1-pro-preview:streamGenerateContent?alt=sse" \
  -H "x-goog-api-key: $KEY" -H 'Content-Type: application/json' \
  -d '{"contents":[{"role":"user","parts":[{"text":"Count to three."}]}],
       "generationConfig":{"maxOutputTokens":512}}'
```

A model name the bridge does not know is not an error: it resolves through `MODEL_MAP` and
`DEFAULT_MODEL`, so `gemini-anything` still answers.
