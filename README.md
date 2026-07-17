# Virdan Project

Virdan is a community/server-based social backend, conceptually similar to Discord: users join
topic-based "servers," post content, comment, react, chat via DMs, and receive push notifications.
It is a single Go binary composed of modular services behind a Fiber HTTP API, backed by
PostgreSQL, Redis, and MinIO, with a full OpenTelemetry observability stack.

## Tech Stack

**Language**

![Go](https://img.shields.io/badge/Go-00ADD8?style=for-the-badge&logo=go&logoColor=white)

**Data & storage**

![PostgreSQL](https://img.shields.io/badge/PostgreSQL-336791?style=for-the-badge&logo=postgresql&logoColor=white)
![Redis](https://img.shields.io/badge/Redis-DC382D?style=for-the-badge&logo=redis&logoColor=white)
![MinIO](https://img.shields.io/badge/MinIO-C72C48?style=for-the-badge&logo=minio&logoColor=white)

**Realtime, auth & payments**

![WebSocket](https://img.shields.io/badge/WebSocket-000000?style=for-the-badge&logo=websocket&logoColor=white)
![JWT](https://img.shields.io/badge/JWT-000000?style=for-the-badge&logo=jsonwebtokens&logoColor=white)
![Firebase](https://img.shields.io/badge/Firebase_Cloud_Messaging-FFCA28?style=for-the-badge&logo=firebase&logoColor=black)
![Xendit](https://img.shields.io/badge/Xendit-5B4DCA?style=for-the-badge&logo=xendit&logoColor=white)

**Observability**

![OpenTelemetry](https://img.shields.io/badge/OpenTelemetry-005B9C?style=for-the-badge&logo=opentelemetry&logoColor=white)
![Prometheus](https://img.shields.io/badge/Prometheus-E6522C?style=for-the-badge&logo=prometheus&logoColor=white)
![Grafana](https://img.shields.io/badge/Grafana-F05A28?style=for-the-badge&logo=grafana&logoColor=white)
<img src="https://raw.githubusercontent.com/homarr-labs/dashboard-icons/main/svg/loki.svg" alt="Loki" title="Grafana Loki" height="28" />
<img src="https://raw.githubusercontent.com/homarr-labs/dashboard-icons/main/svg/tempo.svg" alt="Tempo" title="Grafana Tempo" height="28" />

**API docs & infra**

![Swagger](https://img.shields.io/badge/Swagger_OpenAPI-85EA2D?style=for-the-badge&logo=swagger&logoColor=black)
![Docker](https://img.shields.io/badge/Docker-2496ED?style=for-the-badge&logo=docker&logoColor=white)
![NGINX](https://img.shields.io/badge/NGINX-009639?style=for-the-badge&logo=nginx&logoColor=white)
![GitHub Actions](https://img.shields.io/badge/GitHub_Actions-2088FF?style=for-the-badge&logo=githubactions&logoColor=white)
![Atlas](https://img.shields.io/badge/Atlas_Migrations-555555?style=for-the-badge)

**Testing**

<img src="https://raw.githubusercontent.com/testcontainers/testcontainers-java/main/docs/logo.svg" alt="Testcontainers" title="Testcontainers" height="28" />

> Loki, Tempo, and Testcontainers icons are from [dashboardicons.com](https://dashboardicons.com/icons/)
> and the official Testcontainers repo — neither has an entry in [simple-icons](https://simpleicons.org/).
> Plain gray badges mark libraries with no official icon anywhere.

## Architecture

Virdan is built as a **modular monolith**: a single deployable Go binary (`cmd/main.go`) whose
business logic is split into independent, domain-based service packages under `services/`, each
following the same layered structure. This keeps clear boundaries between domains without the
operational overhead of running separate microservices.

```
services/<name>/
├── controller.go   # HTTP handlers (Fiber), request/response mapping
├── service.go      # business logic, orchestration
├── repository.go   # data access (Postgres, Redis, MinIO)
└── model.go        # request/response and domain structs
```

Services: `auth`, `user`, `server`, `post`, `notification`, `chat`, `payment`.

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

## Request Flow

```
HTTP request → Fiber middlewares (CORS, recovery, compress, observability)
             → route (services/route.go)
             → Controller (parses/validates request)
             → Service (business logic, cross-service calls via repos)
             → Repository (Postgres / Redis / MinIO)
```

## Domain Features

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
