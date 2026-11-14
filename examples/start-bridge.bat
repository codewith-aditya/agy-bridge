@echo off
REM Example launcher for Windows. Fill in your own upstream and key.
REM Then point Antigravity at http://127.0.0.1:8198 with GEMINI_API_KEY set to anything.

set UPSTREAM_BASE=https://api.openai.com/v1
set UPSTREAM_KEY=sk-REPLACE_ME
set DEFAULT_MODEL=gpt-4o-mini
set LISTEN=127.0.0.1:8198

agy-bridge.exe
pause
