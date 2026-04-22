package health

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	amqp "github.com/rabbitmq/amqp091-go"
)

func DBCheck(pool *pgxpool.Pool) CheckFunc {
	return func() CheckResult {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		if err := pool.Ping(ctx); err != nil {
			return CheckResult{
				Name:   "database",
				Status: StatusUnhealthy,
				Error:  err.Error(),
			}
		}
		return CheckResult{
			Name:   "database",
			Status: StatusHealthy,
		}
	}
}

func RabbitMQCheck(conn *amqp.Connection) CheckFunc {
	return func() CheckResult {
		if conn == nil || conn.IsClosed() {
			return CheckResult{
				Name:   "rabbitmq",
				Status: StatusUnhealthy,
				Error:  "connection is nil or closed",
			}
		}
		return CheckResult{
			Name:   "rabbitmq",
			Status: StatusHealthy,
		}
	}
}
