# Guia Completo: APIs Eficientes em Go com DDD, CQRS e Clean Architecture

> Baseado na analise do repositorio [sklinkert/go-ddd](https://github.com/sklinkert/go-ddd)
> Um manual pratico para quem esta aprendendo Go e quer construir APIs profissionais.

---

## Sumario

1. [Organizacao de Projeto](#1-organizacao-de-projeto)
2. [Domain-Driven Design (DDD)](#2-domain-driven-design-ddd)
3. [Clean Architecture / Onion Architecture](#3-clean-architecture--onion-architecture)
4. [CQRS - Command Query Responsibility Segregation](#4-cqrs---command-query-responsibility-segregation)
5. [Padroes Essenciais](#5-padroes-essenciais)
6. [Bibliotecas Importantes](#6-bibliotecas-importantes)
7. [Testes em Go](#7-testes-em-go)
8. [Roadmap de Aprendizado](#8-roadmap-de-aprendizado)
9. [Checklist: Projeto Go Profissional](#9-checklist-projeto-go-profissional)
10. [Exemplos de Codigo por Camada](#10-exemplos-de-codigo-por-camada)

---

## 1. Organizacao de Projeto

### Estrutura de Diretorios (go-ddd)

```
go-ddd/
├── cmd/
│   └── marketplace/
│       └── main.go                  # Entry point da aplicacao
├── internal/
│   ├── domain/
│   │   ├── entities/                # Entidades de negocio (Product, Seller)
│   │   └── repositories/            # Interfaces de repositorio (contratos)
│   ├── application/
│   │   ├── command/                 # Comandos CQRS (Create, Update, Delete)
│   │   ├── query/                   # Queries CQRS (GetById, GetAll)
│   │   ├── interfaces/              # Interfaces de servico
│   │   ├── mapper/                  # Conversores Entity <-> DTO
│   │   └── services/                # Casos de uso / logica de aplicacao
│   ├── infrastructure/
│   │   └── db/
│   │       ├── postgres/            # Implementacoes concretas dos repositorios
│   │       └── sqlc/                # Codigo gerado pelo sqlc
│   └── interface/
│       └── api/
│           └── rest/                # Controllers HTTP / Handlers
│               └── dto/             # Request/Response DTOs
├── sqlc.yaml                        # Configuracao do sqlc
├── go.mod
└── go.sum
```

### Regra de Ouro: `internal/`

O Go possui uma convencao especial: o diretorio `internal/` so pode ser importado pelo codigo dentro do mesmo modulo. Isso **protege sua logica de negocio** de ser importada por projetos externos.

### Nomenclatura de Pacotes

| Camada | Pacote | Responsabilidade |
|--------|--------|-----------------|
| Domain | `entities` | Entidades com regras de negocio |
| Domain | `repositories` | Contratos (interfaces) de persistencia |
| Application | `services` | Orquestracao de casos de uso |
| Application | `command` | Estruturas de dados para escrita |
| Application | `query` | Estruturas de dados para leitura |
| Application | `mapper` | Conversao entre camadas |
| Infrastructure | `postgres` | Implementacao de banco de dados |
| Interface | `rest` | Controllers HTTP |

**Dica**: Nomes de pacotes em Go sao sempre **minusculos, sem underline**, e preferencialmente uma unica palavra.

---

## 2. Domain-Driven Design (DDD)

### O que e DDD?

DDD e uma abordagem que coloca o **dominio de negocio** no centro do software. A logica de negocio nao depende de frameworks, bancos de dados ou APIs - ela e independente e testavel isoladamente.

### Entidades com Comportamento (Nao sao structs burros!)

No go-ddd, as entidades tem **metodos com validacao**:

```go
// internal/domain/entities/product.go
package entities

import (
    "errors"
    "github.com/google/uuid"
    "time"
)

type Product struct {
    Id        uuid.UUID
    CreatedAt time.Time
    UpdatedAt time.Time
    Name      string
    Price     float64
    Seller    Seller
}

// Validacao privada - chamada por todos os metodos que modificam a entidade
func (p *Product) validate() error {
    if p.Name == "" {
        return errors.New("product name cannot be empty")
    }
    if p.Price <= 0 {
        return errors.New("product price must be greater than zero")
    }
    if p.CreatedAt.IsZero() || p.UpdatedAt.IsZero() {
        return errors.New("timestamps cannot be zero")
    }
    return nil
}

// Construtor que ja cria entidade valida com UUID v7
func NewProduct(name string, price float64, seller ValidatedSeller) *Product {
    now := time.Now()
    return &Product{
        Id:        uuid.NewV7(),
        CreatedAt: now,
        UpdatedAt: now,
        Name:      name,
        Price:     price,
        Seller:    seller.Seller,
    }
}

// Metodos de dominio - encapsulam regras de negocio
func (p *Product) UpdateName(name string) error {
    p.Name = name
    p.UpdatedAt = time.Now()
    return p.validate() // Sempre valida apos modificacao
}

func (p *Product) UpdatePrice(price float64) error {
    p.Price = price
    p.UpdatedAt = time.Now()
    return p.validate()
}
```

**Principio**: A entidade nunca permite um estado invalido. Toda modificacao passa por validacao.

### Validated Types (Tipos Validados)

O go-ddd usa um padrao poderoso: **tipos validados** que envolvem entidades base:

```go
// internal/domain/entities/validated_product.go
package entities

type ValidatedProduct struct {
    Product Product
}

// So pode ser criado se o produto for valido
func NewValidatedProduct(product Product) (*ValidatedProduct, error) {
    if err := product.validate(); err != nil {
        return nil, err
    }
    return &ValidatedProduct{Product: product}, nil
}
```

```go
// internal/domain/entities/validated_seller.go
package entities

type ValidatedSeller struct {
    Seller Seller
}

func NewValidatedSeller(seller Seller) (*ValidatedSeller, error) {
    if err := seller.validate(); err != nil {
        return nil, err
    }
    return &ValidatedSeller{Seller: seller}, nil
}
```

**Por que isso importa?** Funcoes que recebem `ValidatedProduct` tem a garantia em **tempo de compilacao** de que o produto ja foi validado. Isso elimina verificacoes repetidas e torna o codigo mais seguro.

### Padrao de Idempotencia

O go-ddd implementa idempotencia para evitar processamento duplicado de requests:

```go
// internal/domain/entities/idempotency_record.go
package entities

type IdempotencyRecord struct {
    Key         string
    Response    string
    CreatedAt   string
}
```

No service, antes de processar qualquer comando:
1. Verifica se ja existe um registro com a chave de idempotencia
2. Se existe, retorna a resposta cacheada
3. Se nao existe, processa e salva o registro

---

## 3. Clean Architecture / Onion Architecture

### Fluxo de Dependencia

```
┌──────────────────────────────────────────────┐
│                 main.go                       │
│          (Composition Root / Wiring)          │
└──────────────┬───────────────────────────────┘
               │
               ▼
┌──────────────────────────────────────────────┐
│          Interface Layer (REST)               │
│    Controllers que recebem HTTP requests      │
│         e retornam HTTP responses             │
└──────────────┬───────────────────────────────┘
               │
               ▼
┌──────────────────────────────────────────────┐
│         Application Layer (Services)          │
│     Casos de uso, Commands, Queries,          │
│              Mappers                          │
└──────────────┬───────────────────────────────┘
               │
               ▼
┌──────────────────────────────────────────────┐
│          Domain Layer (Entities)              │
│   Regras de negocio puras, sem dependencias   │
│   externas. Define INTERFACES de repositorio  │
└──────────────────────────────────────────────┘
               ▲
               │ (implementa as interfaces)
┌──────────────┴───────────────────────────────┐
│       Infrastructure Layer (Postgres)         │
│   Implementacoes concretas de repositorio,    │
│   codigo gerado pelo sqlc                     │
└──────────────────────────────────────────────┘
```

### Regra Fundamental

**A seta de dependencia sempre aponta para DENTRO.** O dominio nao conhece nada sobre:
- HTTP
- Banco de dados
- Frameworks
- Mensageria

### Interfaces de Repositorio (Definidas no Dominio!)

```go
// internal/domain/repositories/product_repository.go
package repositories

import (
    "github.com/sklinkert/go-ddd/internal/domain/entities"
)

type ProductRepository interface {
    GetAll() ([]entities.Product, error)
    GetById(id uuid.UUID) (*entities.Product, error)
    Create(product entities.Product) error
    Update(product entities.Product) error
    Delete(id uuid.UUID) error
}
```

**Ponto chave**: A interface e definida no dominio, mas a implementacao fica na infrastructure. Isso e **Dependency Inversion**.

### Interfaces de Servico (Para Testabilidade)

```go
// internal/application/interfaces/product_service.go
package interfaces

type ProductService interface {
    CreateProduct(cmd *command.CreateProductCommand) (*command.CreateProductCommandResult, error)
    UpdateProduct(cmd *command.UpdateProductCommand) error
    DeleteProduct(cmd *command.DeleteProductCommand) error
    GetAllProducts() (*query.GetAllProductsQueryResult, error)
    GetProductById(query *query.GetProductByIdQuery) (*query.ProductQueryResult, error)
}
```

O controller recebe essa **interface**, nao a implementacao concreta. Assim voce pode testar o controller com um mock.

---

## 4. CQRS - Command Query Responsibility Segregation

### Conceito

CQRS separa operacoes de **escrita** (Commands) de operacoes de **leitura** (Queries). Cada tipo de operacao tem sua estrutura de dados propria.

### Commands (Escrita)

```go
// internal/application/command/create_product_command.go
package command

type CreateProductCommand struct {
    Name           string
    Price          float64
    SellerId       string
    IdempotencyKey string
}

type CreateProductCommandResult struct {
    Id uuid.UUID
}
```

```go
// internal/application/command/update_product_command.go
package command

type UpdateProductCommand struct {
    Id    uuid.UUID
    Name  string
    Price float64
}
```

```go
// internal/application/command/delete_product_command.go
package command

type DeleteProductCommand struct {
    Id uuid.UUID
}
```

### Queries (Leitura)

```go
// internal/application/query/get_product_by_id_query.go
package query

import "github.com/google/uuid"

type GetProductByIdQuery struct {
    Id uuid.UUID
}
```

```go
// internal/application/query/product_query_result.go
package query

type ProductQueryResult struct {
    Id        string  `json:"id"`
    Name      string  `json:"name"`
    Price     float64 `json:"price"`
    SellerId  string  `json:"seller_id"`
    SellerName string `json:"seller_name"`
    CreatedAt string  `json:"created_at"`
    UpdatedAt string  `json:"updated_at"`
}
```

### Service com CQRS Completo

```go
// Fluxo do CreateProduct no service:
func (s *ProductService) CreateProduct(cmd *command.CreateProductCommand) (*command.CreateProductCommandResult, error) {
    // 1. Idempotency check
    existingRecord, _ := s.idempotencyRepo.FindByKey(cmd.IdempotencyKey)
    if existingRecord != nil {
        // Retorna resposta cacheada
        var result command.CreateProductCommandResult
        json.Unmarshal([]byte(existingRecord.Response), &result)
        return &result, nil
    }

    // 2. Valida existencia do seller
    seller, err := s.sellerRepository.GetById(uuid.MustParse(cmd.SellerId))
    if err != nil {
        return nil, err
    }

    // 3. Cria tipos validados (garantia em tempo de compilacao)
    validatedSeller, err := entities.NewValidatedSeller(*seller)
    if err != nil {
        return nil, err
    }

    // 4. Cria entidade de dominio
    product := entities.NewProduct(cmd.Name, cmd.Price, *validatedSeller)

    // 5. Valida o produto
    validatedProduct, err := entities.NewValidatedProduct(*product)
    if err != nil {
        return nil, err
    }

    // 6. Persiste via repositorio
    err = s.productRepository.Create(validatedProduct.Product)
    if err != nil {
        return nil, err
    }

    // 7. Salva registro de idempotencia
    result := &command.CreateProductCommandResult{Id: product.Id}
    resultJson, _ := json.Marshal(result)
    s.idempotencyRepo.Save(entities.IdempotencyRecord{
        Key:      cmd.IdempotencyKey,
        Response: string(resultJson),
    })

    // 8. Retorna resultado
    return result, nil
}
```

---

## 5. Padroes Essenciais

### Repository Pattern

O repositorio **abstrai o acesso a dados**. O dominio define a interface, a infraestrutura implementa:

```go
// Domain define o contrato
type ProductRepository interface {
    GetAll() ([]entities.Product, error)
    GetById(id uuid.UUID) (*entities.Product, error)
    Create(product entities.Product) error
}

// Infrastructure implementa com Postgres
type postgresProductRepository struct {
    db *sql.DB
}

func NewPostgresProductRepository(db *sql.DB) repositories.ProductRepository {
    return &postgresProductRepository{db: db}
}
```

### Mapper / DTO Pattern

Separar dados internos (entities) dos dados expostos pela API (DTOs):

```go
// internal/application/mapper/product_mapper.go
package mapper

import (
    "github.com/sklinkert/go-ddd/internal/application/query"
    "github.com/sklinkert/go-ddd/internal/domain/entities"
)

func MapProductToQueryResult(product entities.Product) query.ProductQueryResult {
    return query.ProductQueryResult{
        Id:         product.Id.String(),
        Name:       product.Name,
        Price:      product.Price,
        SellerId:   product.Seller.Id.String(),
        SellerName: product.Seller.Name,
        CreatedAt:  product.CreatedAt.Format(time.RFC3339),
        UpdatedAt:  product.UpdatedAt.Format(time.RFC3339),
    }
}
```

### Dependency Injection (Constructor Injection)

```go
// internal/application/services/product_service.go
type ProductService struct {
    productRepository repositories.ProductRepository   // interface, nao struct concreto!
    sellerRepository  repositories.SellerRepository
    idempotencyRepo   repositories.IdempotencyRepository
}

func NewProductService(
    productRepository repositories.ProductRepository,
    sellerRepository repositories.SellerRepository,
    idempotencyRepo repositories.IdempotencyRepository,
) interfaces.ProductService {
    return &ProductService{
        productRepository: productRepository,
        sellerRepository:  sellerRepository,
        idempotencyRepo:   idempotencyRepo,
    }
}
```

### Composition Root (main.go)

Todo o wiring acontece no `main.go`:

```go
// cmd/marketplace/main.go
func main() {
    db := connectDB() // conexao com banco

    // Infrastructure (implementacoes concretas)
    productRepo := postgres.NewPostgresProductRepository(db)
    sellerRepo := postgres.NewPostgresSellerRepository(db)
    idempotencyRepo := postgres.NewPostgresIdempotencyRepository(db)

    // Application (services recebem interfaces)
    productService := services.NewProductService(productRepo, sellerRepo, idempotencyRepo)
    sellerService := services.NewSellerService(sellerRepo)

    // Interface (controllers recebem interfaces de service)
    productController := rest.NewProductController(productService)
    sellerController := rest.NewSellerController(sellerService)

    // HTTP routes
    router := setupRoutes(productController, sellerController)
    http.ListenAndServe(":8080", router)
}
```

**Principio**: `main.go` e o unico lugar que sabe quais sao as implementacoes concretas. Todo o resto trabalha com interfaces.

---

## 6. Bibliotecas Importantes

### Dependencias do go-ddd

| Biblioteca | Funcao | Uso |
|-----------|--------|-----|
| `github.com/google/uuid` | Geracao de UUIDs | IDs unicos para entidades (`uuid.NewV7()`) |
| `github.com/lib/pq` | Driver PostgreSQL | Conexao com banco Postgres |
| `github.com/golang-migrate/migrate` | Migrations SQL | Versionamento do schema do banco |
| `github.com/stretchr/testify` | Assercoes de teste | `assert.Equal`, `assert.NoError`, mocks |

### Ferramentas Importantes

#### sqlc - Type-Safe SQL Code Generation

**O que e**: Gera codigo Go type-safe a partir de queries SQL. Sem ORM, sem reflection, sem surpresas em runtime.

```yaml
# sqlc.yaml
version: "2"
sql:
  - engine: "postgresql"
    queries: "queries/"
    schema: "migrations/"
    gen:
      go:
        package: "sqlc"
        out: "sqlc"
```

Voce escreve SQL puro:
```sql
-- queries/product.sql
-- name: GetProductById :one
SELECT * FROM products WHERE id = $1;

-- name: GetAllProducts :many
SELECT * FROM products ORDER BY created_at;

-- name: CreateProduct :one
INSERT INTO products (id, name, price, seller_id, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6) RETURNING *;
```

E o sqlc gera funcoes Go com tipos corretos automaticamente.

**Vantagens sobre ORM**:
- Sem magic, sem reflection
- Erros de SQL detectados em tempo de geracao
- Performance otima (codigo gerado e direto)
- Voce controla exatamente qual SQL roda

### Bibliotecas Recomendadas Para APIs Go

| Biblioteca | Categoria | Quando Usar |
|-----------|-----------|------------|
| `net/http` (stdlib) | HTTP Server | APIs simples, aprendizado |
| `github.com/go-chi/chi/v5` | Router HTTP | APIs profissionais, leve e rapido |
| `github.com/gin-gonic/gin` | Framework HTTP | APIs com muitas features built-in |
| `github.com/golang-jwt/jwt/v5` | JWT | Autenticacao token-based |
| `github.com/google/uuid` | UUID | IDs unicos |
| `github.com/stretchr/testify` | Testes | Assercoes e mocks |
| `github.com/jmoiron/sqlx` | SQL Helper | Quando nao usa sqlc |
| `github.com/golang-migrate/migrate` | Migrations | Versionamento de schema |
| `github.com/sqlc-dev/sqlc` | Code Gen | Type-safe SQL sem ORM |
| `github.com/spf13/cobra` | CLI | Aplicacoes de linha de comando |
| `github.com/jackc/pgx` | PostgreSQL | Driver Postgres moderno e performatico |

---

## 7. Testes em Go

### Estrutura de Testes no go-ddd

```
internal/domain/entities/
├── product.go
├── product_test.go                    # Teste basico
├── product_business_logic_test.go     # Teste de regras de negocio
├── validated_product.go
├── validated_product_test.go          # Teste de tipos validados
├── idempotency_record.go
└── idempotency_record_test.go

internal/application/services/
├── product_service.go
└── product_service_test.go            # Teste do service com mocks

internal/interface/api/rest/
├── product_controller.go
└── rest_test/
    └── product_controller_test.go     # Teste do controller com mock service
```

### Padroes de Teste

**1. Teste de Entidade (Dominio puro)**:
```go
func TestNewProduct(t *testing.T) {
    seller := entities.NewSeller("Test Seller")
    validatedSeller, _ := entities.NewValidatedSeller(*seller)

    product := entities.NewProduct("Widget", 9.99, *validatedSeller)

    assert.NotNil(t, product)
    assert.NotEmpty(t, product.Id)
    assert.Equal(t, "Widget", product.Name)
}
```

**2. Teste de Regra de Negocio**:
```go
func TestProduct_UpdateName_CannotBeEmpty(t *testing.T) {
    product := createTestProduct()

    err := product.UpdateName("")

    assert.Error(t, err)
    assert.Equal(t, "product name cannot be empty", err.Error())
}
```

**3. Teste de Service com Mock**:
```go
type mockProductRepository struct {
    products []entities.Product
}

func (m *mockProductRepository) GetAll() ([]entities.Product, error) {
    return m.products, nil
}
// ... implementa outras funcoes da interface

func TestProductService_GetAllProducts(t *testing.T) {
    mockRepo := &mockProductRepository{products: testProducts}
    service := services.NewProductService(mockRepo, mockSellerRepo, mockIdempotencyRepo)

    result, err := service.GetAllProducts()

    assert.NoError(t, err)
    assert.Len(t, result.Products, 2)
}
```

**4. Table-Driven Tests (Padrao Go)**:
```go
func TestProductValidation(t *testing.T) {
    tests := []struct {
        name      string
        product   entities.Product
        wantError bool
    }{
        {"valid product", validProduct(), false},
        {"empty name", productWithEmptyName(), true},
        {"zero price", productWithZeroPrice(), true},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := tt.product.validate()
            if tt.wantError {
                assert.Error(t, err)
            } else {
                assert.NoError(t, err)
            }
        })
    }
}
```

### Convencoes de Teste em Go

- Arquivos terminam em `_test.go`
- Funcoes de teste comecam com `Test` + nome em PascalCase
- Use `t.Run()` para sub-tests
- Use `testify/assert` para assercoes legiveis
-Mocks sao structs simples que implementam a interface

---

## 8. Roadmap de Aprendizado

### Nivel 1: Fundamentos (Semana 1-2)
- [ ] Sintaxe basica do Go (tipos, funcoes, structs, interfaces)
- [ ] Pacotes e modulos (`go mod init`, `go get`)
- [ ] Tratamento de erros (Go nao tem exceptions!)
- [ ] `net/http` basico (handler, mux, ListenAndServe)

### Nivel 2: API Basica (Semana 3-4)
- [ ] REST com `net/http` ou `chi`
- [ ] JSON encoding/decoding
- [ ] Path parameters e query parameters
- [ ] Organizacao em pacotes (service, handler, model)

### Nivel 3: Banco de Dados (Semana 5-6)
- [ ] `database/sql` com driver Postgres (`pgx` ou `pq`)
- [ ] SQL queries com `sqlx` ou `sqlc`
- [ ] Migrations com `golang-migrate`
- [ ] Transacoes

### Nivel 4: Arquitetura (Semana 7-8)
- [ ] Interfaces em Go (implicitas, duck typing)
- [ ] Dependency Injection via construtores
- [ ] Repository Pattern
- [ ] Separacao em camadas (handler -> service -> repository)

### Nivel 5: DDD e CQRS (Semana 9-10)
- [ ] Entidades com comportamento e validacao
- [ ] Value Objects e Validated Types
- [ ] CQRS (Commands e Queries separados)
- [ ] Idempotency Pattern
- [ ] Mappers / DTOs

### Nivel 6: Testes (Semana 11-12)
- [ ] Testes unitarios com `testing` + `testify`
- [ ] Table-driven tests
- [ ] Mocks implementando interfaces
- [ ] Testes de integracao com banco real
- [ ] Cobertura de testes (`go test -cover`)

### Nivel 7: Producao (Semana 13+)
- [ ] Logging estruturado (`slog` ou `zerolog`)
- [ ] Graceful shutdown
- [ ] Health checks
- [ ] Docker + docker-compose
- [ ] Middleware (auth, logging, cors, recovery)
- [ ] Configuracao com env vars (`envconfig`)
- [ ] Metrics e observabilidade

---

## 9. Checklist: Projeto Go Profissional

### Estrutura
- [ ] `cmd/` para entry points
- [ ] `internal/` para logica protegida
- [ ] `pkg/` para codigo publico reutilizavel (opcional)
- [ ] `go.mod` e `go.sum` versionados

### Dominio
- [ ] Entidades com validacao interna
- [ ] Regras de negocio nas entidades, nao nos services
- [ ] Interfaces de repositorio definidas no dominio
- [ ] Tipos validados para garantia em compile-time

### Aplicacao
- [ ] Services orquestram casos de uso
- [ ] Commands para escrita, Queries para leitura
- [ ] Mappers para converter entre camadas
- [ ] Interfaces de service para testabilidade

### Infraestrutura
- [ ] Repositorios concretos implementam interfaces do dominio
- [ ] sqlc para queries type-safe (recomendado)
- [ ] Migrations versionadas

### Interface (HTTP)
- [ ] Controllers finos - delegam para services
- [ ] DTOs separados de entidades internas
- [ ] Error handling consistente com HTTP status codes
- [ ] Validacao de input no controller

### Testes
- [ ] Testes de entidade (dominio puro)
- [ ] Testes de service com mocks
- [ ] Testes de controller com mock service
- [ ] Table-driven tests para casos multiplas

### DevOps
- [ ] Dockerfile multi-stage
- [ ] docker-compose para dependencias (postgres, etc)
- [ ] CI/CD pipeline
- [ ] Linting com `golangci-lint`

---

## 10. Exemplos de Codigo por Camada

### Camada 1: Domain Entity

```go
// internal/domain/entities/product.go
package entities

import (
    "errors"
    "github.com/google/uuid"
    "time"
)

type Product struct {
    Id        uuid.UUID
    CreatedAt time.Time
    UpdatedAt time.Time
    Name      string
    Price     float64
}

func (p *Product) validate() error {
    if p.Name == "" {
        return errors.New("product name cannot be empty")
    }
    if p.Price <= 0 {
        return errors.New("product price must be greater than zero")
    }
    return nil
}

func NewProduct(name string, price float64) *Product {
    now := time.Now()
    return &Product{
        Id:        uuid.NewV7(),
        CreatedAt: now,
        UpdatedAt: now,
        Name:      name,
        Price:     price,
    }
}
```

### Camada 2: Repository Interface (Domain)

```go
// internal/domain/repositories/product_repository.go
package repositories

import "github.com/google/uuid"

type ProductRepository interface {
    GetAll() ([]Product, error)
    GetById(id uuid.UUID) (*Product, error)
    Create(product Product) error
    Update(product Product) error
    Delete(id uuid.UUID) error
}
```

### Camada 3: Application Service

```go
// internal/application/services/product_service.go
package services

type ProductService struct {
    productRepo repositories.ProductRepository
}

func NewProductService(repo repositories.ProductRepository) ProductService {
    return ProductService{productRepo: repo}
}

func (s *ProductService) CreateProduct(cmd command.CreateProductCommand) error {
    product := entities.NewProduct(cmd.Name, cmd.Price)
    if err := product.validate(); err != nil {
        return err
    }
    return s.productRepo.Create(*product)
}
```

### Camada 4: Infrastructure (Postgres)

```go
// internal/infrastructure/db/postgres/product_repository.go
package postgres

import (
    "database/sql"
    "github.com/google/uuid"
)

type productRepository struct {
    db *sql.DB
}

func NewProductRepository(db *sql.DB) *productRepository {
    return &productRepository{db: db}
}

func (r *productRepository) Create(product entities.Product) error {
    _, err := r.db.Exec(
        "INSERT INTO products (id, name, price, created_at, updated_at) VALUES ($1, $2, $3, $4, $5)",
        product.Id, product.Name, product.Price, product.CreatedAt, product.UpdatedAt,
    )
    return err
}
```

### Camada 5: REST Controller

```go
// internal/interface/api/rest/product_controller.go
package rest

import (
    "encoding/json"
    "net/http"
)

type ProductController struct {
    productService interfaces.ProductService
}

func NewProductController(service interfaces.ProductService) *ProductController {
    return &ProductController{productService: service}
}

func (c *ProductController) CreateProduct(w http.ResponseWriter, r *http.Request) {
    var req dto.CreateProductRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "invalid request", http.StatusBadRequest)
        return
    }

    cmd := command.CreateProductCommand{
        Name:  req.Name,
        Price: req.Price,
    }

    result, err := c.productService.CreateProduct(&cmd)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusCreated)
    json.NewEncoder(w).Encode(result)
}
```

---

## Resumo dos Principios Extraidos do go-ddd

1. **Dominio no centro**: Entidades com validacao, sem dependencias externas
2. **Interfaces definidas pelo consumidor**: Repositorios definidos no domain, implementados na infra
3. **CQRS**: Commands e Queries separados com structs dedicadas
4. **Validated Types**: Tipos que garantem validacao em compile-time
5. **Idempotencia**: Protecao contra processamento duplicado
6. **Mapper/DTO**: Nunca expor entidades internas na API
7. **sqlc > ORM**: SQL type-safe sem magic
8. **Testes por camada**: Cada camada testavel independentemente com mocks
9. **Constructor Injection**: Dependencias recebidas via construtores
10. **Composition Root**: Tudo conectado no main.go

---

> Este documento foi gerado a partir da analise do repositorio [sklinkert/go-ddd](https://github.com/sklinkert/go-ddd).
> Para aprofundar, clone o repositorio e estude os testes - eles sao a melhor documentacao.
