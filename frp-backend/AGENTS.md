# Go backend and embedded UI guidance

## Architecture

- `cmd/ashan-frp/`: process entry points and administrator CLI.
- `internal/http/`: routing, middleware, handlers, OpenAPI, and SSE.
- `internal/repository/`: persistence operations and transactions.
- `internal/integration/`: external provider clients.
- `internal/frpc/`: local FRPC process/config management.
- `internal/worker/`: queued background work and retention tasks.
- `internal/observability/`: structured logs, redaction, and file rotation.
- `internal/web/dist/`: source-controlled embedded HTML/CSS/JavaScript bundle.

## Backend rules

- Use repository methods instead of ad-hoc SQL in handlers.
- Management mutations must emit a safe audit event and structured server logs with request/trace correlation.
- External provider failures must map to stable safe error codes; never return raw upstream bodies or secrets.
- Keep SQLite access compatible with the single shared connection configured in `internal/database`.
- Run `gofmt` on changed Go files. Do not introduce a new CLI or web framework without a clear product need.

## Embedded UI rules

- There is no separate production frontend server or package manager build.
- UI requests must use the local `/api/v1` API with same-origin credentials.
- Saved tokens and passwords must never be rendered back into forms or DOM snapshots.
- Every new operation needs clear loading, success, failure, empty, and disabled states.
- Update the cache-busting query in `internal/web/dist/index.html` whenever `app.js` or `styles.css` behavior changes.
- Extend `internal/web/app.test.mjs` for security-sensitive and critical interaction changes.

## Tests

- Start with the narrow package or Node test that covers the change.
- Before handoff, run `go test ./...`, `go vet ./...`, `node --test internal/web/app.test.mjs`, and a production build.
- For authentication, settings, audit, DNS, or concurrency changes, include regression coverage rather than relying only on browser smoke tests.
