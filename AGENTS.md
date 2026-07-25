# Ashan FRP repository guidance

## Product boundary

- The supported product is the Go management plane under `frp-backend/`.
- The production runtime is one Go binary serving `/api/v1` and the embedded `/ui/` console.
- Docker / Compose is the primary deployment path. Keep changes Linux-container compatible.
- Root JSON, TXT, ENV, and LOG files are legacy migration fixtures or historical evidence unless `docs/project-map.md` says otherwise.
- DNS record management discipline: ashan-frp only has permission to edit or delete DNS records created by or claimed by ashan-frp (or 1Panel). Original pre-existing Cloudflare DNS records are locked/read-only by default until manually claimed.

## Start here

1. Read `docs/project-map.md` for the current product and code map.
2. Read `docs/specs/ashan-frp/roadmap.md` before planning a large feature.
3. For backend or embedded UI changes, also follow `frp-backend/AGENTS.md`.

## Security rules

- Never commit passwords, API tokens, session cookies, Authorization headers, encrypted blobs, or real credential exports.
- Use placeholders in examples and tests. Treat any credential seen in chat, logs, fixtures, or Git history as compromised.
- Cloudflare credentials may be represented only by `token_mask`, `credential_ref`, and `credential_revision` outside encrypted storage.
- Do not add a public forgot-password or unauthenticated password-reset API. Administrator recovery remains terminal-only.
- Do not log raw request/response bodies or unrestricted headers.

## Change discipline

- Fix root causes and keep changes scoped to the requested product behavior.
- Preserve the single-admin constraint unless a feature explicitly redesigns account and role management.
- Keep API, repository, audit, structured-log, OpenAPI, UI, and tests in sync for management operations.
- Do not edit generated/runtime snapshots merely to make tests pass.
- Do not commit Codex, browser, Playwright, build, or local runtime state.

## Validation

- Preferred local check: `powershell -ExecutionPolicy Bypass -File scripts/verify.ps1`.
- Backend module: `cd frp-backend` then run `go test ./...`, `go vet ./...`, and `node --test internal/web/app.test.mjs`.
- Docker smoke verification is optional locally and required when Docker is available: `powershell -ExecutionPolicy Bypass -File scripts/docker-smoke.ps1`.
- The root GitHub Actions workflow is the release source of truth. Do not add nested workflows under `frp-backend/.github/`.

## 🛑 CRITICAL: Anti-Regression & Documentation (防退化与文档更新强制要求)

**TO PREVENT REGRESSIONS AND "FORGETTING" PREVIOUSLY IMPLEMENTED FEATURES:**

1. **Mandatory Documentation Update**: EVERY time a new feature, bug fix, or logic change is implemented, you MUST write the new functionalities and changes into the project documentation (`CHANGELOG.md`, `docs/project-map.md`, or relevant specs). Do this BEFORE committing. This records the project's volatility and state.
2. **Read Before Modifying**: Before changing existing frontend UI (especially Vue components like `App.vue`) or backend handlers, you MUST read the `CHANGELOG.md` and `docs/project-map.md` to understand the current established logic (e.g. ChmlFrp OAuth auto-login, Cloudflare Zone auto-fetching, Uptime-Kuma UI styles). 
3. **Preserve Existing Logic**: Do NOT overwrite or strip out existing complex logic just to satisfy a new simple requirement. If you are asked to change the UI or backend, ensure you do not inadvertently delete or break existing interactions established in previous sessions. "Fixing something this time should not cause it to be lost next time."

## Documentation

- Maintain a `CHANGELOG.md` at the project root. For every feature update or bug fix, append a clear description of the new functionalities and changes to keep track of project evolution.
- Update `docs/project-map.md` when ownership, runtime boundaries, or feature maturity changes.
- Update `frp-backend/internal/http/openapi.json` when HTTP contracts change.
- Update `README.md` for operator-visible commands, environment variables, deployment behavior, and recovery procedures.
