# Ashan FRP

> Go-first FRP management plane for Unraid NAS. One binary, embedded UI, Docker-first deployment, API-first control surface.

## Current status

- Go backend is the mainline implementation.
- Legacy flat demo files have been removed.
- The UI is embedded into the Go binary under `frp-backend/internal/web/dist/`.
- The runtime path is a single Go binary serving `/api/v1` and `/ui/`.
- Production deployment is expected to run in Docker / Compose; do not assume a Windows-native runtime as the primary path.

## What is in this repo

| Path | Purpose |
|---|---|
| `frp-backend/` | Go backend: HTTP API, SSE broker, state store, and frpc runtime manager |
| `frp-backend/internal/web/dist/` | Embedded static UI served by the Go binary |
| `docs/specs/ashan-frp/` | Architecture, API mapping, UI layout, and runtime docs |
| `Dockerfile` / `compose.yaml` / `.dockerignore` | Multi-stage build, local deployment, and lean build context |

## Run locally

### Preferred: Docker / Compose

```bash
docker compose up -d --build
docker compose ps
docker compose logs -f ashan-frp
```

The container mounts `./data` into `/app/data` and uses the image's healthcheck. This is the primary way to run the product.

### Development-only: Go backend

```bash
cd frp-backend
mkdir -p /tmp/ashan-frp-state
HTTP_ADDR=127.0.0.1:18080 DATA_DIR=/tmp/ashan-frp-state go run ./cmd/ashan-frp
```

Use the direct Go command for local development or troubleshooting. Do not treat the Windows host environment as the production runtime target.

## Main endpoints

| Method | Path | Purpose |
|---|---|---|
| `GET` | `/api/v1/version` | Version and runtime info |
| `GET` | `/api/v1/health` | Health snapshot |
| `GET` | `/api/v1/nodes` | Node list |
| `GET` | `/api/v1/tunnels` | Tunnel list |
| `GET` | `/api/v1/website-mappings` | Website mapping list |
| `GET` | `/api/v1/settings` | Settings snapshot |
| `GET` | `/api/v1/jobs` | Job list |
| `GET` | `/api/v1/frpc/runtime` | FRPC runtime summary |
| `GET` | `/api/v1/events/stream` | SSE event stream |
| `GET` | `/api/openapi.json` | OpenAPI JSON |
| `GET` | `/api/docs` | Lightweight docs page |
| `GET` | `/ui/` | Embedded UI entry |

## Docker

### Recommended: Compose

```bash
docker compose up -d --build
docker compose ps
docker compose logs -f ashan-frp
```

`compose.yaml` mounts `./data` to `/app/data` and inherits the image healthcheck. GitHub Actions builds and publishes GHCR images on pushes to `main`; tags include branch, SHA, and `latest` on the default branch. To pull the published image, log in to GHCR and run `docker pull ghcr.io/ashanzzz/ashan-frp:latest`.

### Direct run

```bash
docker build -t ashan-frp .
docker run --rm -p 8080:8080 -v $(pwd)/data:/app/data ashan-frp
```

The image builds into a single Go binary and keeps runtime state under `/app/data`.

### Docker smoke check

On Windows PowerShell, you can run the repo-provided Docker smoke test:

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\docker-smoke.ps1
```

This script:

- builds and starts the Compose stack
- waits for the container healthcheck
- verifies `/api/v1/health`, `/api/v1/version`, `/api/v1/openapi.json`, `/api/v1/docs`
- verifies the compatibility aliases `/api/openapi.json` and `/api/docs`
- tears the stack down automatically unless `-KeepRunning` is passed

## Front-end contract

The browser UI talks only to the local Go API and is implemented with static HTML/CSS/JavaScript embedded into the backend:

- list endpoints under `/api/v1`
- actions under `/api/v1/<resource>/{id}/actions/<action>`
- real-time updates through `/api/v1/events/stream`
- there is no separate runtime frontend server
- UI changes should remain compatible with Docker-first deployment

## How this project is split

This repo uses a very simple split:

- **Backend = Go**
  - HTTP API
  - SSE event stream
  - local state storage
  - job runner
  - FRPC runtime manager

- **Frontend = browser code**
  - HTML + CSS + JavaScript
  - runs in the browser, not in Go
  - served by the Go binary under `/ui/`
  - embedded into the binary with `go:embed`

### What this means in practice

- We are **continuing on the existing project**, not starting a new one.
- The current `frp-backend/` tree is the main source of truth.
- We do **not** run a separate frontend server in production.
- Docker / Compose is the primary deployment target; any local host-specific shortcut must remain development-only.
- If the UI grows later, we can still prebuild assets before embedding them, but the final runtime is still the same: Go serves the static files.
- For this repo, the current choice is **plain JS + static HTML/CSS** embedded in Go.

## Notes

- The old shell scripts (`chmlfrp.sh`, `new_fix_flow.sh`) are historical references only.
- The retired standalone frontend prototype has been folded into the embedded UI.
- Design docs are the source of truth for the next implementation slice.
- Keep new code Docker-compatible; avoid Windows-only runtime assumptions.

