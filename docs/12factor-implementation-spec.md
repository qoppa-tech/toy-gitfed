# toy-gitfed 12-Factor Gap Closure Spec

## Goal
Close Must/Should gaps for factors III, IV, V, VIII, IX, X, XI, XII with low retrofit cost across `cloud`, `self-hosted`, `qoppainfra`, `dev/local`.

## Non-Goals
- No auth/domain behavior redesign.
- No schema redesign beyond migration-tool compatibility.
- No orchestration move (compose -> k8s) in this scope.

## Constraints
- Keep current module boundaries (`cmd/http-api`, `internal/*`, `pkg/logger`).
- Backward-compatible local dev defaults when safe.
- Production path must fail fast on invalid config.

---

## Phase 0: Baseline + Guardrails
### Scope
- Freeze integration points before refactor.

### Deliverables
- Spec this file committed.
- Work checklist tracked in issues/PR tasks.
- New env keys reserved:
  - `MIGRATE_ON_START`
  - `SHUTDOWN_TIMEOUT`
  - `HEALTHCHECK_TIMEOUT`
  - `APP_VERSION`

### Acceptance Criteria
1. Team has single source of truth for phase execution.
2. All new env keys documented in `README.md`.

---

## Phase 1: Graceful Shutdown + Disposability (Must)
### Scope
- Replace blocking `srv.Serve()` fatal path with signal-aware lifecycle.
- Drain inflight HTTP requests.
- Close Redis and Postgres pools on shutdown.

### Implementation Spec
1. In `cmd/http-api/main.go`:
   - Create `http.Server{Addr, Handler}`.
   - Start `ListenAndServe()` in goroutine.
   - Use `signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)`.
   - On signal:
     - Create timeout context (`SHUTDOWN_TIMEOUT`, default `15s`).
     - Call `server.Shutdown(ctx)`.
     - Close redis/db resources.
2. Treat `http.ErrServerClosed` as normal path.
3. Exit code:
   - `0` on clean shutdown.
   - non-zero on startup/run fatal errors.

### Acceptance Criteria
1. `docker compose stop api` triggers graceful stop, no forced kill under normal load.
2. Inflight request completes during grace window.
3. No leaked goroutines/resources in shutdown tests.

### Test Spec
- Unit/integration:
  - start server, send long request, trigger shutdown, assert request completes.
  - assert DB/Redis close invoked once.

---

## Phase 2: Config Validation Fail-Fast (Must)
### Scope
- Validate required runtime config before dependency init.
- Emit explicit missing/invalid keys.

### Implementation Spec
1. Add `Validate()` in `internal/config`.
2. Required keys by profile:
   - `ENV=dev`: allow current defaults except `REPOS_DIR` required.
   - `ENV!=dev`: require explicit:
     - `DB_HOST DB_PORT DB_USER DB_PASSWORD DB_NAME DB_SSLMODE`
     - `REDIS_HOST REDIS_PORT`
     - `HTTP_ADDR REPOS_DIR`
3. Validation checks:
   - port ranges `1..65535`.
   - parseable durations for timeout envs.
   - `REPOS_DIR` exists or creatable/writable.
4. `main`:
   - load config
   - run validate
   - on error: one structured log with list field `missing_vars`/`invalid_vars`, exit `1`.
5. Remove argv fallback for repo dir; env-only source of truth.

### Acceptance Criteria
1. Missing required vars fail startup before DB/Redis connect.
2. Error log includes exact var names.
3. Valid dev config still boots with minimal setup.

### Test Spec
- table tests in `internal/config`:
  - missing vars
  - invalid ports
  - valid dev/prod profiles

---

## Phase 3: Structured Logging + Request Logging (Must)
### Scope
- Ensure JSON structured logs in non-dev.
- Add HTTP request log middleware with stable fields.

### Implementation Spec
1. Reuse `pkg/logger` (`slog` based).
2. Ensure env mapping:
   - `ENV=dev` => human/dev handler allowed.
   - otherwise JSON handler mandatory.
3. Add/confirm request middleware order:
   - request-id -> request-log -> rate-limit -> handlers.
4. Request log fields:
   - `request_id`
   - `method`
   - `path`
   - `status`
   - `duration_ms`
   - `remote_ip`
   - `user_agent`
   - `bytes_out`
5. Standardize error field key as `error`.

### Acceptance Criteria
1. Each HTTP request emits exactly one completion log line.
2. Logs parse as JSON in non-dev.
3. Correlation works through `request_id`.

### Test Spec
- middleware tests:
  - status capture
  - duration presence
  - request id propagation
  - JSON parse check in non-dev logger path

---

## Phase 4: Migration Runner + Admin Process (Must)
### Scope
- Add deterministic migration execution for dev/prod parity.
- Expose operator commands.

### Tool Decision
- Use `golang-migrate` (CLI + optional lib) with SQL files.

### Implementation Spec
1. Migration layout:
   - convert/augment existing schema files to `*.up.sql` and `*.down.sql`.
2. Add Makefile targets:
   - `migrate-up`
   - `migrate-down`
   - `migrate-status`
3. Compose:
   - add `migrate` one-shot service (depends on healthy `psql`).
   - `api` depends on migrate completion success.
4. Optional app bootstrap:
   - if `MIGRATE_ON_START=true`, run up migrations before server start.
   - default false outside local.
5. CI:
   - run migrations before integration tests.

### Acceptance Criteria
1. Fresh DB from compose reaches latest schema before API serves traffic.
2. `make migrate-up/down` works from local shell.
3. Failed migration blocks API start.

### Test Spec
- integration boot test with empty volume.
- migration idempotency check (`up` twice no drift).

---

## Phase 5: Health Endpoint with Dependency Checks (Should)
### Scope
- Add readiness-style endpoint used by compose healthcheck.

### Implementation Spec
1. Add `GET /healthz`.
2. Check with short timeout context:
   - Postgres ping
   - Redis ping
3. Response:
   - `200` when both healthy: `{"status":"ok","version":"..."}`
   - `503` when degraded with component status map.

### Acceptance Criteria
1. Health reflects dependency outage within timeout.
2. Compose `api` healthcheck targets `/healthz`.

### Test Spec
- handler tests for healthy/degraded states.

---

## Phase 6: Build/Release/Run Hardening (Should)
### Scope
- Immutable image tagging with git SHA.

### Implementation Spec
1. `Makefile`:
   - `VERSION ?= $(shell git rev-parse --short HEAD)`
   - `build-image` tags:
     - `gitfed:$(VERSION)`
     - optional `gitfed:latest`
2. Inject `APP_VERSION` env at runtime/log startup.
3. Document release command examples.

### Acceptance Criteria
1. Builds produce SHA-tagged image.
2. Running service reports version in logs and `/healthz`.

---

## Phase 7: Dev/Prod Parity Closure (Should)
### Scope
- Ensure local lifecycle matches production bootstrap path.

### Implementation Spec
1. Compose default path includes migrations.
2. Add minimal admin commands:
   - seed data (`make seed` or `cmd/admin seed`)
   - migration status
3. Update `README.md` env matrix by environment type.

### Acceptance Criteria
1. New developer can boot fully working stack with one command.
2. Prod bootstrap sequence documented and reproducible.

---

## Cross-Phase File Touch Map
- `cmd/http-api/main.go`
- `internal/config/config.go` (+ new validation tests)
- `internal/api/http/http.go` (health route + middleware chain adjustments)
- `pkg/logger/*` (middleware/handler behavior)
- `internal/database/*` and `internal/store/redis.go` (health ping hooks if needed)
- `Makefile`
- `docker-compose.yaml`
- `migrations/schema/*` (or new migration dir naming)
- `README.md`

## Rollout Order
1. Phase 1
2. Phase 2
3. Phase 4
4. Phase 3
5. Phase 5
6. Phase 6
7. Phase 7

## Global Definition of Done
1. Startup fails fast with precise config errors.
2. API exits cleanly on `SIGINT/SIGTERM` within configured timeout.
3. Structured request logs emitted with correlation id.
4. Migrations automated in local + CI + compose startup flow.
5. Health endpoint reflects DB/Redis state.
6. Images versioned by immutable git SHA.
