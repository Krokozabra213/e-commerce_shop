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

	grpcclient "github.com/Krokozabra213/e-commerce_shop/infra/clients/grpc"
	"github.com/Krokozabra213/e-commerce_shop/infra/httpx"
	infrakafka "github.com/Krokozabra213/e-commerce_shop/infra/kafka"
	"github.com/Krokozabra213/e-commerce_shop/infra/logger"
	inframiddleware "github.com/Krokozabra213/e-commerce_shop/infra/middleware"
	"github.com/Krokozabra213/e-commerce_shop/infra/postgres"
	tx_manager "github.com/Krokozabra213/e-commerce_shop/infra/tx-manager"
	"github.com/Krokozabra213/e-commerce_shop/infra/worker"
	"github.com/Krokozabra213/e-commerce_shop/services/order-service/internal/config"
	"github.com/Krokozabra213/e-commerce_shop/services/order-service/internal/domain"
	httphandler "github.com/Krokozabra213/e-commerce_shop/services/order-service/internal/handler/http"
	"github.com/Krokozabra213/e-commerce_shop/services/order-service/internal/health"
	kafkahandler "github.com/Krokozabra213/e-commerce_shop/services/order-service/internal/kafka/handler"
	kafkapublisher "github.com/Krokozabra213/e-commerce_shop/services/order-service/internal/kafka/publisher"
	inboxRepository "github.com/Krokozabra213/e-commerce_shop/services/order-service/internal/repository/postgres/inbox"
	orderRepository "github.com/Krokozabra213/e-commerce_shop/services/order-service/internal/repository/postgres/order"
	outboxRepository "github.com/Krokozabra213/e-commerce_shop/services/order-service/internal/repository/postgres/outbox"
	sagaRepository "github.com/Krokozabra213/e-commerce_shop/services/order-service/internal/repository/postgres/saga"
	orderService "github.com/Krokozabra213/e-commerce_shop/services/order-service/internal/service/order"
	outboxService "github.com/Krokozabra213/e-commerce_shop/services/order-service/internal/service/outbox"
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

	log := logger.Init(&cfg.Logger)

	pool, err := postgres.NewPostgresClient(&cfg.Postgres)
	if err != nil {
		return err
	}
	defer pool.Close()

	txManager := tx_manager.NewPgTxManager(pool)

	inboxRepo := inboxRepository.NewPostgresInboxRepository(pool)
	orderRepo := orderRepository.NewPostgresOrderRepository(pool)
	outboxRepo := outboxRepository.NewPostgresOutboxRepository(pool)
	sagaRepo := sagaRepository.NewPostgresSagaRepository(pool)

	productClient, err := grpcclient.NewProductClient(ctx, cfg.ProductGRPCClient.Addr)
	if err != nil {
		return err
	}
	defer func() {
		_ = productClient.Close()
	}()

	orderFSM := domain.NewOrderStateMachine()
	sagaStepFSM := domain.NewSagaStepStateMachine()
	sagaStatusFSM := domain.NewSagaStatusStateMachine()

	orderSvc := orderService.NewOrderService(txManager, orderRepo, sagaRepo, outboxRepo, inboxRepo, productClient, orderFSM, sagaStepFSM, sagaStatusFSM)

	orderHandler := httphandler.NewOrderHandler(orderSvc)

	schemaRegistry, err := infrakafka.NewSchemaRegistryClient(cfg.SchemaRegistry.URL)
	if err != nil {
		return fmt.Errorf("create schema registry client: %w", err)
	}
	defer schemaRegistry.Close()

	inventoryReservedHandler := kafkahandler.NewInventoryReservedHandler(orderSvc, schemaRegistry, log)
	inventoryReservFailedHandler := kafkahandler.NewInventoryReservationFailedHandler(orderSvc, schemaRegistry, log)

	inventoryReservedDLQ, err := infrakafka.NewDLQProducer(cfg.InventoryCreatedConsumer.Brokers, cfg.InventoryCreatedConsumer.DLQTopic)
	if err != nil {
		return err
	}
	inventoryReservFailedDLQ, err := infrakafka.NewDLQProducer(cfg.InventoryCancelledConsumer.Brokers, cfg.InventoryCancelledConsumer.DLQTopic)
	if err != nil {
		return err
	}

	inventoryReservedConsumer, err := infrakafka.NewKGOConsumer(cfg.InventoryCreatedConsumer, inventoryReservedHandler.Handle, log, inventoryReservedDLQ)
	if err != nil {
		return err
	}

	inventoryReservFailedConsumer, err := infrakafka.NewKGOConsumer(cfg.InventoryCancelledConsumer, inventoryReservFailedHandler.Handle, log, inventoryReservFailedDLQ)
	if err != nil {
		return err
	}

	kafkaProducer, err := infrakafka.NewKGOProducer(cfg.KafkaProducer, log)
	if err != nil {
		return err
	}
	defer kafkaProducer.Close()

	eventPublisher := kafkapublisher.NewOrderEventPublisher(kafkaProducer, schemaRegistry, log)
	orderCreatedOutboxSvc := outboxService.New(outboxRepo, eventPublisher, log, cfg.OrderCreatedOutbox)
	orderCancellInventorySvc := outboxService.New(outboxRepo, eventPublisher, log, cfg.OrderCancellInventoryOutbox)

	orderCreatedWorker := worker.NewWorker(log, orderCreatedOutboxSvc, cfg.OrderCreatedOutbox.PollInterval, cfg.OrderCreatedOutbox.ErrorBackoff, infrakafka.TopicOrderCreated)
	orderCancellInventoryWorker := worker.NewWorker(log, orderCancellInventorySvc, cfg.OrderCancellInventoryOutbox.PollInterval, cfg.OrderCancellInventoryOutbox.ErrorBackoff, infrakafka.TopicOrderCancelInventory)

	go func() { orderCreatedWorker.Run(context.Background()) }()
	go func() { orderCancellInventoryWorker.Run(context.Background()) }()

	errorHandler := inframiddleware.NewErrorHandler(log)
	httpServer := httpx.NewFiberServer(cfg.HTTP, log, errorHandler)
	orderHandler.SetupOrderRoutes(httpServer.App)

	errCh := make(chan error, 3)

	go func() {
		log.Info("http server started", "address", cfg.HTTP.HTTPAddress())
		if err := httpServer.Run(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	go func() {
		log.Info("kafka consumer started", "topic", cfg.InventoryCreatedConsumer.Topic)
		if err := inventoryReservedConsumer.Run(ctx); err != nil {
			errCh <- fmt.Errorf("kafka consumer error: %w", err)
		}
	}()

	go func() {
		log.Info("kafka consumer started", "topic", cfg.InventoryCancelledConsumer.Topic)
		if err := inventoryReservFailedConsumer.Run(ctx); err != nil {
			errCh <- fmt.Errorf("kafka consumer error: %w", err)
		}
	}()

	healthHandler := health.NewHealthHandler(pool, inventoryReservedDLQ, inventoryReservFailedDLQ, inventoryReservedConsumer, inventoryReservFailedConsumer, schemaRegistry)
	healthHandler.RegisterRoutes(httpServer.App)

	select {
	case <-ctx.Done():
		log.Info("received shutdown signal")
	case err := <-errCh:
		log.Error("server error", "error", err)
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = httpServer.Shutdown(shutdownCtx)
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Error("server error", "error", err)
	}

	log.Info("application stopped")
	return nil
}
