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
	"github.com/Krokozabra213/e-commerce_shop/infra/worker"
	"github.com/Krokozabra213/e-commerce_shop/services/inventory-service/internal/config"
	grpchandler "github.com/Krokozabra213/e-commerce_shop/services/inventory-service/internal/handler/grpc"
	httphandler "github.com/Krokozabra213/e-commerce_shop/services/inventory-service/internal/handler/http"
	"github.com/Krokozabra213/e-commerce_shop/services/inventory-service/internal/health"
	"github.com/Krokozabra213/e-commerce_shop/services/inventory-service/internal/kafka"
	advisory_locker "github.com/Krokozabra213/e-commerce_shop/services/inventory-service/internal/repository/postgres/advisory-locker"
	outboxRepository "github.com/Krokozabra213/e-commerce_shop/services/inventory-service/internal/repository/postgres/outbox"
	reservationRepository "github.com/Krokozabra213/e-commerce_shop/services/inventory-service/internal/repository/postgres/reservation"
	stockRepository "github.com/Krokozabra213/e-commerce_shop/services/inventory-service/internal/repository/postgres/stock"
	grpcserver "github.com/Krokozabra213/e-commerce_shop/services/inventory-service/internal/server/grpc"
	outboxService "github.com/Krokozabra213/e-commerce_shop/services/inventory-service/internal/service/outbox"
	reservationService "github.com/Krokozabra213/e-commerce_shop/services/inventory-service/internal/service/reservation"
	stockService "github.com/Krokozabra213/e-commerce_shop/services/inventory-service/internal/service/stock"
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
	defer pool.Close()

	txManager := tx_manager.NewPgTxManager(pool)
	stockRepo := stockRepository.NewPostgresStockRepository(pool)
	outboxRepo := outboxRepository.NewPostgresOutboxRepository(pool)
	reservRepo := reservationRepository.NewPostgresReservationRepository(pool)
	advisoryLocker := advisory_locker.NewAdvisoryLocker(pool)

	stockSvc := stockService.NewStockService(stockRepo, outboxRepo, txManager)
	reservSvc := reservationService.NewReservationService(stockRepo, reservRepo, outboxRepo, advisoryLocker, txManager)

	schemaRegistry, err := infrakafka.NewSchemaRegistryClient(cfg.SchemaRegistry.URL)
	if err != nil {
		return fmt.Errorf("create schema registry client: %w", err)
	}
	defer schemaRegistry.Close()

	stockHandler := httphandler.NewStockHandler(stockSvc)
	orderCreatedHandler := kafka.NewOrderCreatedHandler(reservSvc, schemaRegistry, log)
	orderCancelledHandler := kafka.NewOrderCancelledHandler(reservSvc, log, schemaRegistry)

	orderCancelledDLQ, err := infrakafka.NewDLQProducer(cfg.OrderCancelledConsumer.Brokers, cfg.OrderCancelledConsumer.DLQTopic)
	if err != nil {
		return err
	}
	orderCreatedDLQ, err := infrakafka.NewDLQProducer(cfg.OrderCreatedConsumer.Brokers, cfg.OrderCreatedConsumer.DLQTopic)
	if err != nil {
		return err
	}

	orderCreatedConsumer, err := infrakafka.NewKGOConsumer(cfg.OrderCreatedConsumer, orderCreatedHandler.Handle, log, orderCreatedDLQ)
	if err != nil {
		return err
	}
	defer orderCreatedConsumer.Close()

	orderCancelledConsumer, err := infrakafka.NewKGOConsumer(cfg.OrderCancelledConsumer, orderCancelledHandler.Handle, log, orderCancelledDLQ)
	if err != nil {
		return err
	}
	defer orderCancelledConsumer.Close()

	kafkaProducer, err := infrakafka.NewKGOProducer(cfg.KafkaProducer, log)
	if err != nil {
		return err
	}
	defer kafkaProducer.Close()

	eventPublisher := kafka.NewEventPublisher(kafkaProducer, schemaRegistry, log)

	InvReservedoutboxSvc := outboxService.New(outboxRepo, eventPublisher, log, cfg.InventoryReservedOutbox)
	InvReservFailedoutboxSvc := outboxService.New(outboxRepo, eventPublisher, log, cfg.InventoryReservFailedOutbox)

	inventoryReservedWorker := worker.NewWorker(log, InvReservedoutboxSvc, cfg.InventoryReservedOutbox.PollInterval, cfg.InventoryReservedOutbox.ErrorBackoff, infrakafka.TopicInventoryReserved)
	InventoryReservFailedWorker := worker.NewWorker(log, InvReservFailedoutboxSvc, cfg.InventoryReservFailedOutbox.PollInterval, cfg.InventoryReservFailedOutbox.ErrorBackoff, infrakafka.TopicInventoryReservFailed)

	go func() { inventoryReservedWorker.Run(context.Background()) }()
	go func() { InventoryReservFailedWorker.Run(context.Background()) }()

	loggerRequestIDMiddleware := inframiddleware.RequestLogger(log)
	errorHandler := inframiddleware.NewErrorHandler(log)
	server := httpx.NewFiberServer(cfg.HTTP, log, errorHandler, loggerRequestIDMiddleware)
	stockHandler.SetupStockRoutes(server.App)

	grpcHandler := grpchandler.NewInventoryHandler(stockSvc)
	grpcServer := grpcserver.New(&cfg.GRPC, log, grpcHandler)

	errCh := make(chan error, 4)

	go func() {
		log.Info("grpc server started", "address", cfg.GRPC.GRPCAddress())
		if err := grpcServer.RunGRPC(); err != nil {
			errCh <- err
		}
	}()

	go func() {
		log.Info("http server started", "address", cfg.HTTP.HTTPAddress())
		if err := server.Run(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	go func() {
		log.Info("kafka consumer started", "topic", cfg.OrderCreatedConsumer.Topic)
		if err := orderCreatedConsumer.Run(ctx); err != nil {
			errCh <- fmt.Errorf("kafka consumer error: %w", err)
		}
	}()

	go func() {
		log.Info("kafka consumer started", "topic", cfg.OrderCancelledConsumer.Topic)
		if err := orderCancelledConsumer.Run(ctx); err != nil {
			errCh <- fmt.Errorf("kafka consumer error: %w", err)
		}
	}()

	healthHandler := health.NewHandler(pool, orderCancelledDLQ, orderCreatedDLQ, orderCreatedConsumer, orderCancelledConsumer, schemaRegistry)
	healthHandler.RegisterRoutes(server.App)

	select {
	case <-ctx.Done():
		log.Info("received shutdown signal")
	case err := <-errCh:
		log.Error("server error", "error", err)
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	grpcServer.Stop()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Error("http shutdown error", "error", err)
	}

	err = server.Shutdown(shutdownCtx)
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Error("server error", "error", err)
	}

	log.Info("application stopped")
	return nil
}
