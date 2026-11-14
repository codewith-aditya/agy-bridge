# Getting started

## What you need

- Go 1.22+ to build from source, or a released binary.
- An OpenAI-compatible endpoint and a key for it: OpenAI, OpenRouter, Groq, Together, vLLM,
  Together, a local vLLM, Ollama, or anything else that exposes `/v1/chat/completions`.
- Google Antigravity. The CLI is `agy`; the IDE is a separate download.

## 1. Build

```bash
git clone https://github.com/codewith-aditya/agy-bridge.git
cd agy-bridge
make build          # ./bin/agy-bridge
make cross          # linux/amd64, linux/arm64, windows/amd64, darwin/amd64, darwin/arm64
```

## 2. Run the bridge

Minimal, with your own upstream:

```bash
UPSTREAM_BASE=https://api.openai.com/v1 \
UPSTREAM_KEY=sk-... \
DEFAULT_MODEL=gpt-4o-mini \
./bin/agy-bridge
```

It logs one line and starts listening:

```
agy-bridge v0.1.0 listening on 127.0.0.1:8198 -> https://api.openai.com/v1 (default model "gpt-4o-mini")
```

Check it:

```bash
curl -s http://127.0.0.1:8198/health
```

```json
{"ok":true,"upstream":"https://api.openai.com/v1","default_model":"gpt-4o-mini","model_map":{}}
```

## Windows in four steps

No Go toolchain needed — grab the prebuilt binary.

```powershell
# 1. download
curl.exe -L -o "$env:USERPROFILE\Downloads\agy-bridge.exe" `
  https://github.com/codewith-aditya/agy-bridge/releases/download/v0.1.0/agy-bridge-windows-amd64.exe

# 2. put it somewhere permanent
mkdir "$env:USERPROFILE\tools\agy-bridge" -Force
move "$env:USERPROFILE\Downloads\agy-bridge.exe" "$env:USERPROFILE\tools\agy-bridge\"

# 3. run it (own terminal window)
$env:UPSTREAM_BASE="https://api.openai.com/v1"
$env:UPSTREAM_KEY="sk-..."
$env:DEFAULT_MODEL="gpt-4o-mini"
& "$env:USERPROFILE\tools\agy-bridge\agy-bridge.exe"
```

Step 4 — point the IDE at it. In its user `settings.json`
(`%APPDATA%\Antigravity IDE\User\settings.json`):

```json
{ "jetski.cloudCodeUrl": "http://127.0.0.1:8198" }
```

Restart the IDE afterwards. For the CLI instead of the IDE:

```powershell
$env:GOOGLE_GEMINI_BASE_URL="http://127.0.0.1:8198"
$env:GEMINI_API_KEY="unused"    # the bridge already holds UPSTREAM_KEY
agy
```

`agy-bridge.exe -version` should print `v0.1.0`.

## Building and running on Windows

`make` is a Unix tool, so build with `go` directly. Install Go first:

```powershell
winget install GoLang.Go
```

Open a new PowerShell window (so the toolchain lands on `PATH`), then:

```powershell
git clone https://github.com/codewith-aditya/agy-bridge.git
cd agy-bridge
go build -trimpath -ldflags "-s -w" -o agy-bridge.exe ./cmd/agy-bridge
.\agy-bridge.exe -version
```

Run the bridge in its own terminal window:

```powershell
$env:UPSTREAM_BASE="https://api.openai.com/v1"
$env:UPSTREAM_KEY="sk-..."
$env:DEFAULT_MODEL="gpt-4o-mini"
.\agy-bridge.exe
```

`$env:` variables live only for that window. To make them permanent, use `setx` and reopen the
terminal:

```powershell
setx UPSTREAM_BASE "https://api.openai.com/v1"
setx UPSTREAM_KEY "sk-..."
setx DEFAULT_MODEL "gpt-4o-mini"
```

Check it from a second window:

```powershell
curl.exe -s http://127.0.0.1:8198/health
```

A busy port shows up as `listen tcp 127.0.0.1:8198: bind: Only one usage of each socket address`;
change `LISTEN` or stop the other process.

To run it in the background without an open window, register it as a service with NSSM — see
[Deployment](deployment.md#windows-service).

## 3. Point the Antigravity CLI at it

`~/.gemini/config/settings.json` (`%USERPROFILE%\.gemini\config\settings.json` on Windows):

```json
{ "modelProvider": "gemini" }
```

Then export two variables and run the CLI:

```bash
export GOOGLE_GEMINI_BASE_URL=http://127.0.0.1:8198
export GEMINI_API_KEY=sk-...          # your upstream key, unless the bridge has UPSTREAM_KEY set
agy
```

Windows CMD:

```cmd
set GOOGLE_GEMINI_BASE_URL=http://127.0.0.1:8198
set GEMINI_API_KEY=sk-...
agy
```

PowerShell:

```powershell
$env:GOOGLE_GEMINI_BASE_URL="http://127.0.0.1:8198"
$env:GEMINI_API_KEY="sk-..."
agy
```

Make it permanent with `setx` on Windows, or by adding the exports to `~/.zshrc` / `~/.bashrc`.

## 4. Prove tool calling works

Text replies alone do not exercise the interesting half of the bridge. Run:

```bash
agy -p "create z.txt containing hello" --dangerously-skip-permissions
```

The file must actually appear in the working directory. That single command covers stream
translation, `functionCall` emission, `functionResponse` mapping and the multi-turn loop.

## 5. Point the Antigravity IDE at it

Add one setting to the IDE's user `settings.json`:

- Windows: `%APPDATA%\Antigravity IDE\User\settings.json`
- Linux: `~/.config/Antigravity IDE/User/settings.json`
- macOS: `~/Library/Application Support/Antigravity IDE/User/settings.json`

```json
{ "jetski.cloudCodeUrl": "http://127.0.0.1:8198" }
```

Restart the IDE and sign in with Google as usual. The IDE still needs its own sign-in because the
bridge forwards onboarding, the model catalog and quota calls to Google; only generation is
translated. Pick any model in the native picker and it will be served by your upstream.

## Which route do I use?

| Surface | Protocol | Setting |
| --- | --- | --- |
| CLI (`agy`) | Gemini `v1beta` | `GOOGLE_GEMINI_BASE_URL` |
| IDE | Cloud Code `v1internal` | `jetski.cloudCodeUrl` |
| Hub / language server | Cloud Code `v1internal` | `CLOUD_CODE_URL` or `--cloud_code_endpoint=` |

Both routes are served by the same binary on the same port; nothing needs to be started twice.

## Next

- [Configuration](configuration.md) to map Gemini model names onto your models.
- [Deployment](deployment.md) to put it behind TLS for more than one machine.
