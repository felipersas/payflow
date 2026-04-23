package main

import (
	"github.com/felipersas/payflow/internal/account/application/services"
	accHttp "github.com/felipersas/payflow/internal/account/interfaces/http"
	accMessaging "github.com/felipersas/payflow/internal/account/interfaces/messaging"
	"github.com/felipersas/payflow/internal/account/infrastructure/postgres"
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

	var accountSvc *services.AccountService

	app.New("account-service", cfg).
		WithTracer().
		WithDatabase(postgres.Migrations, "migrations").
		WithRabbitMQ().
		RegisterRoutes(func(r chi.Router, d *app.Deps) {
			accountRepo := postgres.NewAccountRepository(d.Pool)
			accountSvc = services.NewAccountService(accountRepo, d.Publisher, d.Logger)
			handler := accHttp.NewAccountHandler(accountSvc)

			r.Route("/accounts", func(r chi.Router) {
				r.Use(middleware.Auth(d.JWTSecret))
				handler.Routes(r)
			})
		}).
		RegisterConsumers(func(d *app.Deps) error {
			consumer := accMessaging.NewAccountConsumer(accountSvc, d.Consumer, d.Publisher, d.Logger)
			return consumer.Start(d.Ctx)
		}).
		MustRun()
}
