# Changelog

All notable changes to this project are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and the project adheres to
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.1.0] - 2026-11-14

### Added

- Gemini `v1beta` surface for the Antigravity CLI: `generateContent`, `streamGenerateContent`
  and `countTokens`.
- Cloud Code `v1internal` surface for the Antigravity IDE and hub, with selective passthrough of
  non-generation RPCs to Google using the caller's own bearer token.
- OpenAI-compatible request/response translation, including SSE streaming, reasoning deltas and
  function calling.
- Model resolution through `DEFAULT_MODEL` and `MODEL_MAP`.
- `MAX_OUTPUT_TOKENS` clamp, because Antigravity requests 65535 tokens.
- Optional caller-supplied credentials, so a shared deployment cannot spend the operator's balance.
- Static builds for Linux, macOS and Windows; unit tests for translation and stream reassembly.

### Fixed

- Tool results sent under role `model` (as Antigravity does) were previously mapped to an empty
  assistant message, silently breaking the agent loop.
- `functionResponse` parts carry no call id, so tool messages now use synthesised ids matched per
  function name in FIFO order.
- Emitted `functionCall` parts now carry a placeholder `thoughtSignature`, which Gemini 3 era
  clients round-trip on the next turn.

[Unreleased]: https://github.com/codewith-aditya/agy-bridge/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/codewith-aditya/agy-bridge/releases/tag/v0.1.0
