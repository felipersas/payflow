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

	"github.com/felipersas/payflow/internal/transfer/application/services"
	transferHttp "github.com/felipersas/payflow/internal/transfer/interfaces/http"
	transferMessaging "github.com/felipersas/payflow/internal/transfer/interfaces/messaging"
	"github.com/felipersas/payflow/internal/transfer/infrastructure/postgres"
	"github.com/felipersas/payflow/pkg/config"
	"github.com/felipersas/payflow/pkg/messaging"
	"github.com/felipersas/payflow/pkg/middleware"
	"github.com/go-chi/chi/v5"
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

	// 2. PostgreSQL
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

	// 3. RabbitMQ
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

	consumer, err := messaging.NewConsumer(rabbitConn, logger)
	if err != nil {
		logger.Error("creating consumer", "error", err)
		os.Exit(1)
	}
	defer consumer.Close()

	// 4. Composition Root
	repo := postgres.NewTransferRepository(pool)
	if err := repo.InitSchema(ctx); err != nil {
		logger.Error("initializing schema", "error", err)
		os.Exit(1)
	}

	transferService := services.NewTransferService(repo, publisher, logger)
	transferHandler := transferHttp.NewTransferHandler(transferService)
	transferConsumer := transferMessaging.NewTransferConsumer(transferService, consumer, logger)

	// 5. HTTP Server
	r := chi.NewRouter()
	r.Use(middleware.CorrelationID)
	r.Use(middleware.Logging(logger))
	r.Use(middleware.Recovery(logger))

	r.Route("/transfers", transferHandler.Routes)
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	// 6. Start consumers
	if err := transferConsumer.Start(ctx); err != nil {
		logger.Error("starting consumer", "error", err)
		os.Exit(1)
	}

	// 7. HTTP Server com graceful shutdown
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
