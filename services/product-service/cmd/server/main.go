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
	"github.com/Krokozabra213/e-commerce_shop/infra/logger"
	inframiddleware "github.com/Krokozabra213/e-commerce_shop/infra/middleware"
	inframongo "github.com/Krokozabra213/e-commerce_shop/infra/mongo"
	"github.com/Krokozabra213/e-commerce_shop/services/product-service/internal/config"
	httphandler "github.com/Krokozabra213/e-commerce_shop/services/product-service/internal/handler/http"
	"github.com/Krokozabra213/e-commerce_shop/services/product-service/internal/health"
	productMongo "github.com/Krokozabra213/e-commerce_shop/services/product-service/internal/infra/mongodb"
	categoryReadRepo "github.com/Krokozabra213/e-commerce_shop/services/product-service/internal/repository/mongo/category/read"
	categoryWriteRepo "github.com/Krokozabra213/e-commerce_shop/services/product-service/internal/repository/mongo/category/write"
	productReadRepo "github.com/Krokozabra213/e-commerce_shop/services/product-service/internal/repository/mongo/product/read"
	productWriteRepo "github.com/Krokozabra213/e-commerce_shop/services/product-service/internal/repository/mongo/product/write"
	categoryService "github.com/Krokozabra213/e-commerce_shop/services/product-service/internal/service/category"
	productService "github.com/Krokozabra213/e-commerce_shop/services/product-service/internal/service/product"
	swaggo "github.com/gofiber/contrib/v3/swaggo"
	"github.com/gofiber/fiber/v3"
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

	mongoClient, err := inframongo.NewMongoClient(&cfg.Mongo)
	if err != nil {
		return err
	}
	defer func() {
		if err := inframongo.GracefulDisconnect(mongoClient, 10*time.Second); err != nil {
			log.Error("mongodb disconnect error", slog.String("error", err.Error()))
		}
	}()

	mongoDB := mongoClient.Database(cfg.Mongo.Database)
	if err := productMongo.SetupIndexes(ctx, mongoDB); err != nil {
		return fmt.Errorf("setup indexes: %w", err)
	}

	categoryRead := categoryReadRepo.NewCategoryReadRepo(mongoDB)
	categoryWrite := categoryWriteRepo.NewCategoryWriteRepo(mongoDB)
	productRead := productReadRepo.NewProductReadRepo(mongoDB)
	productWrite := productWriteRepo.NewProductWriteRepo(mongoDB)

	productSvc := productService.NewService(productWrite, productRead, categoryRead)
	categorySvc := categoryService.NewService(categoryWrite, categoryRead, productRead)

	handler := httphandler.NewHandler(productSvc, categorySvc, log)

	requestLogger := inframiddleware.RequestLogger(log)
	errorHandler := inframiddleware.NewErrorHandler(log)
	server := httpx.NewFiberServer(cfg.HTTP, log, errorHandler, requestLogger)

	server.App.Get("/swagger/*", swaggo.New(swaggo.Config{
		URL:         "/api/openapi.yaml",
		DeepLinking: true,
	}))

	server.App.Get("/api/openapi.yaml", func(c fiber.Ctx) error {
		return c.SendFile("./api/openapi.yaml")
	})

	handler.RegisterRoutes(server.App)

	healthHandler := health.NewHealthHandler(mongoClient)
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

	log.Info("application soft stopped")

	return nil
}
