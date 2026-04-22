# Guia Completo: Microservicos em Go para Sistema Financeiro

> Arquitetura de microservicos com mensageria (RabbitMQ), eventos, e padroes de sistema financeiro

---

## Sumario

1. [Projeto Sugerido: PayFlow](#1-projeto-sugerido-payflow)
2. [Arquitetura de Microservicos](#2-arquitetura-de-microservicos)
3. [Stack Tecnologica](#3-stack-tecnologica)
4. [Mensageria com RabbitMQ](#4-mensageria-com-rabbitmq)
5. [Padroes de Comunicacao](#5-padroes-de-comunicacao)
6. [Domain Events no Financeiro](#6-domain-events-no-financeiro)
7. [Estrutura do Projeto](#7-estrutura-do-projeto)
8. [Cada Microservico em Detalhe](#8-cada-microservico-em-detalhe)
9. [Transacoes Distribuidas](#9-transacoes-distribuidas)
10. [Seguranca no Financeiro](#10-seguranca-no-financeiro)
11. [Observabilidade](#11-observabilidade)
12. [Roadmap de Implementacao](#12-roadmap-de-implementacao)
13. [Exemplos de Codigo](#13-exemplos-de-codigo)

---

## 1. Projeto Sugerido: PayFlow

### O que e

Um **sistema de pagamentos digital** tipo PIX/PayPal com microservicos. Cobrimos transferencias, contas, notificacoes e antifraude - tudo comunicando via RabbitMQ.

### Funcionalidades

- Criacao e gerenciamento de contas (wallets)
- Transferencias P2P (pessoa para pessoa)
- Deposit e saque
- Analise antifraude em tempo real
- Notificacoes (email, SMS, push)
- Extrato e historico de transacoes
- Conciliacao e auditoria

### Por que esse projeto?

| Conceito | Onde aparece no PayFlow |
|----------|------------------------|
| Microservicos | Cada dominio e um servico independente |
| Mensageria (RabbitMQ) | Comunicacao assincrona entre servicos |
| Eventos de dominio | `TransferCreated`, `FraudDetected`, `AccountCredited` |
| Saga Pattern | Coordenacao de transferencia entre contas |
| Circuit Breaker | Se o antifraude cai, o que fazer? |
| Idempotencia | Nao debitar duas vezes a mesma transferencia |
| CQRS | Leitura de extrato separada da escrita de transacao |
| DDD | Entidades com regras financeiras (saldo nunca negativo) |
| gRPC | Comunicacao interna entre servicos |
| REST | API publica para clientes |
| PostgreSQL | Dados transacionais (ACID) |
| Redis | Cache de saldos, rate limiting |
| Docker/K8s | Deploy e orquestracao |

---

## 2. Arquitetura de Microservicos

### Diagrama Geral

```
                         ┌─────────────────┐
                         │   API Gateway   │
                         │   (chi/echo)    │
                         │   :8080         │
                         └────────┬────────┘
                                  │
                    ┌─────────────┼─────────────┐
                    │             │             │
                    ▼             ▼             ▼
            ┌──────────┐  ┌──────────┐  ┌──────────┐
            │ Account  │  │Transfer  │  │  Auth    │
            │ Service  │  │ Service  │  │ Service  │
            │ :8081    │  │ :8082    │  │ :8083    │
            └────┬─────┘  └────┬─────┘  └──────────┘
                 │             │
         ┌───────┴─────────────┴────────┐
         │         RabbitMQ             │
         │     (Exchange / Queues)      │
         └───────┬─────┬─────┬─────────┘
                 │     │     │
                 ▼     ▼     ▼
         ┌────────┐ ┌────────┐ ┌──────────┐
         │Fraud   │ │Notif   │ │Ledger    │
         │Service │ │Service │ │Service   │
         │ :8084  │ │ :8085  │ │ :8086    │
         └────────┘ └────────┘ └──────────┘
```

### Princpios

1. **Cada servico tem seu proprio banco** (Database per Service)
2. **Comunicacao assincrona via mensageria** para tudo que nao precisa de resposta imediata
3. **Comunicacao sincrona (gRPC/HTTP)** apenas quando necessario
4. **Eventos de dominio** para desacoplar servicos
5. **Cada servico e autonomo** - deploy, escala e falha independentemente

### API Gateway

Ponto de entrada unico. Responsavel por:
- Roteamento para o servico certo
- Autenticacao (valida JWT)
- Rate limiting
- CORS
- Compressao

---

## 3. Stack Tecnologica

### Core

| Tecnologia | Funcao | Porque |
|-----------|--------|--------|
| **Go 1.22+** | Linguagem | Performance, concorrencia, tipagem forte |
| **PostgreSQL** | Banco principal | ACID, essencial para financeiro |
| **RabbitMQ** | Mensageria | AMQP maduro, confirmações, dead letters |
| **Redis** | Cache / Locks | Saldo em cache, distributed locks |

### Comunicacao

| Tecnologia | Uso |
|-----------|-----|
| **REST (chi)** | API publica para clientes web/mobile |
| **gRPC** | Comunicacao interna entre servicos |
| **Protocol Buffers** | Contratos gRPC |

### Infraestrutura

| Tecnologia | Uso |
|-----------|-----|
| **Docker** | Container de cada servico |
| **Docker Compose** | Orquestracao local (dev) |
| **Kubernetes** | Orquestracao de producao |
| **Prometheus + Grafana** | Metricas |
| **Jaeger / Zipkin** | Distributed tracing |
| **slog (stdlib)** | Logging estruturado |

### Ferramentas Go

| Biblioteca | Funcao |
|-----------|--------|
| `github.com/rabbitmq/amqp091-go` | Client RabbitMQ oficial |
| `github.com/go-chi/chi/v5` | Router HTTP |
| `github.com/google/uuid` | IDs unicos |
| `github.com/jackc/pgx/v5` | Driver PostgreSQL |
| `github.com/sqlc-dev/sqlc` | SQL type-safe |
| `github.com/redis/go-redis/v9` | Client Redis |
| `github.com/golang-jwt/jwt/v5` | JWT tokens |
| `github.com/google.golang.org/grpc` | gRPC |
| `github.com/streadway/amqp` | Alt: client AMQP |
| `github.com/stretchr/testify` | Testes |
| `github.com/spf13/viper` | Configuracao |
| `log/slog` | Logging estruturado (stdlib) |

---

## 4. Mensageria com RabbitMQ

### Conceitos Fundamentais

```
┌──────────┐     ┌───────────────┐     ┌───────────────┐     ┌──────────┐
│ Producer  │────▶│   Exchange    │────▶│     Queue     │────▶│Consumer  │
│ (Service) │     │ (Roteador)    │     │  (Fila)       │     │(Service) │
└──────────┘     └───────────────┘     └───────────────┘     └──────────┘
                   routing_key            mensagens
                   bindings               persistentes
```

**Exchange**: Recebe mensagens e roteia para filas
**Queue**: Armazena mensagens ate serem consumidas
**Binding**: Regra que liga exchange a queue
**Routing Key**: Chave que determina para qual queue vai

### Tipos de Exchange

| Tipo | Comportamento | Uso no PayFlow |
|------|--------------|----------------|
| **direct** | Roteia por routing key exata | `transfer.created` -> queue especifica |
| **topic** | Roteia por padrao (wildcard) | `transfer.*` -> todas as operacoes de transfer |
| **fanout** | Broadcast para todas as queues | Notificar todos os servicos |
| **headers** | Roteia por headers | Raramente usado |

### Exchanges do PayFlow

```
Exchange: payflow.events (topic)
├── binding: transfer.created   → queue: fraud-service.check
├── binding: transfer.created   → queue: ledger-service.record
├── binding: transfer.completed → queue: notification-service.notify
├── binding: fraud.detected     → queue: transfer-service.block
└── binding: account.credited   → queue: notification-service.notify
```

### Publisher em Go

```go
package messaging

import (
    "encoding/json"
    "log/slog"
    amqp "github.com/rabbitmq/amqp091-go"
)

type EventPublisher struct {
    channel *amqp.Channel
    logger  *slog.Logger
}

func NewEventPublisher(conn *amqp.Connection, logger *slog.Logger) (*EventPublisher, error) {
    ch, err := conn.Channel()
    if err != nil {
        return nil, err
    }

    // Declara exchange do tipo topic
    err = ch.ExchangeDeclare(
        "payflow.events", // nome
        "topic",          // tipo
        true,             // durable (sobrevive restart)
        false,            // auto-deleted
        false,            // internal
        false,            // no-wait
        nil,              // args
    )
    if err != nil {
        return nil, err
    }

    return &EventPublisher{channel: ch, logger: logger}, nil
}

func (p *EventPublisher) Publish(routingKey string, event interface{}) error {
    body, err := json.Marshal(event)
    if err != nil {
        return err
    }

    err = p.channel.PublishWithContext(
        context.Background(),
        "payflow.events", // exchange
        routingKey,       // routing key
        false,            // mandatory
        false,            // immediate
        amqp.Publishing{
            ContentType:  "application/json",
            DeliveryMode: amqp.Persistent, // Mensagem persiste no disco
            MessageId:    uuid.New().String(),
            Timestamp:    time.Now(),
            Body:         body,
        },
    )
    if err != nil {
        return err
    }

    p.logger.Info("event published",
        "routing_key", routingKey,
        "message_id", uuid.New().String(),
    )
    return nil
}
```

### Consumer em Go

```go
package messaging

import (
    "encoding/json"
    "log/slog"
    amqp "github.com/rabbitmq/amqp091-go"
)

type EventHandler func(body []byte) error

type EventConsumer struct {
    channel *amqp.Channel
    logger  *slog.Logger
}

func NewEventConsumer(conn *amqp.Connection, logger *slog.Logger) (*EventConsumer, error) {
    ch, err := conn.Channel()
    if err != nil {
        return nil, err
    }

    // QoS: prefetch count limita mensagens nao-ack por consumer
    err = ch.Qos(
        10,    // prefetch count (processa 10 por vez)
        0,     // prefetch size
        false, // global
    )
    if err != nil {
        return nil, err
    }

    return &EventConsumer{channel: ch, logger: logger}, nil
}

func (c *EventConsumer) Subscribe(queueName, routingKey string, handler EventHandler) error {
    // Declara queue
    q, err := c.channel.QueueDeclare(
        queueName, // nome
        true,      // durable
        false,     // auto-delete
        false,     // exclusive
        false,     // no-wait
        amqp.Table{
            "x-dead-letter-exchange":    "payflow.dlx",
            "x-dead-letter-routing-key": queueName + ".dead",
        },
    )
    if err != nil {
        return err
    }

    // Bind queue a exchange
    err = c.channel.QueueBind(
        q.Name,           // queue name
        routingKey,       // routing key
        "payflow.events", // exchange
        false,            // no-wait
        nil,              // args
    )
    if err != nil {
        return err
    }

    // Comeca a consumir
    msgs, err := c.channel.Consume(
        q.Name,   // queue
        "",       // consumer tag (auto)
        false,    // auto-ack (FALSO! ack manual para garantir processamento)
        false,    // exclusive
        false,    // no-local
        false,    // no-wait
        nil,      // args
    )
    if err != nil {
        return err
    }

    // Processa mensagens em goroutine
    go func() {
        for msg := range msgs {
            c.processMessage(msg, handler)
        }
    }()

    c.logger.Info("consumer subscribed", "queue", queueName, "routing_key", routingKey)
    return nil
}

func (c *EventConsumer) processMessage(msg amqp.Delivery, handler EventHandler) {
    err := handler(msg.Body)
    if err != nil {
        c.logger.Error("message processing failed",
            "error", err,
            "message_id", msg.MessageId,
            "redelivery", msg.Redelivered,
        )

        if msg.Redelivered {
            // Ja tentou antes, manda pra dead letter queue
            msg.Nack(false, false) // requeue=false -> vai pra DLQ
        } else {
            // Primeira tentativa, tenta de novo
            msg.Nack(false, true) // requeue=true -> volta pra fila
        }
        return
    }

    // Sucesso: ack a mensagem
    msg.Ack(false)
    c.logger.Info("message processed", "message_id", msg.MessageId)
}
```

### Dead Letter Queue (DLQ)

Mensagens que falham repetidamente vao para uma DLQ para investigacao manual:

```go
// Declarar exchange e queue para dead letters
func setupDeadLetter(ch *amqp.Channel) error {
    // Exchange DLX
    ch.ExchangeDeclare("payflow.dlx", "topic", true, false, false, false, nil)

    // Queue DLQ
    q, _ := ch.QueueDeclare("payflow.dead-letters", true, false, false, false, nil)
    ch.QueueBind(q.Name, "#", "payflow.dlx", false, nil) // Tudo vai pra DLQ
    return nil
}
```

---

## 5. Padroes de Comunicacao

### Sincrona vs Assincrona

```
┌─────────────────────────────────────────────────────┐
│              QUANDO USAR CADA UMA                     │
├──────────────────┬──────────────────────────────────┤
│   Sincrona       │   Assincrona                     │
│   (HTTP/gRPC)    │   (RabbitMQ)                     │
├──────────────────┼──────────────────────────────────┤
│ Cliente precisa  │ Processamento pode ser           │
│ da resposta      │ posterior (fraud check,          │
│ imediata         │ notificacao, auditoria)          │
│                  │                                  │
│ GET /balance     │ "transfer.created" event         │
│ POST /login      │ "notification.send" command      │
│ GET /statement   │ "ledger.record" command          │
└──────────────────┴──────────────────────────────────┘
```

### Padrao 1: Event Notification (Fire and Forget)

```
Transfer Service                    Fraud Service
      │                                  │
      │  POST /transfers                 │
      │──(salva no DB)──▶                │
      │                                  │
      │  publish("transfer.created")     │
      │──────────────────────▶           │
      │                                  │──(processa async)
      │  201 Created (nao espera)        │
      │◀──────                           │
```

O Transfer Service nao espera o Fraud Service processar. Responde rapido ao cliente.

### Padrao 2: Command (Request/Reply via MQ)

```
Transfer Service                    Account Service
      │                                  │
      │  publish("account.debit.cmd")    │
      │──────────────────────▶           │
      │                                  │──(processa)
      │     publish("account.debited")   │
      │◀─────────────────────            │
      │                                  │
```

### Padrao 3: Saga (Orquestrada)

Transferencia entre duas contas exige coordenacao:

```
                     Transfer Service (Saga Orchestrator)
                              │
              1. publish("account.debit.cmd")
                              │
                     Account Service
                              │──(debita conta origem)
                              │
              2. publish("account.debited")
                              │
                     Fraud Service
                              │──(analisa transacao)
                              │
              3. publish("fraud.approved")
                              │
              4. publish("account.credit.cmd")
                              │
                     Account Service
                              │──(credita conta destino)
                              │
              5. publish("transfer.completed")
                              │
                     Notification Service
                              │──(envia comprovante)
```

Se qualquer passo falhar, **compensacoes** sao emitidas:
- Fraud rejeitou? -> compensacao: devolve debito
- Credito falhou? -> compensacao: devolve debito

---

## 6. Domain Events no Financeiro

### Eventos do PayFlow

```go
package events

import "time"

// Base de todos os eventos
type BaseEvent struct {
    EventID   string    `json:"event_id"`
    EventType string    `json:"event_type"`
    Timestamp time.Time `json:"timestamp"`
    Version   int       `json:"version"` // schema versioning
}

// Transfer Events
type TransferCreated struct {
    BaseEvent
    TransferID string  `json:"transfer_id"`
    FromAccount string `json:"from_account"`
    ToAccount   string `json:"to_account"`
    Amount      float64 `json:"amount"`
    Currency    string  `json:"currency"`
}

type TransferCompleted struct {
    BaseEvent
    TransferID string `json:"transfer_id"`
}

type TransferFailed struct {
    BaseEvent
    TransferID string `json:"transfer_id"`
    Reason     string `json:"reason"`
}

// Account Events
type AccountDebited struct {
    BaseEvent
    AccountID string  `json:"account_id"`
    Amount    float64 `json:"amount"`
    Balance   float64 `json:"balance_after"`
    Reference string  `json:"reference"` // idempotency key
}

type AccountCredited struct {
    BaseEvent
    AccountID string  `json:"account_id"`
    Amount    float64 `json:"amount"`
    Balance   float64 `json:"balance_after"`
    Reference string  `json:"reference"`
}

// Fraud Events
type FraudCheckRequested struct {
    BaseEvent
    TransferID string  `json:"transfer_id"`
    Amount     float64 `json:"amount"`
    UserID     string  `json:"user_id"`
}

type FraudApproved struct {
    BaseEvent
    TransferID string `json:"transfer_id"`
}

type FraudRejected struct {
    BaseEvent
    TransferID string `json:"transfer_id"`
    Reason     string `json:"reason"`
    RiskScore  float64 `json:"risk_score"`
}

// Notification Events
type NotificationRequested struct {
    BaseEvent
    UserID  string `json:"user_id"`
    Type    string `json:"type"` // "email", "sms", "push"
    Subject string `json:"subject"`
    Body    string `json:"body"`
}
```

### Entidade Financeira com Eventos

```go
package entities

import (
    "errors"
    "fmt"
    "github.com/google/uuid"
)

type Account struct {
    ID        uuid.UUID
    UserID    uuid.UUID
    Balance   float64
    Currency  string
    IsActive  bool
    Version   int // optimistic locking
}

// Regra de negocio: saldo nunca pode ser negativo
func (a *Account) Debit(amount float64, reference string) (*AccountDebited, error) {
    if amount <= 0 {
        return nil, errors.New("debit amount must be positive")
    }
    if a.Balance-amount < 0 {
        return nil, fmt.Errorf("insufficient funds: balance=%.2f, debit=%.2f", a.Balance, amount)
    }
    if !a.IsActive {
        return nil, errors.New("account is blocked")
    }

    a.Balance -= amount
    a.Version++

    return &AccountDebited{
        BaseEvent: BaseEvent{
            EventID:   uuid.New().String(),
            EventType: "account.debited",
            Timestamp: time.Now(),
        },
        AccountID: a.ID.String(),
        Amount:    amount,
        Balance:   a.Balance,
        Reference: reference,
    }, nil
}

func (a *Account) Credit(amount float64, reference string) (*AccountCredited, error) {
    if amount <= 0 {
        return nil, errors.New("credit amount must be positive")
    }
    if !a.IsActive {
        return nil, errors.New("account is blocked")
    }

    a.Balance += amount
    a.Version++

    return &AccountCredited{
        BaseEvent: BaseEvent{
            EventID:   uuid.New().String(),
            EventType: "account.credited",
            Timestamp: time.Now(),
        },
        AccountID: a.ID.String(),
        Amount:    amount,
        Balance:   a.Balance,
        Reference: reference,
    }, nil
}
```

---

## 7. Estrutura do Projeto

### Monorepo (recomendado para comecar)

```
payflow/
├── cmd/
│   ├── api-gateway/main.go
│   ├── account-service/main.go
│   ├── transfer-service/main.go
│   ├── auth-service/main.go
│   ├── fraud-service/main.go
│   ├── notification-service/main.go
│   └── ledger-service/main.go
├── internal/
│   ├── account/
│   │   ├── domain/
│   │   │   ├── entities/
│   │   │   └── repositories/
│   │   ├── application/
│   │   │   ├── commands/
│   │   │   ├── queries/
│   │   │   └── services/
│   │   ├── infrastructure/
│   │   │   ├── postgres/
│   │   │   └── redis/
│   │   └── interfaces/
│   │       ├── grpc/
│   │       └── consumers/        # RabbitMQ consumers
│   ├── transfer/
│   │   ├── domain/
│   │   ├── application/
│   │   ├── infrastructure/
│   │   └── interfaces/
│   ├── fraud/
│   │   └── ...
│   ├── notification/
│   │   └── ...
│   └── ledger/
│       └── ...
├── pkg/                            # Codigo compartilhado
│   ├── messaging/
│   │   ├── publisher.go
│   │   ├── consumer.go
│   │   └── config.go
│   ├── events/                     # Contratos de eventos
│   │   ├── transfer_events.go
│   │   ├── account_events.go
│   │   └── fraud_events.go
│   ├── middleware/
│   │   ├── auth.go
│   │   ├── logging.go
│   │   ├── recovery.go
│   │   └── ratelimit.go
│   ├── database/
│   │   └── postgres.go
│   └── config/
│       └── config.go
├── api/
│   └── proto/                      # gRPC definitions
│       ├── account.proto
│       └── transfer.proto
├── migrations/
│   ├── account/
│   ├── transfer/
│   └── ledger/
├── deployments/
│   ├── docker-compose.yml
│   ├── docker-compose.monitoring.yml
│   └── k8s/
├── scripts/
│   ├── setup-rabbitmq.sh
│   └── seed-data.sh
├── go.mod
└── go.sum
```

### Multi-repo (avancado)

Cada servico em seu proprio repositorio:
```
github.com/payflow/account-service
github.com/payflow/transfer-service
github.com/payflow/shared-lib (eventos, contratos)
```

---

## 8. Cada Microservico em Detalhe

### Account Service (porta 8081)

**Responsabilidade**: Gerenciar contas/wallets e saldos

| Operacao | Tipo | Tecnologia |
|----------|------|-----------|
| Criar conta | REST + Event | POST /accounts |
| Consultar saldo | REST (com cache Redis) | GET /accounts/{id}/balance |
| Debitar | Consumer MQ | listen `account.debit.cmd` |
| Creditar | Consumer MQ | listen `account.credit.cmd` |
| Bloquear conta | Consumer MQ | listen `fraud.rejected` |

**Regras de negocio**:
- Saldo nunca negativo
- Double-entry bookkeeping (debito = credito)
- Idempotencia por reference ID
- Optimistic locking (version)

### Transfer Service (porta 8082)

**Responsabilidade**: Orquestrar transferencias (Saga Orchestrator)

| Operacao | Tipo | Tecnologia |
|----------|------|-----------|
| Criar transferencia | REST | POST /transfers |
| Consultar status | REST | GET /transfers/{id} |
| Historico | REST | GET /transfers?account=X |
| Processar saga | Consumer MQ | listen events de compensacao |

**Fluxo de transferencia**:
```
1. Recebe POST /transfers
2. Salva transferencia com status PENDING
3. Publica "transfer.created"
4. Aguarda eventos de resposta
5. Coordena debito -> fraud check -> credito
6. Atualiza status para COMPLETED ou FAILED
```

### Auth Service (porta 8083)

**Responsabilidade**: Autenticacao e autorizacao

- JWT tokens (access + refresh)
- Login/register via REST
- Middleware de autenticacao compartilhado

### Fraud Service (porta 8084)

**Responsabilidade**: Analise antifraude em tempo real

| Operacao | Tipo |
|----------|------|
| Analisar transacao | Consumer MQ - listen `transfer.created` |
| Aprovar | Publisher - `fraud.approved` |
| Rejeitar | Publisher - `fraud.rejected` |

**Regras de analise**:
- Valor acima de limite -> revisao manual
- Mais de N transferencias em X minutos -> suspeita
- Destinatario nunca visto antes -> score alto
- Padrão de lavagem de dinheiro -> bloqueio

### Notification Service (porta 8085)

**Responsabilidade**: Enviar notificacoes

- Listen `transfer.completed` -> comprovante
- Listen `fraud.rejected` -> alerta
- Listen `account.credited` -> notificacao de recebimento
- Canais: email (SMTP/SendGrid), SMS (Twilio), push (Firebase)

### Ledger Service (porta 8086)

**Responsabilidade**: Registro imutavel de todas as transacoes (auditoria)

- Listen TODOS os eventos financeiros
- Registra em tabela append-only (nunca UPDATE/DELETE)
- Base para extrato, conciliacao e compliance

---

## 9. Transacoes Distribuidas

### O Problema

Em monolito, uma transacao no banco garante tudo ou nada. Em microservicos, uma transferencia toca Account Service E Transfer Service. Como garantir consistencia?

### Solucao: Saga Pattern

#### Saga Coreografada

Cada servico sabe o que fazer quando recebe um evento:

```
Transfer: publish("transfer.created")
  → Fraud: recebe, analisa, publish("fraud.approved")
    → Account: recebe, debita, publish("account.debited")
      → Account: recebe, credita destino, publish("account.credited")
        → Transfer: recebe, marca COMPLETED, publish("transfer.completed")
```

Vantagem: simples, sem coordenador central
Desvantagem: dificil rastrear o fluxo, compensacoes complexas

#### Saga Orquestrada (recomendada para financeiro)

O Transfer Service centraliza a logica:

```go
package saga

type TransferSaga struct {
    publisher *messaging.EventPublisher
    transfer  *entities.Transfer
    step      int
}

func (s *TransferSaga) Execute() error {
    switch s.step {
    case 0:
        // Step 1: Solicitar debito
        s.publisher.Publish("account.debit.cmd", DebitCommand{
            AccountID:  s.transfer.FromAccount,
            Amount:     s.transfer.Amount,
            Reference:  s.transfer.ID.String(),
        })
        s.step = 1

    case 1:
        // Step 2: Solicitar fraud check
        s.publisher.Publish("fraud.check.cmd", FraudCheckCommand{
            TransferID: s.transfer.ID.String(),
            Amount:     s.transfer.Amount,
        })
        s.step = 2

    case 2:
        // Step 3: Solicitar credito no destino
        s.publisher.Publish("account.credit.cmd", CreditCommand{
            AccountID:  s.transfer.ToAccount,
            Amount:     s.transfer.Amount,
            Reference:  s.transfer.ID.String(),
        })
        s.step = 3

    case 3:
        // Step 4: Completo
        s.transfer.Status = entities.TransferCompleted
        s.publisher.Publish("transfer.completed", TransferCompleted{
            TransferID: s.transfer.ID.String(),
        })
    }
    return nil
}

func (s *TransferSaga) Compensate(reason string) {
    switch s.step {
    case 1:
        // Debit ja feito, compensar com credito
        s.publisher.Publish("account.credit.cmd", CreditCommand{
            AccountID: s.transfer.FromAccount,
            Amount:    s.transfer.Amount,
            Reference: "compensate:" + s.transfer.ID.String(),
        })
    }
    s.transfer.Status = entities.TransferFailed
    s.transfer.FailureReason = reason
}
```

### Idempotencia em Transacoes Distribuidas

Cada operacao usa um **reference ID** unico. Se o consumer receber a mesma mensagem duas vezes (reconexao, retry), o resultado e o mesmo:

```go
func (s *AccountService) HandleDebit(cmd DebitCommand) error {
    // Verifica se ja processou esse reference
    existing, _ := s.repo.GetByReference(cmd.Reference)
    if existing != nil {
        s.logger.Info("duplicate debit ignored", "reference", cmd.Reference)
        return nil // Ja processou, idempotente
    }

    // Processa debito
    account, _ := s.repo.GetByID(cmd.AccountID)
    event, err := account.Debit(cmd.Amount, cmd.Reference)
    if err != nil {
        return err
    }

    // Salva com reference
    s.repo.SaveWithReference(account, cmd.Reference)
    s.publisher.Publish("account.debited", event)
    return nil
}
```

---

## 10. Seguranca no Financeiro

### Autenticacao e Autorizacao

```go
// JWT Middleware
func AuthMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        tokenString := r.Header.Get("Authorization")
        tokenString = strings.TrimPrefix(tokenString, "Bearer ")

        token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
            return []byte(os.Getenv("JWT_SECRET")), nil
        })

        if err != nil || !token.Valid {
            http.Error(w, "unauthorized", http.StatusUnauthorized)
            return
        }

        // Adiciona claims no contexto
        claims := token.Claims.(jwt.MapClaims)
        ctx := context.WithValue(r.Context(), "user_id", claims["sub"])
        ctx = context.WithValue(ctx, "account_id", claims["account_id"])

        next.ServeHTTP(w, r.WithContext(ctx))
    })
}
```

### Protecoes Especificas do Financeiro

| Protecao | Implementacao |
|----------|--------------|
| Idempotencia | Reference ID unico por operacao |
| Optimistic Locking | Version na entidade Account |
| Double-entry | Debito sempre igual ao credito |
| Rate Limiting | Redis: max N transferencias por minuto |
| Auditoria | Ledger service (append-only) |
| Criptografia | Dados sensiveis criptografados no banco |
| TLS | Toda comunicacao criptografada |
| Secrets | Vault ou env vars, nunca hardcoded |

### Valores Monetarios

**NUNCA use `float64` para dinheiro em producao!** Use inteiros (centavos) ou `decimal`:

```go
import "github.com/shopspring/decimal"

type Money struct {
    Amount   decimal.Decimal
    Currency string
}

func NewMoney(amount int64, currency string) Money {
    return Money{
        Amount:   decimal.NewFromInt(amount), // em centavos
        Currency: currency,
    }
}

func (m Money) Add(other Money) (Money, error) {
    if m.Currency != other.Currency {
        return Money{}, errors.New("currency mismatch")
    }
    return Money{Amount: m.Amount.Add(other.Amount), Currency: m.Currency}, nil
}
```

---

## 11. Observabilidade

### Logging Estruturado

```go
import "log/slog"

// Setup no main.go
logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
    Level: slog.LevelInfo,
}))

// Em qualquer lugar
logger.Info("transfer created",
    "transfer_id", transfer.ID,
    "from_account", transfer.FromAccount,
    "to_account", transfer.ToAccount,
    "amount", transfer.Amount,
    "correlation_id", correlationID,
)
```

### Distributed Tracing

Cada request recebe um **correlation ID** que passa por todos os servicos:

```go
// Middleware que gera/propaga correlation ID
func CorrelationIDMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        corrID := r.Header.Get("X-Correlation-ID")
        if corrID == "" {
            corrID = uuid.New().String()
        }

        ctx := context.WithValue(r.Context(), "correlation_id", corrID)
        w.Header().Set("X-Correlation-ID", corrID)

        next.ServeHTTP(w, r.WithContext(ctx))
    })
}
```

No RabbitMQ, o correlation ID vai no header da mensagem:

```go
amqp.Publishing{
    Headers: amqp.Table{
        "X-Correlation-ID": correlationID,
    },
    Body: body,
}
```

### Health Checks

```go
func HealthCheck(w http.ResponseWriter, r *http.Request) {
    // Verifica dependencias
    if err := db.Ping(); err != nil {
        json.NewEncoder(w).Encode(map[string]string{
            "status":   "unhealthy",
            "database": "down",
        })
        w.WriteHeader(http.StatusServiceUnavailable)
        return
    }

    json.NewEncoder(w).Encode(map[string]string{
        "status": "healthy",
    })
}
```

### Docker Compose com Monitoramento

```yaml
# docker-compose.monitoring.yml
version: "3.8"
services:
  prometheus:
    image: prom/prometheus
    ports: ["9090:9090"]
    volumes:
      - ./monitoring/prometheus.yml:/etc/prometheus/prometheus.yml

  grafana:
    image: grafana/grafana
    ports: ["3000:3000"]

  jaeger:
    image: jaegertracing/all-in-one
    ports: ["16686:16686", "4318:4318"]
```

---

## 12. Roadmap de Implementacao

### Fase 1: Fundacao (Semanas 1-2)

```
1. Inicializar modulo Go
   go mod init github.com/seuuser/payflow

2. Docker Compose base
   - PostgreSQL
   - RabbitMQ (com management UI em :15672)
   - Redis

3. pkg/ compartilhado
   - config (viper)
   - database (pgx connection)
   - messaging (publisher + consumer)
   - events (contratos)
   - middleware (auth, logging, correlation)

4. API Gateway basico
   - chi router
   - proxy para servicos
   - JWT middleware
```

### Fase 2: Auth Service (Semana 3)

```
1. CRUD de usuarios
2. Login com JWT (access + refresh)
3. Hash de senhas com bcrypt
4. Middleware de auth compartilhado
```

### Fase 3: Account Service (Semanas 4-5)

```
1. Criar conta/wallet
2. Consultar saldo (com cache Redis)
3. Consumer MQ: debitar/creditar
4. Idempotencia por reference
5. Optimistic locking
6. Testes unitarios e de integracao
```

### Fase 4: Transfer Service (Semanas 6-7)

```
1. Criar transferencia (REST)
2. Saga orquestrada
3. Compensacoes
4. Consultar status
5. Historico
6. Testes de saga end-to-end
```

### Fase 5: Fraud Service (Semana 8)

```
1. Consumer de transfer.created
2. Regras de analise (valor limite, frequencia, score)
3. Publicar fraud.approved ou fraud.rejected
4. Testes com cenarios de fraude
```

### Fase 6: Notification + Ledger (Semanas 9-10)

```
1. Notification: consumers de eventos, envio de email/SMS
2. Ledger: consumer de todos os eventos, registro append-only
3. Extrato via Ledger
```

### Fase 7: Producao (Semanas 11-12)

```
1. Docker Compose completo
2. Prometheus + Grafana
3. Distributed tracing (Jaeger)
4. Graceful shutdown
5. Circuit breaker (go-resilience)
6. Rate limiting (Redis)
7. Load testing (k6)
8. CI/CD basico (GitHub Actions)
```

---

## 13. Exemplos de Codigo

### docker-compose.yml (base)

```yaml
version: "3.8"

services:
  postgres:
    image: postgres:16
    environment:
      POSTGRES_USER: payflow
      POSTGRES_PASSWORD: payflow123
      POSTGRES_DB: payflow
    ports:
      - "5432:5432"
    volumes:
      - pgdata:/var/lib/postgresql/data

  rabbitmq:
    image: rabbitmq:3-management
    environment:
      RABBITMQ_DEFAULT_USER: payflow
      RABBITMQ_DEFAULT_PASS: payflow123
    ports:
      - "5672:5672"    # AMQP
      - "15672:15672"  # Management UI

  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"

volumes:
  pgdata:
```

### main.go (Account Service)

```go
package main

import (
    "context"
    "log/slog"
    "net/http"
    "os"
    "os/signal"
    "syscall"
    "time"

    "github.com/go-chi/chi/v5"
    "github.com/go-chi/chi/v5/middleware"
    amqp "github.com/rabbitmq/amqp091-go"
    "github.com/jackc/pgx/v5/pgxpool"
    "github.com/redis/go-redis/v9"
)

func main() {
    logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

    // DB
    db, err := pgxpool.New(context.Background(), os.Getenv("DATABASE_URL"))
    if err != nil {
        logger.Error("failed to connect to database", "error", err)
        os.Exit(1)
    }
    defer db.Close()

    // Redis
    rdb := redis.NewClient(&redis.Options{Addr: os.Getenv("REDIS_URL")})
    defer rdb.Close()

    // RabbitMQ
    amqpConn, err := amqp.Dial(os.Getenv("RABBITMQ_URL"))
    if err != nil {
        logger.Error("failed to connect to RabbitMQ", "error", err)
        os.Exit(1)
    }
    defer amqpConn.Close()

    // Messaging
    publisher, _ := messaging.NewEventPublisher(amqpConn, logger)
    consumer, _ := messaging.NewEventConsumer(amqpConn, logger)

    // Repositories
    accountRepo := postgres.NewAccountRepository(db)

    // Services
    accountService := services.NewAccountService(accountRepo, publisher, rdb)

    // Consumers (async)
    consumer.Subscribe("account.debit.queue", "account.debit.cmd", accountService.HandleDebit)
    consumer.Subscribe("account.credit.queue", "account.credit.cmd", accountService.HandleCredit)
    consumer.Subscribe("account.block.queue", "fraud.rejected", accountService.HandleBlock)

    // HTTP
    r := chi.NewRouter()
    r.Use(middleware.Logger)
    r.Use(middleware.Recoverer)
    r.Use(CorrelationIDMiddleware)

    r.Get("/health", HealthCheck)
    r.Route("/accounts", func(r chi.Router) {
        r.Use(AuthMiddleware)
        r.Post("/", accountHandler.Create)
        r.Get("/{id}/balance", accountHandler.GetBalance)
    })

    // Graceful shutdown
    srv := &http.Server{Addr: ":8081", Handler: r}

    go func() {
        logger.Info("account service starting", "port", 8081)
        if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
            logger.Error("server error", "error", err)
            os.Exit(1)
        }
    }()

    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
    <-quit

    logger.Info("shutting down gracefully...")
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()
    srv.Shutdown(ctx)
}
```

### Config com Viper

```go
// pkg/config/config.go
package config

import "github.com/spf13/viper"

type Config struct {
    DatabaseURL  string `mapstructure:"DATABASE_URL"`
    RabbitMQURL  string `mapstructure:"RABBITMQ_URL"`
    RedisURL     string `mapstructure:"REDIS_URL"`
    JWTSecret    string `mapstructure:"JWT_SECRET"`
    ServicePort  string `mapstructure:"SERVICE_PORT"`
}

func Load() *Config {
    viper.AutomaticEnv()
    viper.SetDefault("SERVICE_PORT", "8081")
    viper.SetDefault("DATABASE_URL", "postgres://payflow:payflow123@localhost:5432/payflow")
    viper.SetDefault("RABBITMQ_URL", "amqp://payflow:payflow123@localhost:5672/")
    viper.SetDefault("REDIS_URL", "localhost:6379")

    return &Config{
        DatabaseURL:  viper.GetString("DATABASE_URL"),
        RabbitMQURL:  viper.GetString("RABBITMQ_URL"),
        RedisURL:     viper.GetString("REDIS_URL"),
        JWTSecret:    viper.GetString("JWT_SECRET"),
        ServicePort:  viper.GetString("SERVICE_PORT"),
    }
}
```

---

## Resumo

| Conceito | Ferramenta | No PayFlow |
|----------|-----------|-----------|
| Mensageria | RabbitMQ | Comunicacao entre todos os servicos |
| Saga Pattern | Orquestrada | Transferencia coordena debito -> fraud -> credito |
| Idempotencia | Reference ID | Nunca debitar duas vezes |
| Cache | Redis | Saldo em cache |
| Auth | JWT | API Gateway valida tokens |
| Auditoria | Ledger append-only | Todas as transacoes registradas |
| Antifraude | Regras + Events | Analise assincrona via MQ |
| Observabilidade | slog + Prometheus + Jaeger | Logs, metricas, tracing |
| Money | decimal ou int64 (centavos) | Nunca float64 |

> **Proximo passo**: Clone o repositorio go-ddd como referencia de DDD, e comece pelo docker-compose.yml + Account Service. Um servico de cada vez.
