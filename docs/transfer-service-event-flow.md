# Transfer Service - Fluxo de Eventos com Account Service

## Arquitetura Atual do Account

O account já publica eventos no exchange `payflow.events` (topic):
- `account.credited`
- `account.debited`
- `account.blocked`

E consome comandos nas filas:
- `account-service.credit.cmd`
- `account-service.debit.cmd`

---

## Diagrama do Fluxo

```
                    ┌─────────────────────┐
                    │   TRANSFER SERVICE   │
                    └──────────┬──────────┘
                               │
                    1. Publica comando DEBIT
                               │
                               ▼
┌──────────────────────────────────────────────────────┐
│  RabbitMQ Exchange: payflow.commands (direct/topic)   │
└──────────┬───────────────────────────┬───────────────┘
           │                           │
           ▼                           ▼
┌─────────────────────┐    ┌─────────────────────┐
│  account-service     │    │  account-service     │
│  .debit.cmd          │    │  .credit.cmd         │
│  (debita origem)     │    │  (credita destino)   │
└──────────┬──────────┘    └──────────┬──────────┘
           │                          │
           ▼                          ▼
┌─────────────────────┐    ┌─────────────────────┐
│   ACCOUNT SERVICE   │    │   ACCOUNT SERVICE   │
│   DebitAccount()    │    │   CreditAccount()   │
└──────────┬──────────┘    └──────────┬──────────┘
           │                          │
           ▼                          ▼
    Publica evento               Publica evento
  account.debited            account.credited
           │                          │
           ▼                          ▼
┌──────────────────────────────────────────────────────┐
│  RabbitMQ Exchange: payflow.events (topic)            │
└──────────────────────────┬───────────────────────────┘
                           │
              ┌────────────┼────────────┐
              ▼                         ▼
   transfer-service              transfer-service
   .transfer.events             .transfer.events
   (consome ambos)              (consome ambos)
```

---

## Fluxo Passo a Passo

### 1. Transfer Service recebe request

```
POST /transfers
{
  "from_account_id": "uuid-aaa",
  "to_account_id": "uuid-bbb",
  "amount": 10000,  // R$ 100,00 em centavos
  "reference": "tf-uuid-123"
}
```

### 2. Transfer Service publica comando de débito

```go
// Fila: account-service.debit.cmd
{
  "account_id": "uuid-aaa",
  "amount": 10000,
  "reference": "tf-uuid-123"  // mesma reference = idempotência
}
```

### 3. Account Service processa débito

- `DebitAccount()` verifica saldo, bloqueio otimista, debita
- Publica evento `account.debited` no exchange `payflow.events`
- Se saldo insuficiente: publica `account.debit.failed` (novo evento) ou retorna erro na DLQ

### 4. Transfer Service consome `account.debited`

- Filtra pelo `reference` da transferência
- Muda estado interno: `PENDING_DEBIT → DEBITED`
- Publica comando de crédito:

```go
// Fila: account-service.credit.cmd
{
  "account_id": "uuid-bbb",
  "amount": 10000,
  "reference": "tf-uuid-123-credit"  // reference única p/ crédito
}
```

### 5. Account Service processa crédito

- `CreditAccount()` credita destino
- Publica `account.credited`

### 6. Transfer Service consome `account.credited`

- Muda estado: `DEBITED → COMPLETED`
- Publica `transfer.completed`

---

## Fluxo de Falha (Compensação)

Se o **crédito falhar** (conta destino bloqueada/inexistente):

```
transfer-service detecta falha no crédito
       │
       ▼
Publica comando de compensação (estorno):
  Fila: account-service.credit.cmd
  {
    "account_id": "uuid-aaa",      // devolve pra origem
    "amount": 10000,
    "reference": "tf-uuid-123-refund"
  }
       │
       ▼
Account credita de volta → account.credited
       │
       ▼
Transfer consome → estado: FAILED (com estorno realizado)
Publica: transfer.failed
```

---

## Novos Eventos Necessários

### No Account Service — adicionar:

```go
// pkg/events/account_events.go
AccountDebitFailedEvent = "account.debit.failed"  // saldo insuficiente, conta bloqueada
AccountCreditFailedEvent = "account.credit.failed" // conta não existe, bloqueada
```

### No Transfer Service — criar:

```go
// Eventos publicados
TransferInitiatedEvent  = "transfer.initiated"
TransferCompletedEvent  = "transfer.completed"
TransferFailedEvent     = "transfer.failed"
TransferRefundedEvent   = "transfer.refunded"

// Eventos consumidos (do account)
account.debited
account.credited
account.debit.failed
account.credit.failed
```

---

## Tabela de Estados da Transferência

```
PENDING_DEBIT → DEBITED → PENDING_CREDIT → COMPLETED
     │                        │
     ▼                        ▼
  FAILED                  PENDING_REFUND → REFUNDED → FAILED
 (débito falhou)         (crédito falhou, estornando)
```

---

## Observações Arquiteturais

- **Reference compartilhada**: O `reference` da transferência (`tf-uuid-123`) é usado como idempotency key tanto no débito quanto no crédito. Para o crédito de compensação, usa-se `tf-uuid-123-refund`
- **O Transfer nunca acessa o DB do Account**: Toda comunicação via RabbitMQ. Account é o dono dos dados de saldo
- **Coreografia > Orquestração**: Sem saga centralizada — cada serviço reage a eventos. Mais simples, mas exige estado na transfer
- **Dead Letter Queue**: Se o Account não conseguir processar, vai pra DLQ. O Transfer precisa de um timeout para detectar que não recebeu resposta e compensar
