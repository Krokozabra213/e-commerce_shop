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
	infrakafka "github.com/Krokozabra213/e-commerce_shop/infra/kafka"
	"github.com/Krokozabra213/e-commerce_shop/infra/logger"
	inframiddleware "github.com/Krokozabra213/e-commerce_shop/infra/middleware"
	"github.com/Krokozabra213/e-commerce_shop/infra/postgres"
	"github.com/Krokozabra213/e-commerce_shop/infra/telemetry"
	tx_manager "github.com/Krokozabra213/e-commerce_shop/infra/tx-manager"
	"github.com/Krokozabra213/e-commerce_shop/services/user-service/internal/config"
	"github.com/Krokozabra213/e-commerce_shop/services/user-service/internal/handler"
	"github.com/Krokozabra213/e-commerce_shop/services/user-service/internal/health"
	"github.com/Krokozabra213/e-commerce_shop/services/user-service/internal/kafka"
	userRepo "github.com/Krokozabra213/e-commerce_shop/services/user-service/internal/repository/postgres/user"
	"github.com/Krokozabra213/e-commerce_shop/services/user-service/internal/service"
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

	txManager := tx_manager.NewPgTxManager(pool)
	userRepository := userRepo.NewPostgresUserRepository(pool)
	service := service.NewService(userRepository, txManager)
	handler := handler.NewHandler(service)

	schemaRegistry, err := infrakafka.NewSchemaRegistryClient(cfg.SchemaRegistry.URL)
	if err != nil {
		return fmt.Errorf("create schema registry client: %w", err)
	}
	defer schemaRegistry.Close()

	userCreatedHandler := kafka.NewUserCreatedHandler(service, schemaRegistry, log)
	dlqProducer, err := kafka.NewDLQProducer(cfg.KafkaConsumer.Brokers, cfg.KafkaConsumer.DLQTopic)
	if err != nil {
		return err
	}
	defer dlqProducer.Close()
	kafkaConsumer, err := kafka.NewKGOConsumer(cfg.KafkaConsumer, userCreatedHandler.Handle, log, dlqProducer)
	if err != nil {
		return err
	}
	defer kafkaConsumer.Close()

	loggerRequestIDMiddleware := inframiddleware.RequestLogger(log)
	errorHandler := inframiddleware.NewErrorHandler(log)
	server := httpx.NewFiberServer(cfg.HTTP, log, errorHandler, loggerRequestIDMiddleware)

	handler.RegisterRoutes(server.App, userRepository)

	healthHandler := health.NewHandler(pool, dlqProducer, kafkaConsumer, schemaRegistry)
	healthHandler.RegisterRoutes(server.App)

	errCh := make(chan error, 2)
	go func() {
		log.Info("server started", "address", cfg.HTTP.HTTPAddress())
		if err := server.Run(); !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	go func() {
		log.Info("kafka consumer started", "topic", cfg.KafkaConsumer.Topic)
		if err := kafkaConsumer.Run(ctx); err != nil {
			errCh <- fmt.Errorf("kafka consumer error: %w", err)
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
