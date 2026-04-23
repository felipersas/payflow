# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

PayFlow — a Go microservices system for financial transfers between accounts. Three services communicate asynchronously via RabbitMQ using a saga orchestration pattern.

## Build & Run Commands

```bash
# Run all tests
go test ./...

# Run tests for a specific package
go test ./internal/account/...
go test ./internal/transfer/...
go test ./internal/user/...
go test ./pkg/...

# Run a single test
go test ./internal/transfer/domain/entities -run TestTransfer_IsPending

# Run with verbose output
go test -v ./internal/account/application/services/...

# Start infrastructure (Postgres, RabbitMQ, Redis, Jaeger, Prometheus)
docker-compose up -d

# Run all services locally (starts infra + account + transfer services)
./run.sh

# Run a single service
DB_NAME=payflow_accounts SERVICE_PORT=8080 SERVICE_NAME=account-service go run cmd/account-service/main.go

# Regenerate mocks (after changing interfaces with //go:generate directives)
go generate ./...
```

## Architecture

### Service Topology

```
User (8082) ── registers/logins, issues JWT
Account (8080) ── manages balances, processes debit/credit commands
Transfer (8081) ── orchestrates saga, routes commands to Account via RabbitMQ
Caddy Gateway (80) ── reverse proxy to Account + Transfer services
```

Each service owns its own PostgreSQL database (`payflow_users`, `payflow_accounts`, `payflow_transfers`). Services communicate exclusively through RabbitMQ messaging — no direct service-to-service HTTP calls.

### App Builder Pattern (`pkg/app/app.go`)

All services use a fluent builder to bootstrap infrastructure. Each `main.go` only specifies what's unique:

```go
app.New("service-name", cfg).
    WithTracer().
    WithDatabase(postgres.Migrations, "migrations").
    WithRabbitMQ().
    RegisterRoutes(func(r chi.Router, d *app.Deps) { /* wire handlers */ }).
    RegisterConsumers(func(d *app.Deps) error { /* start consumers */ }).
    MustRun()
```

`app.Deps` provides: `Pool` (pgxpool), `Publisher`, `Consumer`, `Logger`, `JWTSecret`, `Ctx`. Global middleware (correlation ID, logging, recovery, OTel, metrics) is applied automatically.

### Internal Package Structure (per service)

Each service under `internal/{service}/` follows DDD layering:

```
domain/entities/     — aggregate roots with business rules and domain events
domain/repositories/ — interfaces (consumers of these use //go:generate mockgen)
application/commands/ — write-side command structs
application/queries/  — read-side query structs and result DTOs
application/services/ — orchestration layer, depends on repository + messaging interfaces
interfaces/http/      — chi handlers, thin HTTP adapters
interfaces/messaging/ — RabbitMQ consumers that call application services
infrastructure/postgres/ — repository implementations + embedded SQL migrations
```

### Saga Orchestration (Transfer Flow)

Transfer service orchestrates a choreography saga via RabbitMQ:

1. `POST /transfers` → creates pending transfer → publishes `account.debit.cmd`
2. Account service debits → publishes `account.debited` → Transfer handles `HandleDebitConfirmed` → publishes `account.credit.cmd`
3. Account service credits → publishes `account.credited` → Transfer handles `HandleCreditConfirmed` → marks completed, publishes `transfer.completed`
4. On failure at any step: compensation (reversal debit) is sent, transfer marked failed, `transfer.failed` published

### Auth Flow

- User service: `POST /auth/register` and `POST /auth/login` (public, no auth middleware)
- Auth middleware (`pkg/middleware/auth.go`) validates JWT Bearer tokens and injects `user_id` into context via `middleware.GetUserID(ctx)`
- Account and Transfer routes are protected with `middleware.Auth(jwtSecret)`

### Key Conventions

- Monetary amounts stored as `int64` cents (never float)
- UUID v7 for all entity IDs (time-ordered)
- Domain entities return domain events from mutation methods (e.g., `account.Debit()` returns `*events.AccountDebited`)
- Event types and shared event contracts live in `pkg/events/`
- Configuration via environment variables using viper (`pkg/config/config.go`), defaults work for local development
- SQL migrations embedded per service via `embed.FS` in `infrastructure/postgres/embed.go`
- Circuit breaker wrapping around message publisher (`pkg/messaging/resilient_publisher.go`)

### Testing

Tests use **testify** (`assert`/`require`) for assertions and **gomock** (`go.uber.org/mock`) for mocks. Mocks are generated from `//go:generate mockgen` directives on repository and messaging interfaces. Test files live alongside the code they test (`*_test.go`).

### Infrastructure Defaults (local dev)

| Service | Port | DB |
|---|---|---|
| user-service | 8082 | payflow_users |
| account-service | 8080 | payflow_accounts |
| transfer-service | 8081 | payflow_transfers |
| PostgreSQL | 5432 | payflow / payflow123 |
| RabbitMQ | 5672 / 15672 (mgmt) | payflow / payflow123 |
| Jaeger | 16686 (UI) / 4317 (OTLP) | — |
| Prometheus | 9090 | — |

API collection available in `insomnia-collection.json`.
