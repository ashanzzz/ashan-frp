# Ashan FRP project map

This document is the working product and code map for maintainers and coding agents. It distinguishes the supported Go product from historical migration artifacts.

## 1. Product definition

Ashan FRP is a single-administrator FRP operations console for Unraid and Docker deployments. One Go binary owns authentication, state, jobs, provider integrations, FRPC lifecycle, the HTTP API, and the embedded browser UI.

```text
Browser /ui/
    |
    v
Go API /api/v1
    |-- SQLite state and audit
    |-- background worker
    |-- FRPC runtime manager (Local Daemon)
    |-- Cloudflare / chmlfrp / OnePanel clients (Upstream Providers)
    `-- structured JSON logs
```

### 1.1 Service Boundary: ChmlFrp Upstream Provider vs Local FRPC Daemon

It is critical to distinguish between the **upstream provider service** and the **local client daemon**:

1. **ChmlFrp Upstream Provider (ChmlFrp 服务商)**:
   - ChmlFrp is a third-party FRP node & tunnel service provider (`https://cf-v2.uapis.cn`).
   - It provides remote server nodes (`Nodes`), web hosting capabilities (`wed`), anti-DDoS notes (`fangyu`), and tunnel forwarding rule definitions (`ChmlFrp Tunnels`).
   - Our system interacts with ChmlFrp via OAuth2 / API Tokens to manage nodes, fetch configurations, and sync tunnel rules.

2. **Local FRPC Daemon (FRPC 本地客户端守护进程)**:
   - FRPC is a single, embedded Go binary daemon running locally inside the `ashan-frp` environment.
   - It takes the generated `frpc.toml` (which configures local ports to connect to ChmlFrp's remote nodes) and manages the local FRPC process lifecycle (start, stop, restart, reload, stdout logs).

## 2. Source-of-truth directories

| Path | Ownership |
|---|---|
| `frp-backend/cmd/ashan-frp/` | Server and terminal administrator commands |
| `frp-backend/internal/http/` | API routes, auth middleware, handlers, SSE, OpenAPI |
| `frp-backend/internal/domain/` | Persistent models and API payloads |
| `frp-backend/internal/repository/` | Database access and transactions |
| `frp-backend/internal/integration/` | Cloudflare, chmlfrp, and OnePanel adapters |
| `frp-backend/internal/frpc/` | FRPC process and configuration runtime |
| `frp-backend/internal/worker/` | Jobs, retries, and retention work |
| `frp-backend/internal/observability/` | JSON logging, redaction, fingerprints, rotation |
| `frp-backend/internal/web/dist/` | Embedded operations console |
| `docs/specs/ashan-frp/` | Architecture, approved product design, active remediation plans, and the mandatory change-safety contract |
| `.github/workflows/build-push.yml` | Test, build, and GHCR publishing pipeline |

## 3. Current feature maturity

| Area | API | UI | Notes |
|---|---|---|---|
| Single-admin login/session | Complete | Complete | Argon2 password, lockout, session/API-token storage |
| Terminal credential recovery | Complete | Help entry | Interactive reset; old sessions/tokens are revoked |
| Dashboard | Complete | Complete | Health, counts, recent jobs, recent audit activity |
| Jobs and event timeline | Complete | Complete | Role-filtered event details |
| Cloudflare credential setup | Complete | Complete | Auto-detects scoped API Token vs Global API Key, requests email only for Global Key auth, auto-saves a single verified Zone, and prompts for selection when multiple Zones are accessible; the authenticated personal-project settings view intentionally shows the full secret |
| Cloudflare DNS records | Complete | Complete | Grouped UI with sync polling, origin tags (ashan-frp/1Panel/原生), claim/unclaim, and original record protection |
| Structured logging | Complete | Complete via audit view | stdout plus rotating JSONL file; secret redaction |
| Audit search and details | Complete | Complete | Result, provider, actor, time, request ID, safe details |
| FRPC lifecycle | Complete | Complete | Start, stop, restart, status |
| Nodes | CRUD/sync complete | Read/sync focused | UI creation/edit forms remain a product gap |
| Tunnels | CRUD/provision complete | Complete via Control Center | Standalone tunnel view hidden; Control Center owns create/edit/delete/provision |
| Website mappings | CRUD/sync complete | Integrated / standalone views hidden | Website tunnel and mapping views are hidden from the simplified nav |
| Domains | Derived view | Integrated | Standalone nav hidden; domain state is surfaced through Control Center, statistics, and DNS |
| chmlfrp / OnePanel settings | Backend adapters exist | Card-based credential forms | ChmlFrp saved secrets are intentionally displayed in full in the authenticated Settings Center; secrets remain encrypted at rest and excluded from logs/audits |

## 4. Runtime and deployment

- Primary image: root `Dockerfile` built by `.github/workflows/build-push.yml`.
- Primary local deployment: `compose.yaml`.
- Persistent data: `DATA_DIR`, normally `/app/data` in the container.
- Database: SQLite at `DATABASE_DSN`, using WAL and a single shared connection to avoid write-contention authentication failures.
- Logs: stdout and `DATA_DIR/logs/ashan-frp.jsonl` by default.
- UI: compiled into the Go binary with `go:embed`; no separate frontend process exists in production.

## 5. Security invariants

- Exactly one `admin` or `super_admin` account is supported.
- Passwords are irreversible hashes and cannot be listed or recovered.
- Forgot-password recovery requires server/container terminal access.
- Provider secrets are encrypted at rest. The only API plaintext exception is the authenticated single-admin `GET /api/v1/settings` response required by the personal-project settings UI.
- Logs and audits contain only approved safe fields; secret-like fields are redacted.
- Cloudflare credential identity is `token_mask + credential_ref + credential_revision`, not the original token.
- `userdata.txt` and real provider exports are local-only and must never be committed.

## 6. Legacy and reference assets

The following root files are not runtime inputs to the Go product. They are migration fixtures or historical evidence and should not be expanded into new product logic:

- `CHANGELOG-OAUTH2.md`
- `settings.env`
- `fixed_tunnels.txt` and `exempt_names.txt`
- `cloudflare_dns_*.json` and `chmlfrp隧道列表_*.json`
- `临服-固定隧道标准.json`
- `日志-新修复流程.log`
- `SHA256SUMS.txt`

If their remaining knowledge is needed, migrate the useful rules into `docs/specs/ashan-frp/` or tests, then remove the fixture in a dedicated cleanup change.

## 7. Recommended next product slices

1. Complete node create/edit/archive UI with validation and permission-aware actions.
2. Complete tunnel create/edit/delete UI, including protocol-specific forms and dry-run diff.
3. Complete website mapping create/edit UI and OnePanel association flow.
4. Complete explicit ChmlFrp credential-mode handling (legacy username/password versus API/OAuth Token), verified current-account display, and provider failure diagnostics without weakening the authenticated plaintext-display boundary.
5. Add backup/restore and database maintenance commands for operators.
6. Add integration health dashboards, retry controls, and provider-specific diagnostics.
7. Add end-to-end browser tests against a mock provider stack.

## 8. Codex workflow

- Keep stable repository guidance in `AGENTS.md`; keep task-specific intent in the current task.
- Use skills for reusable workflows, MCP/connectors for live external systems, and browser control for UI smoke tests.
- Prefer one task/branch per coherent change. Run `scripts/verify.ps1` before publishing.
- Never store Codex session state, browser state, local logs, generated binaries, or credentials in Git.
