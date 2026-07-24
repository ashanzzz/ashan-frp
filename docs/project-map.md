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
    |-- FRPC runtime manager
    |-- Cloudflare / chmlfrp / OnePanel clients
    `-- structured JSON logs
```

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
| `docs/specs/ashan-frp/` | Architecture and product design references |
| `.github/workflows/build-push.yml` | Test, build, and GHCR publishing pipeline |

## 3. Current feature maturity

| Area | API | UI | Notes |
|---|---|---|---|
| Single-admin login/session | Complete | Complete | Argon2 password, lockout, session/API-token storage |
| Terminal credential recovery | Complete | Help entry | Interactive reset; old sessions/tokens are revoked |
| Dashboard | Complete | Complete | Health, counts, recent jobs, recent audit activity |
| Jobs and event timeline | Complete | Complete | Role-filtered event details |
| Cloudflare credential setup | Complete | Complete | Card-based setup; token never echoed; mask/ref/revision identify the credential |
| Cloudflare DNS records | Complete | Complete | Grouped focused UI with tunnel-managed labels; CRUD for A/AAAA/CNAME/TXT/MX/CAA |
| Structured logging | Complete | Complete via audit view | stdout plus rotating JSONL file; secret redaction |
| Audit search and details | Complete | Complete | Result, provider, actor, time, request ID, safe details |
| FRPC lifecycle | Complete | Complete | Start, stop, restart, status |
| Nodes | CRUD/sync complete | Read/sync focused | UI creation/edit forms remain a product gap |
| Tunnels | CRUD/provision complete | Complete via Control Center | Standalone tunnel view hidden; Control Center owns create/edit/delete/provision |
| Website mappings | CRUD/sync complete | Integrated / standalone views hidden | Website tunnel and mapping views are hidden from the simplified nav |
| Domains | Derived view | Integrated | Standalone nav hidden; domain state is surfaced through Control Center, statistics, and DNS |
| chmlfrp / OnePanel settings | Backend adapters exist | Card-based credential forms | Passwords/tokens stay write-only; save actions trigger backend validation |

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
- Provider secrets are encrypted at rest and never returned by the API.
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
4. Add chmlfrp and OnePanel credential forms with the same safe identity model used by Cloudflare.
5. Add backup/restore and database maintenance commands for operators.
6. Add integration health dashboards, retry controls, and provider-specific diagnostics.
7. Add end-to-end browser tests against a mock provider stack.

## 8. Codex workflow

- Keep stable repository guidance in `AGENTS.md`; keep task-specific intent in the current task.
- Use skills for reusable workflows, MCP/connectors for live external systems, and browser control for UI smoke tests.
- Prefer one task/branch per coherent change. Run `scripts/verify.ps1` before publishing.
- Never store Codex session state, browser state, local logs, generated binaries, or credentials in Git.
