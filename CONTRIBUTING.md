# Contributing

Thanks for taking the time to contribute. This project is small on purpose: one binary, no
dependencies, one translation layer. Keep it that way where you can.

## Getting started

```bash
git clone https://github.com/<owner>/agy-bridge.git
cd agy-bridge
make test vet build
```

Requirements: Go 1.22 or newer. Nothing else.

## Project layout

```
cmd/agy-bridge/        entrypoint: config + HTTP server wiring
internal/config/       environment parsing and model resolution
internal/translate/    the whole Gemini <-> OpenAI translation
internal/server/       routing, the two protocol handlers, passthrough
docs/                  reverse-engineering notes
```

Rules of thumb:

- `translate` must not import `server` or `config`. It is pure data in, data out, and it is where
  tests belong.
- `server` must not contain translation logic. If a handler starts doing string surgery on parts,
  move it into `translate`.
- No third-party modules. Standard library only, unless a dependency removes far more code than it
  adds and there is no alternative.

## Testing

```bash
make test          # go test ./...
make vet           # go vet ./...
make fmt           # gofmt -w .
```

Every behaviour change needs a test that fails before it. The two highest-value regressions to cover
are tool-result mapping and stream reassembly; both have tests in
`internal/translate/translate_test.go` already.

For anything that touches the wire format, prove it end to end against a real Antigravity client:

```bash
# CLI surface
GOOGLE_GEMINI_BASE_URL=http://127.0.0.1:8198 GEMINI_API_KEY=<key> \
  agy -p "create z.txt containing hello" --dangerously-skip-permissions
```

A text-only answer proves nothing about tool calling. The file must actually appear.

## Adding a provider

Most OpenAI-compatible providers need no code at all — set `UPSTREAM_BASE` and `UPSTREAM_KEY`.
If a provider deviates (non-standard tool-call streaming, missing `stream_options`, unusual finish
reasons), add the compatibility shim in `internal/translate`, behind a config flag, with a test that
captures the malformed payload.

## Commit messages

Conventional Commits:

```
fix(translate): map functionResponse sent under role=model
feat(server): add cloud code passthrough
docs(readme): document the IDE override
```

## Pull requests

1. One logical change per PR.
2. `make test vet fmt` must be clean; CI runs the same on Linux, macOS and Windows.
3. Describe the behaviour before and after, and paste the real command output that proves it.
4. New configuration must be documented in the README table in the same PR.

## Reporting bugs

Open an issue with: Antigravity surface (CLI or IDE), version, OS, the exact request path, and the
bridge log line. Redact credentials — never paste an API key or an OAuth token into an issue.

## Security

Do not open a public issue for a vulnerability. See [SECURITY.md](SECURITY.md).

## License

By contributing you agree that your work is licensed under the MIT license in [LICENSE](LICENSE).
