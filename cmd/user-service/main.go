package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/felipersas/payflow/internal/user/application/services"
	userHttp "github.com/felipersas/payflow/internal/user/interfaces/http"
	"github.com/felipersas/payflow/internal/user/infrastructure/postgres"
	"github.com/felipersas/payflow/pkg/config"
	"github.com/felipersas/payflow/pkg/health"
	"github.com/felipersas/payflow/pkg/migrate"
	"github.com/felipersas/payflow/pkg/middleware"
	"github.com/felipersas/payflow/pkg/telemetry"
	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
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

	// 4. Composition Root
	userRepo := postgres.NewUserRepository(pool)

	if err := migrate.Run(cfg.DatabaseURL, postgres.Migrations, "migrations"); err != nil {
		logger.Error("running migrations", "error", err)
		os.Exit(1)
	}
	logger.Info("migrations applied")

	authService := services.NewAuthService(userRepo, cfg.JWTSecret, logger)
	authHandler := userHttp.NewAuthHandler(authService)

	// 5. Health checks
	healthChecker := health.NewChecker()
	healthChecker.AddCheck(health.DBCheck(pool))

	// 6. HTTP Server
	r := chi.NewRouter()
	r.Use(otelhttp.NewMiddleware(cfg.ServiceName))
	r.Use(middleware.CorrelationID)
	r.Use(middleware.Logging(logger))
	r.Use(middleware.Recovery(logger))
	r.Use(middleware.Metrics)

	// Rotas públicas (auth não requer token)
	r.Route("/auth", authHandler.Routes)

	r.Get("/health", healthChecker.Handler())
	r.Handle("/metrics", promhttp.Handler())

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

	if shutdownTracer != nil {
		if err := shutdownTracer(shutdownCtx); err != nil {
			logger.Error("tracer shutdown error", "error", err)
		}
	}

	logger.Info("server stopped")
}
