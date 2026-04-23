# PayFlow

> Sistema de transferências financeiras em Go baseado em microsserviços, comunicação assíncrona via RabbitMQ e orquestração por saga.

**[English version](README.en.md)**

---

## Sumário

- [Visão Geral](#visão-geral)
- [Funcionalidades](#funcionalidades)
- [Arquitetura](#arquitetura)
- [Tecnologias](#tecnologias)
- [Pré-requisitos](#pré-requisitos)
- [Executando o Projeto](#executando-o-projeto)
- [Referência da API](#referência-da-api)
- [Estrutura do Projeto](#estrutura-do-projeto)
- [Testes](#testes)
- [Observabilidade](#observabilidade)
- [Variáveis de Ambiente](#variáveis-de-ambiente)
- [Licença](#licença)

---

## Visão Geral

O PayFlow é um sistema financeiro composto por três microsserviços em Go que se comunicam exclusivamente por mensageria (RabbitMQ). O serviço de transferência orquestra uma saga corográfica para coordenar débitos e créditos entre contas, com compensação automática em caso de falha.

O projeto aplica Domain-Driven Design (DDD) com separação rigorosa de camadas em cada serviço, operações financeiras idempotentes, e uma stack completa de observabilidade com tracing distribuído, métricas e dashboards.

---

## Funcionalidades

- **Autenticação** — Registro e login de usuários com JWT (bcrypt para senhas)
- **Gestão de contas** — Criação de contas, consulta de saldo, operações de crédito e débito com validação de regras de negócio
- **Transferências assíncronas** — Transferências entre contas via saga pattern com compensação automática em caso de falha
- **Idempotência** — Operações financeiras protegidas contra processamento duplicado por chave de referência
- **Paginação cursor-based** — Listagem de transferências com cursor para navegação eficiente
- **Documentação OpenAPI + Scalar** — Especificação OpenAPI 3.0 por serviço com interface interativa Scalar em `/docs`, componentes compartilhados mesclados automaticamente
- **Observabilidade completa** — Tracing distribuído (Jaeger), métricas (Prometheus) e dashboards (Grafana)
- **Resiliência** — Circuit breaker no publisher de mensagens, graceful shutdown, health checks
- **API Gateway** — Caddy como proxy reverso unificando os serviços sob uma única porta

---

## Arquitetura

### Topologia dos Serviços

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

### Fluxo da Saga de Transferência

```
1. POST /transfers → Transfer cria registro pendente → publica "account.debit.cmd"
2. Account debita → publica "account.debited" → Transfer processa → publica "account.credit.cmd"
3. Account credita → publica "account.credited" → Transfer marca como concluída
4. Falha em qualquer etapa → compensação (estorno) → Transfer marca como falha
```

### Camadas DDD (por serviço)

```
domain/entities/        Raízes de agregação com regras de negócio e eventos de domínio
domain/repositories/    Interfaces (contratos)
application/commands/   Commands (escrita)
application/queries/    Queries e DTOs de resposta (leitura)
application/services/   Orquestração, depende de repositório + mensageria
interfaces/http/        Handlers Chi, adaptadores HTTP finos
interfaces/messaging/   Consumers RabbitMQ que chamam serviços de aplicação
infrastructure/postgres/Implementações de repositório + migrações SQL embutidas
```

---

## Tecnologias

| Camada | Tecnologia |
|---|---|
| **Linguagem** | Go 1.25 |
| **HTTP Router** | Chi v5 |
| **Banco de dados** | PostgreSQL 16 (3 databases) |
| **Mensageria** | RabbitMQ 3 |
| **Cache** | Redis 7 |
| **Gateway** | Caddy 2 |
| **Autenticação** | JWT (golang-jwt/v5) + bcrypt |
| **Migrações** | golang-migrate/migrate |
| **Tracing** | OpenTelemetry + Jaeger |
| **Métricas** | Prometheus + Grafana |
| **Resiliência** | Circuit breaker (sony/gobreaker) |
| **Documentação** | OpenAPI 3.0 + Scalar |
| **Testes** | testify + gomock (go.uber.org/mock) |
| **IDs** | UUID v7 (time-ordered) |
| **Configuração** | Viper (variáveis de ambiente) |

---

## Pré-requisitos

- [Go](https://go.dev/dl/) 1.25+
- [Docker](https://docs.docker.com/get-docker/) e Docker Compose

---

## Executando o Projeto

### Com Docker Compose (recomendado)

Sobe toda a infraestrutura + serviços:

```bash
docker-compose up -d --build
```

Aguardando os serviços iniciarem, a API estará disponível em `http://localhost:80`.

### Desenvolvimento Local

Infraestrutura apenas (PostgreSQL, RabbitMQ, Redis, Jaeger, Prometheus, Grafana):

```bash
docker-compose up -d
```

Todos os serviços via script de conveniência:

```bash
./run.sh
```

Ou executar serviços individualmente:

```bash
# User Service
DB_NAME=payflow_users SERVICE_PORT=8082 SERVICE_NAME=user-service go run cmd/user-service/main.go

# Account Service
DB_NAME=payflow_accounts SERVICE_PORT=8080 SERVICE_NAME=account-service go run cmd/account-service/main.go

# Transfer Service
DB_NAME=payflow_transfers SERVICE_PORT=8081 SERVICE_NAME=transfer-service go run cmd/transfer-service/main.go
```

---

## Referência da API

### User Service

| Método | Rota | Descrição |
|---|---|---|
| `POST` | `/auth/register` | Registra novo usuário |
| `POST` | `/auth/login` | Autentica e retorna JWT |

### Account Service

| Método | Rota | Descrição |
|---|---|---|
| `POST` | `/accounts` | Cria uma nova conta |
| `GET` | `/accounts/{id}/balance` | Consulta saldo da conta |
| `POST` | `/accounts/{id}/credit` | Credita valor na conta |
| `POST` | `/accounts/{id}/debit` | Debita valor da conta |

### Transfer Service

| Método | Rota | Descrição |
|---|---|---|
| `POST` | `/transfers` | Cria uma transferência |
| `GET` | `/transfers/{id}` | Consulta transferência por ID |
| `GET` | `/transfers` | Lista transferências (paginado) |

Parâmetros de consulta para listagem: `account_id`, `cursor`, `limit`.

### Endpoints Comuns (todos os serviços)

| Método | Rota | Descrição |
|---|---|---|
| `GET` | `/health` | Health check do serviço |
| `GET` | `/metrics` | Métricas Prometheus |
| `GET` | `/docs` | Documentação OpenAPI interativa |
| `GET` | `/openapi.json` | Especificação OpenAPI 3.0 |

### Exemplo de Fluxo Completo

```bash
# 1. Registrar usuário
curl -X POST http://localhost:80/auth/register \
  -H "Content-Type: application/json" \
  -d '{"name":"João","email":"joao@email.com","password":"senha123"}'

# 2. Login
curl -X POST http://localhost:80/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"joao@email.com","password":"senha123"}'
# → Retorna JWT

# 3. Criar contas
curl -X POST http://localhost:80/accounts \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"currency":"BRL"}'

# 4. Realizar transferência
curl -X POST http://localhost:80/transfers \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"from_account_id":"<id>","to_account_id":"<id>","amount":5000}'
# amount em centavos (5000 = R$50,00)
```

Uma collection do Insomnia está disponível em [`insomnia-collection.json`](insomnia-collection.json).

---

## Estrutura do Projeto

```
├── cmd/                          Entrada dos serviços
│   ├── user-service/
│   ├── account-service/
│   └── transfer-service/
├── internal/                     Código privado por serviço (DDD)
│   ├── user/
│   │   ├── domain/               Entidade User + interface UserRepository
│   │   ├── application/          AuthService, commands, queries
│   │   ├── interfaces/http/      AuthHandler + OpenAPI
│   │   └── infrastructure/       Repositório PostgreSQL + migrações
│   ├── account/
│   │   ├── domain/               Entidade Account + interface AccountRepository
│   │   ├── application/          AccountService, commands, queries
│   │   ├── interfaces/           HTTP handler + consumer RabbitMQ
│   │   └── infrastructure/       Repositório PostgreSQL + migrações
│   └── transfer/
│       ├── domain/               Entidade Transfer + interface TransferRepository
│       ├── application/          TransferService (saga), commands, queries
│       ├── interfaces/           HTTP handler + consumer RabbitMQ
│       └── infrastructure/       Repositório PostgreSQL + migrações
├── pkg/                          Pacotes compartilhados
│   ├── app/                      Builder fluente para bootstrap dos serviços
│   ├── auth/                     Utilitários JWT
│   ├── config/                   Configuração com Viper
│   ├── errors/                   Tipos de erro customizados
│   ├── events/                   Contratos de eventos compartilhados
│   ├── health/                   Health checks
│   ├── httputil/                 Utilitários HTTP (respostas, erros)
│   ├── messaging/                Pub/sub RabbitMQ + circuit breaker
│   ├── middleware/               Middleware Chi (auth, logging, recovery, OTel)
│   ├── migrate/                  Utilitário de migração
│   ├── openapi/                  Serviço de documentação OpenAPI
│   ├── pagination/               Paginação cursor-based
│   ├── telemetry/                OpenTelemetry tracing + métricas
│   └── validation/               Validação de requests
├── docker/                       Configuração de infraestrutura
│   ├── caddy/                    Caddyfile (gateway)
│   ├── grafana/                  Provisionamento de dashboards + datasources
│   ├── postgres/                 Scripts de inicialização do banco
│   └── prometheus/               Configuração de scrape
├── docker-compose.yml            Stack completa
├── run.sh                        Script de execução local
└── insomnia-collection.json      Collection de API para testes
```

---

## Testes

```bash
# Todos os testes
go test ./...

# Por serviço
go test ./internal/account/...
go test ./internal/transfer/...
go test ./internal/user/...
go test ./pkg/...

# Teste específico
go test ./internal/transfer/domain/entities -run TestTransfer_IsPending

# Com verbose
go test -v ./internal/account/application/services/...

# Regenerar mocks (após alterar interfaces)
go generate ./...
```

Os testes utilizam **testify** para asserções e **gomock** para mocks gerados automaticamente a partir de diretivas `//go:generate mockgen`.

---

## Observabilidade

| Ferramenta | Porta | Credenciais |
|---|---|---|
| **Jaeger UI** | [http://localhost:16686](http://localhost:16686) | — |
| **Prometheus** | [http://localhost:9090](http://localhost:9090) | — |
| **Grafana** | [http://localhost:3000](http://localhost:3000) | `admin` / `payflow123` |
| **RabbitMQ Management** | [http://localhost:15672](http://localhost:15672) | `payflow` / `payflow123` |

Cada serviço expõe `/metrics` para o Prometheus e envia traces para o Jaeger via OTLP. O Grafana já vem provisionado com datasource Prometheus e dashboard de overview do PayFlow.

---

## Variáveis de Ambiente

Todas as configurações são feitas via variáveis de ambiente com valores padrão para desenvolvimento local:

| Variável | Descrição | Padrão |
|---|---|---|
| `SERVICE_PORT` | Porta do serviço | `8080` |
| `SERVICE_NAME` | Nome do serviço | — |
| `DB_HOST` | Host do PostgreSQL | `localhost` |
| `DB_PORT` | Porta do PostgreSQL | `5432` |
| `DB_USER` | Usuário do PostgreSQL | `payflow` |
| `DB_PASSWORD` | Senha do PostgreSQL | `payflow123` |
| `DB_NAME` | Nome do database | — |
| `RABBITMQ_URL` | URL do RabbitMQ | `amqp://payflow:payflow123@localhost:5672/` |
| `JWT_SECRET` | Chave de assinatura JWT | — |
| `JAEGER_ENDPOINT` | Endpoint do Jaeger | `localhost:4317` |

---

## Licença

Este projeto está licenciado sob a licença MIT. Veja o arquivo [LICENSE](LICENSE) para mais detalhes.

---

<p align="center">
  Desenvolvido com Go, RabbitMQ e arquitetura de microsserviços.
</p>
