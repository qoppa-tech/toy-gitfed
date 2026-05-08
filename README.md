# gitfed

Federated git forge toy project by qoppatech.

## Quick Start (dev/local)

1. Copy env file:
   `cp .env.example .env`
2. Boot full stack (postgres, redis, migrations, api):
   `make compose-up`
3. Verify health:
   `curl -fsS http://localhost:8080/healthz`

Single-command bootstrap path is `make compose-up` (includes migration + seed jobs before API starts).

## Admin Commands

- Apply migrations: `make migrate-up`
- Roll back last migration: `make migrate-down`
- Migration status/version: `make migrate-status`
- Seed baseline data: `make seed`

Seed creates/updates:
- user `admin` (`admin@gitfed.local`, password `admin123`)
- organization `Gitfed Team`
- repository `hello-gitfed`

## Environment Matrix

All environments use the same config keys and migration flow.

| Key | dev/local | self-hosted | cloud | qoppainfra |
|---|---|---|---|---|
| `ENV` | `DEV` | `PROD` | `PROD` | `PROD` |
| `HTTP_ADDR` | `0.0.0.0:8080` | required | required | required |
| `REPOS_DIR` | `/data/repos` | required | required | required |
| `DB_HOST` | `psql` | required | required | required |
| `DB_PORT` | `5432` | required | required | required |
| `DB_USER` | `gitfed` | required | required | required |
| `DB_PASSWORD` | `gitfed` | required | required | required |
| `DB_NAME` | `gitfed` | required | required | required |
| `DB_SSLMODE` | `disable` | required | required | required |
| `REDIS_HOST` | `redis` | required | required | required |
| `REDIS_PORT` | `6379` | required | required | required |
| `REDIS_PASSWORD` | empty | optional/required by policy | optional/required by policy | optional/required by policy |
| `REDIS_DB` | `0` | `0` (or policy) | `0` (or policy) | `0` (or policy) |
| `SHUTDOWN_TIMEOUT` | `15s` | required | required | required |
| `HEALTHCHECK_TIMEOUT` | `2s` | required | required | required |
| `APP_VERSION` | `dev` | release version/SHA | release version/SHA | release version/SHA |

Optional SSO keys:
- `GOOGLE_CLIENT_ID`
- `GOOGLE_CLIENT_SECRET`
- `GOOGLE_REDIRECT_URL`

## Production Bootstrap Sequence

Use this same order in self-hosted, cloud, and qoppainfra:

1. Provision Postgres + Redis.
2. Export required env vars.
3. Run migrations (`make migrate-up` or migration job).
4. Start API image tagged with immutable version (`gitfed:<sha>`).
5. Wait for `/healthz` readiness before routing traffic.
