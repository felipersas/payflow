package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/felipersas/payflow/internal/account/application/services"
	accHttp "github.com/felipersas/payflow/internal/account/interfaces/http"
	accMessaging "github.com/felipersas/payflow/internal/account/interfaces/messaging"
	"github.com/felipersas/payflow/internal/account/infrastructure/postgres"
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

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	// 1. Config
	cfg, err := config.Load()
	if err != nil {
		logger.Error("loading config", "error", err)
		os.Exit(1)
	}

	// 2. Telemetry (OTel tracing)
	shutdownTracer, err := telemetry.InitTracer(context.Background(), cfg.OTLPEndpoint, cfg.ServiceName, logger)
	if err != nil {
		logger.Warn("tracer init failed, continuing without tracing", "error", err)
	}

	// 3. PostgreSQL
	ctx := context.Background()
	poolConfig, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		logger.Error("parsing database url", "error", err)
		os.Exit(1)
	}
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		logger.Error("connecting to database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		logger.Error("pinging database", "error", err)
		os.Exit(1)
	}
	logger.Info("database connected")

	// 4. RabbitMQ
	rabbitConn, err := connectRabbitMQ(cfg.RabbitMQURL, logger)
	if err != nil {
		logger.Error("connecting to rabbitmq", "error", err)
		os.Exit(1)
	}
	defer rabbitConn.Close()

	publisher, err := messaging.NewPublisher(rabbitConn, logger)
	if err != nil {
		logger.Error("creating publisher", "error", err)
		os.Exit(1)
	}
	defer publisher.Close()

	resilientPub := messaging.NewResilientPublisher(publisher, logger)

	consumer, err := messaging.NewConsumer(rabbitConn, logger)
	if err != nil {
		logger.Error("creating consumer", "error", err)
		os.Exit(1)
	}
	defer consumer.Close()

	// 5. Composition Root (DDD: monta o grafo de dependências)
	accountRepo := postgres.NewAccountRepository(pool)
	userRepo := postgres.NewUserRepository(pool)

	if err := migrate.Run(cfg.DatabaseURL, postgres.Migrations, "migrations"); err != nil {
		logger.Error("running migrations", "error", err)
		os.Exit(1)
	}
	logger.Info("migrations applied")

	accountService := services.NewAccountService(accountRepo, resilientPub, logger)
	authService := services.NewAuthService(userRepo, cfg.JWTSecret, logger)
	accountHandler := accHttp.NewAccountHandler(accountService)
	authHandler := accHttp.NewAuthHandler(authService)
	accountConsumer := accMessaging.NewAccountConsumer(accountService, consumer, resilientPub, logger)

	// 6. Health checks
	healthChecker := health.NewChecker()
	healthChecker.AddCheck(health.DBCheck(pool))
	healthChecker.AddCheck(health.RabbitMQCheck(rabbitConn))

	// 7. HTTP Server
	r := chi.NewRouter()
	r.Use(otelhttp.NewMiddleware(cfg.ServiceName))
	r.Use(middleware.CorrelationID)
	r.Use(middleware.Logging(logger))
	r.Use(middleware.Recovery(logger))
	r.Use(middleware.Metrics)

	// Rotas públicas (sem auth)
	r.Route("/auth", authHandler.Routes)

	// Rotas protegidas (com auth)
	r.Route("/accounts", func(r chi.Router) {
		r.Use(middleware.Auth(cfg.JWTSecret))
		accountHandler.Routes(r)
	})

	r.Get("/health", healthChecker.Handler())
	r.Handle("/metrics", promhttp.Handler())

	// 8. Start consumers
	if err := accountConsumer.Start(ctx); err != nil {
		logger.Error("starting consumer", "error", err)
		os.Exit(1)
	}

	// 9. HTTP Server com graceful shutdown
	srv := &http.Server{
		Addr:         ":" + cfg.ServicePort,
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGTERM)

	go func() {
		logger.Info("starting server", "port", cfg.ServicePort)
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
