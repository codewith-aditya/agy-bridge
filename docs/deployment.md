# Deployment

Running the bridge on the same machine as Antigravity is the simple case. This page covers the other
one: a shared instance behind TLS.

## systemd (Linux)

`/etc/systemd/system/agy-bridge.service`:

```ini
[Unit]
Description=agy-bridge — Gemini/Cloud Code to OpenAI-compatible upstream
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=agy
WorkingDirectory=/opt/agy-bridge
EnvironmentFile=/etc/agy-bridge/agy-bridge.env
ExecStart=/opt/agy-bridge/agy-bridge
Restart=always
RestartSec=3
StandardOutput=append:/var/log/agy-bridge.log
StandardError=append:/var/log/agy-bridge.log
NoNewPrivileges=true
PrivateTmp=false
ProtectSystem=strict
ProtectHome=true
LimitNOFILE=65535

[Install]
WantedBy=multi-user.target
```

`/etc/agy-bridge/agy-bridge.env`:

```bash
LISTEN=127.0.0.1:8198
UPSTREAM_BASE=https://api.openai.com/v1
DEFAULT_MODEL=gpt-4o-mini
MODEL_MAP=gemini-3.1-pro-preview=gpt-4o,gemini-3-flash=gpt-4o-mini
MAX_OUTPUT_TOKENS=32000
```

```bash
sudo chown root:agy /etc/agy-bridge/agy-bridge.env
sudo chmod 640 /etc/agy-bridge/agy-bridge.env
sudo systemctl daemon-reload
sudo systemctl enable --now agy-bridge
systemctl status agy-bridge
curl -s http://127.0.0.1:8198/health
```

`PrivateTmp=false` is required when `DEBUG` writes to `/tmp`; otherwise those files land in the unit's
private namespace.

## nginx

Two blocks, because the two routes have different trust models. The CLI route can stay public — every
caller brings their own key. The Cloud Code route uses a server-side key, so give it an unguessable
prefix.

```nginx
# CLI route: callers bring their own upstream key
location ^~ /agy/ {
    proxy_pass http://127.0.0.1:8198/;
    proxy_http_version 1.1;
    proxy_set_header Connection "";
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
    proxy_buffering off;
    proxy_cache off;
    proxy_read_timeout 900s;
    proxy_send_timeout 900s;
    client_max_body_size 64m;
}

# IDE route: server-side key, so keep the prefix secret
location ^~ /agy-ide-CHANGE-ME/ {
    proxy_pass http://127.0.0.1:8198/;
    proxy_http_version 1.1;
    proxy_set_header Connection "";
    proxy_set_header Host $host;
    proxy_buffering off;
    proxy_cache off;
    proxy_read_timeout 1800s;
    proxy_send_timeout 1800s;
    client_max_body_size 128m;
}
```

Why each directive matters:

| Directive | Reason |
| --- | --- |
| `^~ /path/` | Prefix match that beats the site's catch-all and skips any global auth or branding filters. |
| trailing `/` in `proxy_pass` | Strips the prefix, so the bridge sees `/v1beta/...` and `/v1internal:...` unchanged. |
| `proxy_buffering off` | Streaming dies or arrives in one lump otherwise. |
| `proxy_read_timeout 900s+` | A single agent turn can run for minutes; the default 60s is not enough. |
| `client_max_body_size` | Antigravity sends a ~38 KB system prompt, and attachments inflate it fast. |
| `proxy_http_version 1.1` + empty `Connection` | Keep-alive for chunked SSE. |

Reload safely:

```bash
sudo nginx -t && sudo systemctl reload nginx
```

Old workers can serve a few in-flight requests after a reload. If a new path returns your site's
dashboard instead of the bridge, wait a second and retry before debugging anything.

## TLS

Terminate TLS at nginx (certbot or your existing certificates) and keep the bridge on loopback. The
bridge itself speaks plain HTTP by design; it has no certificate handling and should not be exposed
directly.

## Windows service

Run the bridge without keeping a PowerShell window open. [NSSM](https://nssm.cc) wraps any
executable as a Windows service:

```powershell
winget install NSSM.NSSM
```

```powershell
nssm install agy-bridge "C:\tools\agy-bridge\agy-bridge.exe"
nssm set agy-bridge AppEnvironmentExtra `
  UPSTREAM_BASE=https://api.openai.com/v1 `
  UPSTREAM_KEY=sk-... `
  DEFAULT_MODEL=gpt-4o-mini `
  LISTEN=127.0.0.1:8198
nssm set agy-bridge AppStdout "C:\tools\agy-bridge\agy-bridge.log"
nssm set agy-bridge AppStderr "C:\tools\agy-bridge\agy-bridge.log"
nssm set agy-bridge Start SERVICE_AUTO_START
nssm start agy-bridge
```

Check it:

```powershell
nssm status agy-bridge
curl.exe -s http://127.0.0.1:8198/health
```

Useful commands:

| Command | Effect |
| --- | --- |
| `nssm restart agy-bridge` | Restart after editing environment variables. |
| `nssm edit agy-bridge` | GUI for the service settings. |
| `nssm remove agy-bridge confirm` | Uninstall the service. |

A scheduled task works too (Task Scheduler → Create Task → "Run whether user is logged on or not",
trigger "At startup", action = the exe with arguments). NSSM is less fiddly.

Firewall: if the bridge should stay local, do not add an inbound rule for 8198. If another machine
must reach it, allow the port and set `LISTEN=0.0.0.0:8198`, ideally behind a TLS reverse proxy.

## Docker

```dockerfile
FROM golang:1.22-alpine AS build
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o /agy-bridge ./cmd/agy-bridge

FROM alpine:3.20
COPY --from=build /agy-bridge /usr/local/bin/agy-bridge
EXPOSE 8198
ENTRYPOINT ["agy-bridge"]
```

```bash
docker build -t agy-bridge .
docker run --rm -p 8198:8198 \
  -e LISTEN=0.0.0.0:8198 \
  -e UPSTREAM_BASE=https://api.openai.com/v1 \
  -e UPSTREAM_KEY=sk-... \
  -e DEFAULT_MODEL=gpt-4o-mini \
  agy-bridge
```

In a container, `0.0.0.0` is correct; publish the port to loopback only (`-p 127.0.0.1:8198:8198`)
when the proxy runs on the host.

## Hardening checklist

- [ ] `UPSTREAM_KEY` unset on any endpoint reachable from the internet.
- [ ] Cloud Code route behind a random path, VPN, or auth proxy.
- [ ] Bridge bound to `127.0.0.1` behind nginx.
- [ ] Env file `0600`/`0640`, owned by root, not in a shell profile.
- [ ] `DEBUG` off in production; the dumps contain full prompts.
- [ ] Log rotation for `/var/log/agy-bridge.log` (logrotate or journald).
- [ ] Rate limits at the proxy if the endpoint is shared, since the bridge has none.
