package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Krokozabra213/e-commerce_shop/infra/httpx"
	infrajwt "github.com/Krokozabra213/e-commerce_shop/infra/jwt"
	infrakafka "github.com/Krokozabra213/e-commerce_shop/infra/kafka"
	"github.com/Krokozabra213/e-commerce_shop/infra/logger"
	inframiddleware "github.com/Krokozabra213/e-commerce_shop/infra/middleware"
	"github.com/Krokozabra213/e-commerce_shop/infra/postgres"
	infraredis "github.com/Krokozabra213/e-commerce_shop/infra/redis"
	"github.com/Krokozabra213/e-commerce_shop/infra/telemetry"
	"github.com/Krokozabra213/e-commerce_shop/infra/worker"
	"github.com/Krokozabra213/e-commerce_shop/services/auth-service/internal/config"
	authfeature "github.com/Krokozabra213/e-commerce_shop/services/auth-service/internal/features/auth"
	"github.com/Krokozabra213/e-commerce_shop/services/auth-service/internal/features/health"
	oauthfeature "github.com/Krokozabra213/e-commerce_shop/services/auth-service/internal/features/oauth"
	outboxRepo "github.com/Krokozabra213/e-commerce_shop/services/auth-service/internal/features/outbox/repository/postgres"
	outboxservice "github.com/Krokozabra213/e-commerce_shop/services/auth-service/internal/features/outbox/service"
	"github.com/Krokozabra213/e-commerce_shop/services/auth-service/internal/kafka"
)

func main() {
	if err := run(); err != nil {
		slog.Error("application failed", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Init()
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	openTelemetry, err := telemetry.Setup(ctx, cfg.Telemetry, cfg.Logger.Level)
	if err != nil {
		return err
	}
	defer func() {
		_ = openTelemetry.Shutdown(ctx)
	}()

	log := logger.Init(&cfg.Logger, openTelemetry.Handler)

	pool, err := postgres.NewPostgresClient(&cfg.Postgres)
	if err != nil {
		return err
	}

	redis, err := infraredis.NewClient(cfg.Redis, log)
	if err != nil {
		return err
	}
	defer func() {
		_ = redis.Close()
	}()

	privateKey, err := infrajwt.LoadRSAPrivateKey(cfg.AuthJWT.PrivateKeyPath)
	if err != nil {
		return err
	}

	authModule := authfeature.New(&authfeature.Dependencies{
		PGXPool:       pool,
		Config:        cfg,
		RSAPrivateKey: privateKey,
	})

	oauthModule := oauthfeature.New(&oauthfeature.Dependencies{
		PGXPool:       pool,
		RedisClient:   redis,
		Config:        cfg,
		RSAPrivateKey: privateKey,
	})

	kafkaProducer, err := infrakafka.NewKGOProducer(cfg.KafkaProducer, log)
	if err != nil {
		return err
	}
	defer kafkaProducer.Close()

	schemaRegistry, err := infrakafka.NewSchemaRegistryClient(cfg.SchemaRegistry.URL)
	if err != nil {
		return fmt.Errorf("create schema registry client: %w", err)
	}
	defer schemaRegistry.Close()

	eventPublisher := kafka.NewEventPublisher(kafkaProducer, schemaRegistry, log)

	outboxRepository := outboxRepo.NewPostgresOutboxRepository(pool)
	outboxService := outboxservice.New(outboxRepository, eventPublisher, log, cfg.UserCreatedOutbox)
	batchWorker := worker.NewWorker(log, outboxService, cfg.UserCreatedOutbox.PollInterval, cfg.UserCreatedOutbox.ErrorBackoff, infrakafka.TopicUserCreated)
	go func() { batchWorker.Run(context.Background()) }()

	loggerRequestIDMiddleware := inframiddleware.RequestLogger(log)
	errorHandler := inframiddleware.NewErrorHandler(log)
	server := httpx.NewFiberServer(cfg.HTTP, log, errorHandler, loggerRequestIDMiddleware)

	authModule.HTTPv1()(server.App)
	oauthModule.HTTPv1()(server.App)

	healthHandler := health.NewHandler(pool, kafkaProducer, schemaRegistry, redis)
	healthHandler.RegisterRoutes(server.App)

	errCh := make(chan error, 1)
	go func() {
		log.Info("server started", "address", cfg.HTTP.HTTPAddress())
		if err := server.Run(); !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case sig := <-ctx.Done():
		log.Info("received shutdown signal", "signal", sig)
	case err := <-errCh:
		log.Error("server error", "error", err)
		return err
	}

	log.Info("shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Error("server shutdown error", "error", err)
	}

	pool.Close()

	log.Info("application soft stopped")

	return nil
}
