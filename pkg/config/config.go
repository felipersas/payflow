package config

import (
	"fmt"

	"github.com/spf13/viper"
)

type Config struct {
	DatabaseURL  string
	RabbitMQURL  string
	RedisURL     string
	ServicePort  string
	JWTSecret    string
	ServiceName  string
}

func Load() (*Config, error) {
	viper.SetDefault("DB_HOST", "localhost")
	viper.SetDefault("DB_PORT", "5432")
	viper.SetDefault("DB_USER", "payflow")
	viper.SetDefault("DB_PASSWORD", "payflow123")
	viper.SetDefault("DB_NAME", "payflow")
	viper.SetDefault("RABBITMQ_HOST", "localhost")
	viper.SetDefault("RABBITMQ_PORT", "5672")
	viper.SetDefault("RABBITMQ_USER", "payflow")
	viper.SetDefault("RABBITMQ_PASSWORD", "payflow123")
	viper.SetDefault("REDIS_HOST", "localhost")
	viper.SetDefault("REDIS_PORT", "6379")
	viper.SetDefault("SERVICE_PORT", "8080")
	viper.SetDefault("JWT_SECRET", "dev-secret-change-in-production")
	viper.SetDefault("SERVICE_NAME", "account-service")

	viper.AutomaticEnv()

	if err := viper.BindEnv("DB_HOST", "DB_HOST"); err != nil {
		return nil, fmt.Errorf("binding DB_HOST: %w", err)
	}
	if err := viper.BindEnv("DB_PORT", "DB_PORT"); err != nil {
		return nil, fmt.Errorf("binding DB_PORT: %w", err)
	}
	if err := viper.BindEnv("DB_USER", "DB_USER"); err != nil {
		return nil, fmt.Errorf("binding DB_USER: %w", err)
	}
	if err := viper.BindEnv("DB_PASSWORD", "DB_PASSWORD"); err != nil {
		return nil, fmt.Errorf("binding DB_PASSWORD: %w", err)
	}
	if err := viper.BindEnv("DB_NAME", "DB_NAME"); err != nil {
		return nil, fmt.Errorf("binding DB_NAME: %w", err)
	}
	if err := viper.BindEnv("RABBITMQ_HOST", "RABBITMQ_HOST"); err != nil {
		return nil, fmt.Errorf("binding RABBITMQ_HOST: %w", err)
	}
	if err := viper.BindEnv("RABBITMQ_PORT", "RABBITMQ_PORT"); err != nil {
		return nil, fmt.Errorf("binding RABBITMQ_PORT: %w", err)
	}
	if err := viper.BindEnv("RABBITMQ_USER", "RABBITMQ_USER"); err != nil {
		return nil, fmt.Errorf("binding RABBITMQ_USER: %w", err)
	}
	if err := viper.BindEnv("RABBITMQ_PASSWORD", "RABBITMQ_PASSWORD"); err != nil {
		return nil, fmt.Errorf("binding RABBITMQ_PASSWORD: %w", err)
	}
	if err := viper.BindEnv("REDIS_HOST", "REDIS_HOST"); err != nil {
		return nil, fmt.Errorf("binding REDIS_HOST: %w", err)
	}
	if err := viper.BindEnv("REDIS_PORT", "REDIS_PORT"); err != nil {
		return nil, fmt.Errorf("binding REDIS_PORT: %w", err)
	}
	if err := viper.BindEnv("SERVICE_PORT", "SERVICE_PORT"); err != nil {
		return nil, fmt.Errorf("binding SERVICE_PORT: %w", err)
	}
	if err := viper.BindEnv("JWT_SECRET", "JWT_SECRET"); err != nil {
		return nil, fmt.Errorf("binding JWT_SECRET: %w", err)
	}
	if err := viper.BindEnv("SERVICE_NAME", "SERVICE_NAME"); err != nil {
		return nil, fmt.Errorf("binding SERVICE_NAME: %w", err)
	}

	dbURL := fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=disable",
		viper.GetString("DB_USER"),
		viper.GetString("DB_PASSWORD"),
		viper.GetString("DB_HOST"),
		viper.GetString("DB_PORT"),
		viper.GetString("DB_NAME"),
	)

	rabbitURL := fmt.Sprintf(
		"amqp://%s:%s@%s:%s/",
		viper.GetString("RABBITMQ_USER"),
		viper.GetString("RABBITMQ_PASSWORD"),
		viper.GetString("RABBITMQ_HOST"),
		viper.GetString("RABBITMQ_PORT"),
	)

	redisURL := fmt.Sprintf(
		"redis://%s:%s/0",
		viper.GetString("REDIS_HOST"),
		viper.GetString("REDIS_PORT"),
	)

	return &Config{
		DatabaseURL:  dbURL,
		RabbitMQURL:  rabbitURL,
		RedisURL:     redisURL,
		ServicePort:  viper.GetString("SERVICE_PORT"),
		JWTSecret:    viper.GetString("JWT_SECRET"),
		ServiceName:  viper.GetString("SERVICE_NAME"),
	}, nil
}
