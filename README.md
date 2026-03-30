# Bananas POS

Bananas POS is a small desktop tray app that accepts ZPL over HTTP and raw TCP, then routes jobs to a configurable output target.

The app is built with Go and Fyne. At runtime it starts one process, enforces single-instance semantics, exposes local listeners, and keeps a minimal settings UI in the system tray.

## Runtime model

- HTTP input defaults to `:9180`
- TCP input defaults to `:9100`
- `POST /zpl` submits print payloads over HTTP
- `GET /_/health` reports basic target health
- Browser access to the HTTP server is CORS-enabled

Incoming payloads are split into labels and forwarded as `job.PrintJob` units to the active target.

## Target modes

- `system-print-queue`: submits raw label payloads to the host platform print queue and checks that a default printer is available
- `http-proxy`: forwards jobs to another HTTP endpoint and can proxy HTTP traffic through to that upstream
- `emulator`: renders label previews in a local window via Labelary

## Configuration

Environment variables:

- `HTTP_ENABLED` default `true`
- `HTTP_LISTEN_ADDR` default `:9180`
- `TCP_ENABLED` default `true`
- `TCP_LISTEN_ADDR` default `:9100`
- `TARGET_MODE` default `system-print-queue`
- `PROXY_HTTP_URL` default `http://localhost:9100`
- `EMULATOR_DPMM` default `8`

## Development

Run locally:

```bash
go run ./cmd/bananas-pos
```

Build:

```bash
make build
```

Package:

```bash
make package-linux
make package-macos
make package-windows
```

Notes:

- macOS packaging produces a `.app` bundle and ad hoc re-signs it during `make package-macos`
- the Windows package target expects the workflow toolchain setup from `.github/workflows/release.yml`
