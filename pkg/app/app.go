package app

import (
	"context"
	"embed"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/felipersas/payflow/pkg/config"
	"github.com/felipersas/payflow/pkg/health"
	"github.com/felipersas/payflow/pkg/messaging"
	"github.com/felipersas/payflow/pkg/migrate"
	"github.com/felipersas/payflow/pkg/middleware"
	"github.com/felipersas/payflow/pkg/telemetry"
	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Deps holds all initialized dependencies available to route and consumer registrars.
type Deps struct {
	Config     *config.Config
	Logger     *slog.Logger
	Pool       *pgxpool.Pool
	RabbitConn *amqp.Connection
	Publisher  messaging.MessagePublisher
	Consumer   *messaging.Consumer
	JWTSecret  string
	Ctx        context.Context
}

// RouteRegistrar registers HTTP routes on the chi router.
type RouteRegistrar func(r chi.Router, d *Deps)

// ConsumerRegistrar starts message consumers.
type ConsumerRegistrar func(d *Deps) error

// App is the bootstrap builder for a microservice.
// Encapsula toda a infraestrutura comum (DB, RabbitMQ, tracer, health, server)
// para que cada main.go só defina o que é único: rotas e consumers.
type App struct {
	name          string
	cfg           *config.Config
	useTracer     bool
	useDB         bool
	migrations    embed.FS
	migrationsDir string
	useRabbit     bool
	onRoutes      RouteRegistrar
	onConsumers   ConsumerRegistrar
}

// New cria um App builder com nome e config.
func New(name string, cfg *config.Config) *App {
	return &App{name: name, cfg: cfg}
}

// WithTracer habilita OpenTelemetry tracing.
func (a *App) WithTracer() *App {
	a.useTracer = true
	return a
}

// WithDatabase habilita PostgreSQL com migrations.
func (a *App) WithDatabase(migrations embed.FS, dir string) *App {
	a.useDB = true
	a.migrations = migrations
	a.migrationsDir = dir
	return a
}

// WithRabbitMQ habilita RabbitMQ (publisher + consumer).
func (a *App) WithRabbitMQ() *App {
	a.useRabbit = true
	return a
}

// RegisterRoutes registra a callback para configurar rotas HTTP.
// Recebe o chi.Router e os Deps com todas as dependências prontas.
func (a *App) RegisterRoutes(fn RouteRegistrar) *App {
	a.onRoutes = fn
	return a
}

// RegisterConsumers registra a callback para iniciar consumers RabbitMQ.
func (a *App) RegisterConsumers(fn ConsumerRegistrar) *App {
	a.onConsumers = fn
	return a
}

// MustRun executa o App e faz os.Exit(1) em caso de erro.
func (a *App) MustRun() {
	if err := a.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
		os.Exit(1)
	}
}

// Run inicializa toda a infraestrutura e inicia o HTTP server.
// Ordem: Logger → Tracer → DB → RabbitMQ → Router → Consumers → Server.
func (a *App) Run() error {
	// 1. Logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	// 2. Tracer
	ctx := context.Background()
	var shutdownTracer func(context.Context) error
	if a.useTracer {
		var err error
		shutdownTracer, err = telemetry.InitTracer(ctx, a.cfg.OTLPEndpoint, a.name, logger)
		if err != nil {
			logger.Warn("tracer init failed, continuing without tracing", "error", err)
		}
	}

	// 3. Database
	var pool *pgxpool.Pool
	if a.useDB {
		poolConfig, err := pgxpool.ParseConfig(a.cfg.DatabaseURL)
		if err != nil {
			return fmt.Errorf("parsing database url: %w", err)
		}
		var dbErr error
		pool, dbErr = pgxpool.NewWithConfig(ctx, poolConfig)
		if dbErr != nil {
			return fmt.Errorf("connecting to database: %w", dbErr)
		}
		defer pool.Close()

		if err := pool.Ping(ctx); err != nil {
			return fmt.Errorf("pinging database: %w", err)
		}
		logger.Info("database connected")

		if err := migrate.Run(a.cfg.DatabaseURL, a.migrations, a.migrationsDir); err != nil {
			return fmt.Errorf("running migrations: %w", err)
		}
		logger.Info("migrations applied")
	}

	// 4. RabbitMQ
	var rabbitConn *amqp.Connection
	var publisher messaging.MessagePublisher
	var consumer *messaging.Consumer
	if a.useRabbit {
		var err error
		rabbitConn, err = connectRabbitMQ(a.cfg.RabbitMQURL, logger)
		if err != nil {
			return fmt.Errorf("connecting to rabbitmq: %w", err)
		}
		defer rabbitConn.Close()

		pub, err := messaging.NewPublisher(rabbitConn, logger)
		if err != nil {
			return fmt.Errorf("creating publisher: %w", err)
		}
		defer pub.Close()

		publisher = messaging.NewResilientPublisher(pub, logger)

		consumer, err = messaging.NewConsumer(rabbitConn, logger)
		if err != nil {
			return fmt.Errorf("creating consumer: %w", err)
		}
		defer consumer.Close()
	}

	// 5. Deps
	deps := &Deps{
		Config:     a.cfg,
		Logger:     logger,
		Pool:       pool,
		RabbitConn: rabbitConn,
		Publisher:  publisher,
		Consumer:   consumer,
		JWTSecret:  a.cfg.JWTSecret,
		Ctx:        ctx,
	}

	// 6. Health checks
	healthChecker := health.NewChecker()
	if pool != nil {
		healthChecker.AddCheck(health.DBCheck(pool))
	}
	if rabbitConn != nil {
		healthChecker.AddCheck(health.RabbitMQCheck(rabbitConn))
	}

	// 7. Router
	r := chi.NewRouter()
	r.Use(otelhttp.NewMiddleware(a.name))
	r.Use(middleware.CorrelationID)
	r.Use(middleware.Logging(logger))
	r.Use(middleware.Recovery(logger))
	r.Use(middleware.Metrics)

	if a.onRoutes != nil {
		a.onRoutes(r, deps)
	}

	r.Get("/health", healthChecker.Handler())
	r.Handle("/metrics", promhttp.Handler())

	// 8. Consumers
	if a.onConsumers != nil {
		if err := a.onConsumers(deps); err != nil {
			return fmt.Errorf("starting consumers: %w", err)
		}
	}

	// 9. HTTP Server + graceful shutdown
	srv := &http.Server{
		Addr:         ":" + a.cfg.ServicePort,
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGTERM)

	go func() {
		logger.Info("starting server", "port", a.cfg.ServicePort)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	<-done
	logger.Info("shutting down...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("server shutdown error", "error", err)
	}

	if shutdownTracer != nil {
		if err := shutdownTracer(shutdownCtx); err != nil {
			logger.Error("tracer shutdown error", "error", err)
		}
	}

	logger.Info("server stopped")
	return nil
}

func connectRabbitMQ(url string, logger *slog.Logger) (*amqp.Connection, error) {
	var conn *amqp.Connection
	var err error

	for i := 0; i < 10; i++ {
		conn, err = amqp.Dial(url)
		if err == nil {
			logger.Info("rabbitmq connected")
			return conn, nil
		}
		logger.Warn("rabbitmq not ready, retrying...", "attempt", i+1, "error", err)
		time.Sleep(2 * time.Second)
	}
	return nil, fmt.Errorf("failed to connect to rabbitmq after 10 attempts: %w", err)
}
