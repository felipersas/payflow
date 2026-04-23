# PayFlow

> A Go-based financial transfer system built with microservices, asynchronous communication via RabbitMQ, and saga orchestration.

---

## Table of Contents

- [Overview](#overview)
- [Features](#features)
- [Architecture](#architecture)
- [Tech Stack](#tech-stack)
- [Prerequisites](#prerequisites)
- [Getting Started](#getting-started)
- [API Reference](#api-reference)
- [Project Structure](#project-structure)
- [Testing](#testing)
- [Observability](#observability)
- [Environment Variables](#environment-variables)
- [License](#license)

---

## Overview

PayFlow is a financial system composed of three Go microservices that communicate exclusively through RabbitMQ messaging. The transfer service orchestrates a choreographic saga to coordinate debits and credits between accounts, with automatic compensation on failure.

The project applies Domain-Driven Design (DDD) with strict layer separation per service, idempotent financial operations, and a full observability stack with distributed tracing, metrics, and dashboards.

---

## Features

- **Authentication** — User registration and login with JWT (bcrypt for passwords)
- **Account management** — Account creation, balance queries, credit and debit operations with business rule validation
- **Asynchronous transfers** — Cross-account transfers via saga pattern with automatic compensation on failure
- **Idempotency** — Financial operations protected against duplicate processing by reference key
- **Cursor-based pagination** — Transfer listing with cursor for efficient navigation
- **OpenAPI + Scalar documentation** — OpenAPI 3.0 spec per service with interactive Scalar UI at `/docs`, shared components auto-merged
- **Full observability** — Distributed tracing (Jaeger), metrics (Prometheus), and dashboards (Grafana)
- **Resilience** — Circuit breaker on message publisher, graceful shutdown, health checks
- **API Gateway** — Caddy as reverse proxy unifying services under a single port

---

## Architecture

### Service Topology

```
┌──────────────────────────────────────────────────────────────────────┐
│                          Caddy Gateway (:80)                         │
│                 /auth/*  /accounts/*  /transfers/*                   │
└──────────┬──────────────┬──────────────────┬────────────────────────┘
           │              │                  │
           ▼              ▼                  ▼
┌──────────────┐  ┌───────────────┐  ┌────────────────┐
│  User Service │  │Account Service│  │Transfer Service│
│   (:8082)     │  │   (:8080)     │  │   (:8081)      │
│               │  │               │  │                │
│ payflow_users │  │payflow_accounts│ │payflow_transfers│
└──────────────┘  └───────┬───────┘  └───────┬────────┘
                          │                  │
                          │    RabbitMQ      │
                          └──────────────────┘
                          account.debit.cmd
                          account.credit.cmd
                          account.compensate.cmd
```

### Transfer Saga Flow

```
1. POST /transfers → Transfer creates pending record → publishes "account.debit.cmd"
2. Account debits → publishes "account.debited" → Transfer processes → publishes "account.credit.cmd"
3. Account credits → publishes "account.credited" → Transfer marks as completed
4. Failure at any step → compensation (reversal) → Transfer marks as failed
```

### DDD Layers (per service)

```
domain/entities/        Aggregate roots with business rules and domain events
domain/repositories/    Interfaces (contracts)
application/commands/   Commands (write side)
application/queries/    Query result DTOs (read side)
application/services/   Orchestration, depends on repository + messaging
interfaces/http/        Chi handlers, thin HTTP adapters
interfaces/messaging/   RabbitMQ consumers calling application services
infrastructure/postgres/Repository implementations + embedded SQL migrations
```

---

## Tech Stack

| Layer | Technology |
|---|---|
| **Language** | Go 1.25 |
| **HTTP Router** | Chi v5 |
| **Database** | PostgreSQL 16 (3 databases) |
| **Messaging** | RabbitMQ 3 |
| **Cache** | Redis 7 |
| **Gateway** | Caddy 2 |
| **Authentication** | JWT (golang-jwt/v5) + bcrypt |
| **Migrations** | golang-migrate/migrate |
| **Tracing** | OpenTelemetry + Jaeger |
| **Metrics** | Prometheus + Grafana |
| **Resilience** | Circuit breaker (sony/gobreaker) |
| **Documentation** | OpenAPI 3.0 + Scalar |
| **Testing** | testify + gomock (go.uber.org/mock) |
| **IDs** | UUID v7 (time-ordered) |
| **Configuration** | Viper (environment variables) |

---

## Prerequisites

- [Go](https://go.dev/dl/) 1.25+
- [Docker](https://docs.docker.com/get-docker/) and Docker Compose

---

## Getting Started

### With Docker Compose (recommended)

Starts the entire infrastructure + services:

```bash
docker-compose up -d --build
```

Once services are up, the API is available at `http://localhost:80`.

### Local Development

Infrastructure only (PostgreSQL, RabbitMQ, Redis, Jaeger, Prometheus, Grafana):

```bash
docker-compose up -d
```

All services via convenience script:

```bash
./run.sh
```

Or run services individually:

```bash
# User Service
DB_NAME=payflow_users SERVICE_PORT=8082 SERVICE_NAME=user-service go run cmd/user-service/main.go

# Account Service
DB_NAME=payflow_accounts SERVICE_PORT=8080 SERVICE_NAME=account-service go run cmd/account-service/main.go

# Transfer Service
DB_NAME=payflow_transfers SERVICE_PORT=8081 SERVICE_NAME=transfer-service go run cmd/transfer-service/main.go
```

---

## API Reference

### User Service

| Method | Route | Description |
|---|---|---|
| `POST` | `/auth/register` | Register a new user |
| `POST` | `/auth/login` | Authenticate and return JWT |

### Account Service

| Method | Route | Description |
|---|---|---|
| `POST` | `/accounts` | Create a new account |
| `GET` | `/accounts/{id}/balance` | Get account balance |
| `POST` | `/accounts/{id}/credit` | Credit amount to account |
| `POST` | `/accounts/{id}/debit` | Debit amount from account |

### Transfer Service

| Method | Route | Description |
|---|---|---|
| `POST` | `/transfers` | Create a transfer |
| `GET` | `/transfers/{id}` | Get transfer by ID |
| `GET` | `/transfers` | List transfers (paginated) |

Query parameters for listing: `account_id`, `cursor`, `limit`.

### Common Endpoints (all services)

| Method | Route | Description |
|---|---|---|
| `GET` | `/health` | Service health check |
| `GET` | `/metrics` | Prometheus metrics |
| `GET` | `/docs` | Interactive OpenAPI documentation |
| `GET` | `/openapi.json` | OpenAPI 3.0 specification |

### Example Flow

```bash
# 1. Register user
curl -X POST http://localhost:80/auth/register \
  -H "Content-Type: application/json" \
  -d '{"name":"John","email":"john@email.com","password":"password123"}'

# 2. Login
curl -X POST http://localhost:80/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"john@email.com","password":"password123"}'
# → Returns JWT

# 3. Create accounts
curl -X POST http://localhost:80/accounts \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"currency":"BRL"}'

# 4. Create transfer
curl -X POST http://localhost:80/transfers \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"from_account_id":"<id>","to_account_id":"<id>","amount":5000}'
# amount in cents (5000 = R$50.00)
```

An Insomnia API collection is available at [`insomnia-collection.json`](insomnia-collection.json).

---

## Project Structure

```
├── cmd/                          Service entry points
│   ├── user-service/
│   ├── account-service/
│   └── transfer-service/
├── internal/                     Private code per service (DDD)
│   ├── user/
│   │   ├── domain/               User entity + UserRepository interface
│   │   ├── application/          AuthService, commands, queries
│   │   ├── interfaces/http/      AuthHandler + OpenAPI
│   │   └── infrastructure/       PostgreSQL repository + migrations
│   ├── account/
│   │   ├── domain/               Account entity + AccountRepository interface
│   │   ├── application/          AccountService, commands, queries
│   │   ├── interfaces/           HTTP handler + RabbitMQ consumer
│   │   └── infrastructure/       PostgreSQL repository + migrations
│   └── transfer/
│       ├── domain/               Transfer entity + TransferRepository interface
│       ├── application/          TransferService (saga), commands, queries
│       ├── interfaces/           HTTP handler + RabbitMQ consumer
│       └── infrastructure/       PostgreSQL repository + migrations
├── pkg/                          Shared packages
│   ├── app/                      Fluent builder for service bootstrap
│   ├── auth/                     JWT utilities
│   ├── config/                   Viper configuration
│   ├── errors/                   Custom error types
│   ├── events/                   Shared event contracts
│   ├── health/                   Health checks
│   ├── httputil/                 HTTP utilities (responses, errors)
│   ├── messaging/                RabbitMQ pub/sub + circuit breaker
│   ├── middleware/               Chi middleware (auth, logging, recovery, OTel)
│   ├── migrate/                  Migration utility
│   ├── openapi/                  OpenAPI documentation serving
│   ├── pagination/               Cursor-based pagination
│   ├── telemetry/                OpenTelemetry tracing + metrics
│   └── validation/               Request validation
├── docker/                       Infrastructure configuration
│   ├── caddy/                    Caddyfile (gateway)
│   ├── grafana/                  Dashboard + datasource provisioning
│   ├── postgres/                 Database initialization scripts
│   └── prometheus/               Scrape configuration
├── docker-compose.yml            Full stack
├── run.sh                        Local development runner
└── insomnia-collection.json      API collection for testing
```

---

## Testing

```bash
# All tests
go test ./...

# Per service
go test ./internal/account/...
go test ./internal/transfer/...
go test ./internal/user/...
go test ./pkg/...

# Specific test
go test ./internal/transfer/domain/entities -run TestTransfer_IsPending

# Verbose output
go test -v ./internal/account/application/services/...

# Regenerate mocks (after changing interfaces)
go generate ./...
```

Tests use **testify** for assertions and **gomock** for mocks auto-generated from `//go:generate mockgen` directives.

---

## Observability

| Tool | Port | Credentials |
|---|---|---|
| **Jaeger UI** | [http://localhost:16686](http://localhost:16686) | — |
| **Prometheus** | [http://localhost:9090](http://localhost:9090) | — |
| **Grafana** | [http://localhost:3000](http://localhost:3000) | `admin` / `payflow123` |
| **RabbitMQ Management** | [http://localhost:15672](http://localhost:15672) | `payflow` / `payflow123` |

Each service exposes `/metrics` for Prometheus and sends traces to Jaeger via OTLP. Grafana comes pre-provisioned with a Prometheus datasource and a PayFlow overview dashboard.

---

## Environment Variables

All configuration is done via environment variables with sensible defaults for local development:

| Variable | Description | Default |
|---|---|---|
| `SERVICE_PORT` | Service port | `8080` |
| `SERVICE_NAME` | Service name | — |
| `DB_HOST` | PostgreSQL host | `localhost` |
| `DB_PORT` | PostgreSQL port | `5432` |
| `DB_USER` | PostgreSQL user | `payflow` |
| `DB_PASSWORD` | PostgreSQL password | `payflow123` |
| `DB_NAME` | Database name | — |
| `RABBITMQ_URL` | RabbitMQ URL | `amqp://payflow:payflow123@localhost:5672/` |
| `JWT_SECRET` | JWT signing key | — |
| `JAEGER_ENDPOINT` | Jaeger endpoint | `localhost:4317` |

---

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.

---

<p align="center">
  Built with Go, RabbitMQ, and microservices architecture.
</p>
