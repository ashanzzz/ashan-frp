# Ashan FRP

> Go-first FRP management plane for Unraid NAS. One binary, embedded UI, Docker-friendly deployment, API-first control surface.

## Current status

- Go backend is the mainline implementation.
- Legacy flat demo files have been removed.
- The UI is embedded into the Go binary under `frp-backend/internal/web/dist/`.
- The runtime path is a single Go binary serving `/api/v1` and `/ui/`.

## What is in this repo

| Path | Purpose |
|---|---|
| `frp-backend/` | Go backend: HTTP API, SSE broker, state store, and frpc runtime manager |
| `frp-backend/internal/web/dist/` | Embedded static UI served by the Go binary |
| `docs/specs/ashan-frp/` | Architecture, API mapping, UI layout, and runtime docs |
| `Dockerfile` / `compose.yaml` / `.dockerignore` | Multi-stage build, local deployment, and lean build context |

## Run locally

```bash
cd frp-backend
mkdir -p /tmp/ashan-frp-state
HTTP_ADDR=127.0.0.1:18080 DATA_DIR=/tmp/ashan-frp-state go run ./cmd/ashan-frp
```

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

### 推荐：Compose

```bash
docker compose up -d --build
docker compose ps
docker compose logs -f ashan-frp
```

`compose.yaml` 会把 `./data` 挂载到容器的 `/app/data`，并继承镜像内置的健康检查。GitHub Actions 会在 `main` 推送后自动构建并发布 GHCR 镜像，发布的标签包含分支 / SHA / `latest`（默认分支）。如果你想直接使用仓库发布的镜像，可以先登录 GHCR，然后 `docker pull ghcr.io/ashanzzz/ashan-frp:latest`。

### 直接运行

```bash
docker build -t ashan-frp .
docker run --rm -p 8080:8080 -v $(pwd)/data:/app/data ashan-frp
```

The image builds into a single Go binary and keeps runtime state under `/app/data`.

## Front-end contract

The browser UI talks only to the local Go API and is implemented with static HTML/CSS/JavaScript embedded into the backend:

- list endpoints under `/api/v1`
- actions under `/api/v1/<resource>/{id}/actions/<action>`
- real-time updates through `/api/v1/events/stream`
- there is no separate runtime frontend server

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
- If the UI grows later, we can still prebuild assets before embedding them, but the final runtime is still the same: Go serves the static files.
- For this repo, the current choice is **plain JS + static HTML/CSS** embedded in Go.

## Notes

- The old shell scripts (`chmlfrp.sh`, `new_fix_flow.sh`) are historical references only.
- The retired standalone frontend prototype has been folded into the embedded UI.
- Design docs are the source of truth for the next implementation slice.
