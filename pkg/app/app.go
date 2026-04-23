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
// Ordem: Validate → Logger → Tracer → DB → RabbitMQ → Router → Consumers → Server.
func (a *App) Run() error {
	if err := a.validate(); err != nil {
		return err
	}

	// Logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	// Context with cancel for consumer shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Cleanup slice for proper resource management in error paths
	var cleanups []func()
	defer func() {
		for i := len(cleanups) - 1; i >= 0; i-- {
			cleanups[i]()
		}
	}()

	// Tracer
	var shutdownTracer func(context.Context) error
	if a.useTracer {
		var err error
		shutdownTracer, err = telemetry.InitTracer(ctx, a.cfg.OTLPEndpoint, a.name, logger)
		if err != nil {
			logger.Warn("tracer init failed, continuing without tracing", "error", err)
		}
	}

	// Database
	pool, dbCleanup, err := a.initDB(ctx, logger)
	if err != nil {
		return err
	}
	if dbCleanup != nil {
		cleanups = append(cleanups, dbCleanup)
	}

	// RabbitMQ
	rabbitConn, publisher, consumer, mqCleanup, err := a.initRabbitMQ(ctx, logger)
	if err != nil {
		return err
	}
	if mqCleanup != nil {
		cleanups = append(cleanups, mqCleanup)
	}

	// Deps
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

	// Health checks
	healthChecker := health.NewChecker()
	if pool != nil {
		healthChecker.AddCheck(health.DBCheck(pool))
	}
	if rabbitConn != nil {
		healthChecker.AddCheck(health.RabbitMQCheck(rabbitConn))
	}

	// Router
	r := a.initRouter(deps, healthChecker)

	// Consumers
	if a.onConsumers != nil {
		if err := a.onConsumers(deps); err != nil {
			return fmt.Errorf("starting consumers: %w", err)
		}
	}

	// HTTP Server + graceful shutdown
	srv := &http.Server{
		Addr:         ":" + a.cfg.ServicePort,
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	serverErr := make(chan error, 1)
	go func() {
		logger.Info("starting server", "port", a.cfg.ServicePort)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErr <- err
		}
	}()

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-serverErr:
		return fmt.Errorf("server error: %w", err)
	case sig := <-done:
		logger.Info("shutting down...", "signal", sig)
	}

	// Signal consumers to stop
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

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

// validate checks that the configuration is consistent.
func (a *App) validate() error {
	if a.onConsumers != nil && !a.useRabbit {
		return fmt.Errorf("consumers registered but RabbitMQ not enabled: call WithRabbitMQ()")
	}
	return nil
}

// initDB initializes the database connection and runs migrations.
func (a *App) initDB(ctx context.Context, logger *slog.Logger) (*pgxpool.Pool, func(), error) {
	if !a.useDB {
		return nil, func() {}, nil
	}

	poolConfig, err := pgxpool.ParseConfig(a.cfg.DatabaseURL)
	if err != nil {
		return nil, nil, fmt.Errorf("parsing database url: %w", err)
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, nil, fmt.Errorf("connecting to database: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, nil, fmt.Errorf("pinging database: %w", err)
	}
	logger.Info("database connected")

	if err := migrate.Run(a.cfg.DatabaseURL, a.migrations, a.migrationsDir); err != nil {
		pool.Close()
		return nil, nil, fmt.Errorf("running migrations: %w", err)
	}
	logger.Info("migrations applied")

	cleanup := func() { pool.Close() }
	return pool, cleanup, nil
}

// initRabbitMQ initializes RabbitMQ connection, publisher, and consumer.
func (a *App) initRabbitMQ(ctx context.Context, logger *slog.Logger) (*amqp.Connection, messaging.MessagePublisher, *messaging.Consumer, func(), error) {
	if !a.useRabbit {
		return nil, nil, nil, func() {}, nil
	}

	rabbitConn, err := connectRabbitMQ(ctx, a.cfg.RabbitMQURL, logger)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("connecting to rabbitmq: %w", err)
	}

	pub, err := messaging.NewPublisher(rabbitConn, logger)
	if err != nil {
		rabbitConn.Close()
		return nil, nil, nil, nil, fmt.Errorf("creating publisher: %w", err)
	}

	publisher := messaging.NewResilientPublisher(pub, logger)

	consumer, err := messaging.NewConsumer(rabbitConn, logger)
	if err != nil {
		pub.Close()
		rabbitConn.Close()
		return nil, nil, nil, nil, fmt.Errorf("creating consumer: %w", err)
	}

	cleanup := func() {
		consumer.Close()
		pub.Close()
		rabbitConn.Close()
	}

	return rabbitConn, publisher, consumer, cleanup, nil
}

// initRouter initializes the HTTP router with middleware and routes.
func (a *App) initRouter(deps *Deps, healthChecker *health.Checker) *chi.Mux {
	r := chi.NewRouter()
	r.Use(otelhttp.NewMiddleware(a.name))
	r.Use(middleware.CorrelationID)
	r.Use(middleware.Logging(deps.Logger))
	r.Use(middleware.Recovery(deps.Logger))
	r.Use(middleware.Metrics)

	if a.onRoutes != nil {
		a.onRoutes(r, deps)
	}

	r.Get("/health", healthChecker.Handler())
	r.Handle("/metrics", promhttp.Handler())

	return r
}

// connectRabbitMQ attempts to connect to RabbitMQ with retries.
func connectRabbitMQ(ctx context.Context, url string, logger *slog.Logger) (*amqp.Connection, error) {
	var conn *amqp.Connection
	var err error

	for i := 0; i < 10; i++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

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
