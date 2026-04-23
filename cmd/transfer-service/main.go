package main

import (
	"github.com/felipersas/payflow/internal/transfer/application/services"
	transferHttp "github.com/felipersas/payflow/internal/transfer/interfaces/http"
	transferMessaging "github.com/felipersas/payflow/internal/transfer/interfaces/messaging"
	"github.com/felipersas/payflow/internal/transfer/infrastructure/postgres"
	"github.com/felipersas/payflow/pkg/app"
	"github.com/felipersas/payflow/pkg/config"
	"github.com/felipersas/payflow/pkg/middleware"
	"github.com/go-chi/chi/v5"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}

	var transferSvc *services.TransferService

	app.New("transfer-service", cfg).
		WithTracer().
		WithDatabase(postgres.Migrations, "migrations").
		WithRabbitMQ().
		RegisterRoutes(func(r chi.Router, d *app.Deps) {
			repo := postgres.NewTransferRepository(d.Pool)
			transferSvc = services.NewTransferService(repo, d.Publisher, d.Logger)
			handler := transferHttp.NewTransferHandler(transferSvc)

			r.Route("/transfers", func(r chi.Router) {
				r.Use(middleware.Auth(d.JWTSecret))
				handler.Routes(r)
			})
		}).
		RegisterConsumers(func(d *app.Deps) error {
			consumer := transferMessaging.NewTransferConsumer(transferSvc, d.Consumer, d.Logger)
			return consumer.Start(d.Ctx)
		}).
		MustRun()
}
