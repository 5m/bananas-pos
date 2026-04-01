# Bananas POS

Bananas POS is a small desktop tray app that accepts ZPL over HTTP and raw TCP, then routes jobs to a configurable output target.

The app is built with Go and Fyne. At runtime it starts one process, enforces single-instance semantics, exposes local listeners, and keeps a minimal settings UI in the system tray.

## Runtime model

- HTTP input defaults to `:9180`
- TCP input defaults to `:9100`
- `POST /zpl` submits print payloads over HTTP and rejects empty payloads
- `GET /_/health` reports target health and returns a non-200 response when the active target is unhealthy
- Browser access to the HTTP server is CORS-enabled

Incoming payloads are split into labels and forwarded as `job.PrintJob` units to the active target.

The tray settings window persists the selected target mode, printer, transform, and listener ports between launches. Output mode and printer changes apply immediately. HTTP/TCP listener changes are saved but require an app restart to take effect.

## Target modes

- `system-print-queue`: submits raw label payloads to the host platform print queue and checks that the selected printer is available
- `http-proxy`: forwards jobs to another HTTP endpoint, proxies HTTP traffic through to that upstream, and reports unhealthy if the upstream returns a non-2xx health response
- `emulator`: renders label previews in a local window via Labelary

## Transforms

- `None`: forwards incoming payloads unchanged
- `Epson ESC/POS (debug)`: available for `system-print-queue`; renders ZPL through Labelary and converts the preview into ESC/POS raster output

## Configuration

Environment variables:

- `HTTP_ENABLED` default `true`
- `HTTP_LISTEN_ADDR` default `:9180`
- `TCP_ENABLED` default `true`
- `TCP_LISTEN_ADDR` default `:9100`
- `TARGET_MODE` default `system-print-queue`
- `PRINTER_NAME` default empty; when no printer is configured, the app resolves the system default printer at startup
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
