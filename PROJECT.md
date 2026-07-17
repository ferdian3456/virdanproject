# Virdan Project

Virdan is a community/server-based social backend, conceptually similar to Discord: users join
topic-based "servers," post content, comment, react, chat via DMs, and receive push notifications.
It is a single Go binary composed of modular services behind a Fiber HTTP API, backed by
PostgreSQL, Redis, and MinIO, with a full OpenTelemetry observability stack.

## Tech Stack

| Concern | Technology |
|---|---|
| Language | Go 1.26 |
| HTTP framework | [Fiber v3](https://github.com/gofiber/fiber) |
| Database | PostgreSQL (via `pgx/v5`) |
| Cache / sessions | Redis (`go-redis/v9`) |
| Object storage | MinIO (avatars, banners, post media) |
| Realtime | WebSockets (`gofiber/contrib/websocket`) for chat |
| Auth | JWT (`golang-jwt/jwt/v5`) |
| Push notifications | Firebase Cloud Messaging |
| Payments | Xendit (one-time "Virdan Plus" purchases) |
| Email/OTP | SMTP via `gomail.v2` |
| Image processing | `bimg` (libvips bindings) |
| Config | `koanf` (`.env` file + environment variables) |
| Logging | `zap`, exported via OpenTelemetry (OTLP) |
| Observability | OpenTelemetry SDK → otel-collector → Prometheus (metrics), Loki (logs), Tempo (traces), Grafana (dashboards) |
| DB migrations | [Atlas](https://atlasgo.io/), schema-as-code from `db/schema.sql` |
| Testing | `testify` + `testcontainers-go` (real Postgres/Redis containers) |

## Architecture

The app is a **modular monolith**: one deployable binary (`cmd/main.go`), with business logic
split into independent service packages under `services/`, each following the same layered
structure:

```
services/<name>/
├── controller.go   # HTTP handlers (Fiber), request/response mapping
├── service.go      # business logic, orchestration
├── repository.go   # data access (Postgres, Redis, MinIO)
└── model.go        # request/response and domain structs
```

Services: `auth`, `user`, `server`, `post`, `notification`, `chat`, `payment`.

Cross-cutting wiring lives directly in `services/`:
- `services/wire.go` — manual dependency injection; constructs every repository → service →
  controller and assembles the `Registry`.
- `services/route.go` — registers all HTTP routes (and the WebSocket route) against the
  `Registry`, including a `/api/health` endpoint that pings Postgres, Redis, and MinIO.

Shared infrastructure code (not business logic) lives in `shared/`: Fiber setup, config loading
(`koanf`), Postgres/Redis/MinIO/FCM/Xendit clients, JWT auth middleware, CORS, structured
logging, OpenTelemetry providers, the WebSocket hub/broker, image handling, and email templates.

### Request flow

```
HTTP request → Fiber middlewares (CORS, recovery, compress, observability)
             → route (services/route.go)
             → Controller (parses/validates request)
             → Service (business logic, cross-service calls via repos)
             → Repository (Postgres / Redis / MinIO)
```

### Domains / features

- **Auth** — signup (email OTP → password), login, JWT access/refresh token rotation, logout.
- **User** — profile info, password change, email change (OTP-confirmed), account deletion,
  notification preferences.
- **Server** — create/discover/join servers, categories, invite links, member roles
  (kick, assign role, transfer ownership), per-server member profiles, server settings
  (name, avatar, banner, description).
- **Post** — create/update/delete posts (with image/video support), likes, saves, comments
  (threaded), search, per-user/per-server post feeds.
- **Notification** — in-app notification feed, unread counts, mark-as-read, device token
  registration, FCM push delivery.
- **Chat** — direct messages between server members, conversations, WebSocket-based realtime
  delivery via an in-process hub/broker.
- **Payment** — "Virdan Plus" one-time purchases via Xendit checkout + webhook handling, order
  history, plus-status per server.

## Repository Layout

```
cmd/                      # main.go — process entrypoint, wiring of infra clients
services/                 # business logic (see Architecture above)
  wire.go                 # dependency injection
  route.go                # HTTP route registration
shared/                   # cross-cutting infra: config, db clients, middleware, ws hub, etc.
db/
  schema.sql              # source of truth for the DB schema (Atlas)
  migrations/              # generated Atlas migrations
docs/
  flows/en/                # per-endpoint flow docs (source for Swagger generation)
  specs/                   # design docs for specific features
  swagger.yaml             # generated OpenAPI spec
tests/integration/         # testcontainers-backed integration tests, grouped by domain
deployments/
  docker-compose/local/    # local infra (Postgres, Redis, MinIO, observability stack)
  docker-compose/prod/     # production compose stack
  config/                  # otel-collector, Prometheus, Loki, Tempo, Grafana, nginx configs
build/Dockerfile           # app container image
atlas.hcl                  # Atlas migration environment config
Makefile                   # CI, test, migration, and deploy targets
```

## Getting Started

### Prerequisites

- Go 1.26+
- Docker + Docker Compose (for local infra)
- [Atlas CLI](https://atlasgo.io/getting-started) (for migrations, optional if you don't touch
  the schema)
- `libvips` (required by `bimg` for image processing)

### Local setup

1. Copy the environment file and adjust as needed:
   ```
   cp .env.example .env
   ```
2. Start local infrastructure (Postgres, Redis, MinIO, and the observability stack):
   ```
   docker compose -f deployments/docker-compose/local/docker-compose.yml up -d
   ```
3. Apply database migrations:
   ```
   make migrate-apply
   ```
4. Run the API:
   ```
   go run ./cmd
   ```

The API listens on `GO_SERVER` from `.env` (default `:8081`). Health check: `GET /api/health`
(reports Postgres, Redis, and MinIO connectivity).

### Observability

With the local compose stack running:
- Grafana: `http://localhost:3001` (anonymous admin access, dashboards auto-provisioned)
- Prometheus: `http://localhost:9090`
- MinIO console: `http://localhost:9001`

The app exports traces, metrics, and logs via OTLP to the local `otel-collector`, which fans them
out to Tempo, Prometheus, and Loki respectively.

## Testing

Integration tests use `testcontainers-go` to spin up real Postgres/Redis instances, grouped by
domain under `tests/integration/`.

```
make test               # run all integration tests
make test-auth          # auth domain only
make test-user          # user domain only
make test-server        # server domain only
make test-post          # post domain only
make test-profile       # profile domain only
make test-system        # system-level tests (health, etc.)
make test-list          # list all available tests
make test-one name=X    # run a single test by name
make test-coverage      # generate coverage report
```

## Database Migrations

Schema changes are authored in `db/schema.sql` and diffed into versioned migrations with Atlas:

```
make migrate-diff name=add_some_column   # generate a migration from schema.sql changes
make migrate-lint                        # lint the latest migration for unsafe changes
make migrate-apply                       # apply pending migrations locally
```

## CI

`.github/workflows/ci.yml` runs on pushes/PRs to `main`: build, `golangci-lint`, `govulncheck`,
Swagger generation (auto-committed on `main`), and the integration test suite. The same checks
can be run locally:

```
make ci            # build, lint, vuln check, test
make ci-build
make ci-lint
make ci-vuln
make ci-test
```

## API Documentation

- `docs/flows/en/*.md` — one file per endpoint/flow, the source used to generate the OpenAPI spec.
- `docs/swagger.yaml` — generated OpenAPI 3 spec (`make generate-swagger-en`).
- `docs/specs/` — design docs for individual features (e.g. FCM push notifications).

## Deployment

Production deploys run from the VPS via Make targets that pull `main`, apply migrations through
a Dockerized Atlas (no CLI needed on the host), and rebuild the app container:

```
make migrate-deploy   # pull + apply pending migrations
make deploy-app       # pull + rebuild the app container
make deploy-full      # pull + migrate + rebuild (schema ready before new binary)
```

The production stack is defined in `deployments/docker-compose/prod/`, fronted by nginx
(`deployments/config/nginx/`).
