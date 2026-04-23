package main

import (
	"github.com/felipersas/payflow/internal/user/application/services"
	userHttp "github.com/felipersas/payflow/internal/user/interfaces/http"
	"github.com/felipersas/payflow/internal/user/infrastructure/postgres"
	"github.com/felipersas/payflow/pkg/app"
	"github.com/felipersas/payflow/pkg/config"
	"github.com/go-chi/chi/v5"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}

	app.New("user-service", cfg).
		WithTracer().
		WithDatabase(postgres.Migrations, "migrations").
		RegisterRoutes(func(r chi.Router, d *app.Deps) {
			userRepo := postgres.NewUserRepository(d.Pool)
			authService := services.NewAuthService(userRepo, d.JWTSecret, d.Logger)
			authHandler := userHttp.NewAuthHandler(authService)

			r.Route("/auth", authHandler.Routes)
		}).
		MustRun()
}
