package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	grpcclient "github.com/Krokozabra213/e-commerce_shop/infra/clients/grpc"
	httpclient "github.com/Krokozabra213/e-commerce_shop/infra/clients/http"
	"github.com/Krokozabra213/e-commerce_shop/infra/httpx"
	infrajwt "github.com/Krokozabra213/e-commerce_shop/infra/jwt"
	jwtvalidator "github.com/Krokozabra213/e-commerce_shop/infra/jwt/validator"
	"github.com/Krokozabra213/e-commerce_shop/infra/logger"
	inframiddleware "github.com/Krokozabra213/e-commerce_shop/infra/middleware"
	"github.com/Krokozabra213/e-commerce_shop/infra/ratelimit"
	infraredis "github.com/Krokozabra213/e-commerce_shop/infra/redis"
	"github.com/Krokozabra213/e-commerce_shop/services/api-gateway/internal/config"
	"github.com/Krokozabra213/e-commerce_shop/services/api-gateway/internal/handler/auth"
	"github.com/Krokozabra213/e-commerce_shop/services/api-gateway/internal/handler/inventory"
	"github.com/Krokozabra213/e-commerce_shop/services/api-gateway/internal/handler/order"
	"github.com/Krokozabra213/e-commerce_shop/services/api-gateway/internal/handler/product"
	"github.com/Krokozabra213/e-commerce_shop/services/api-gateway/internal/handler/user"
	"github.com/Krokozabra213/e-commerce_shop/services/api-gateway/internal/health"
	"github.com/Krokozabra213/e-commerce_shop/services/api-gateway/internal/middleware"
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

	redis, err := infraredis.NewClient(cfg.Redis, log)
	if err != nil {
		return err
	}
	defer func() {
		_ = redis.Close()
	}()

	authHTTPClient := httpclient.NewAuthClient(cfg.HTTPAuthClient)
	inventoryHTTPClient := httpclient.NewInventoryClient(cfg.HTTPInventoryClient)
	orderHTTPClient := httpclient.NewOrderClient(cfg.HTTPOrderClient)
	productHTTPClient := httpclient.NewProductClient(cfg.HTTPProductClient)
	userHTTPClient := httpclient.NewUserClient(cfg.HTTPUserClient)

	inventoryGRPCClient, err := grpcclient.NewInventoryClient(context.Background(), cfg.GRPCInventoryClient.Addr)
	if err != nil {
		return err
	}
	defer func() {
		_ = inventoryGRPCClient.Close()
	}()

	resp, err := authHTTPClient.GetPublicKey(context.Background())
	if err != nil {
		return err
	}

	pubKey, err := infrajwt.ParseRSAPublicKey(resp.PublicKey)
	if err != nil {
		return err
	}

	rs256Validator, err := jwtvalidator.NewRS256Validator(pubKey, cfg.JWTClient.Leeway, cfg.JWTClient.Issuer)
	if err != nil {
		return err
	}

	limiter := ratelimit.New(redis, cfg.RateLimiter.KeyPrefix, log)

	jwtMiddleware := middleware.NewJWTMiddleware(rs256Validator)
	rolesMiddleware := middleware.NewRolesMiddleware(userHTTPClient)
	rateLimitMiddleware := middleware.NewRateLimitMiddleware(limiter, cfg.RateLimiter, log)

	authHandler := auth.NewHandler(authHTTPClient)
	orderHandler := order.NewHandler(orderHTTPClient)
	inventoryHandler := inventory.NewHandler(inventoryHTTPClient)
	productHandler := product.NewHandler(productHTTPClient, inventoryGRPCClient, log)
	userHandler := user.NewHandler(userHTTPClient)

	loggerRequestIDMiddleware := inframiddleware.RequestLogger(log)
	errorHandler := inframiddleware.NewErrorHandler(log)
	httpServer := httpx.NewFiberServer(cfg.HTTP, log, errorHandler, rateLimitMiddleware, loggerRequestIDMiddleware)

	httpServer.App.Get("/swagger/*", swaggo.New(swaggo.Config{
		URL:         "/api/openapi.yaml",
		DeepLinking: true,
	}))

	httpServer.App.Get("/api/openapi.yaml", func(c fiber.Ctx) error {
		return c.SendFile("./api/openapi.yaml")
	})

	authHandler.RegisterRoutes(httpServer.App, jwtMiddleware)
	orderHandler.RegisterRoutes(httpServer.App, jwtMiddleware, rolesMiddleware)
	inventoryHandler.RegisterRoutes(httpServer.App, jwtMiddleware, rolesMiddleware)
	productHandler.RegisterRoutes(httpServer.App, jwtMiddleware, rolesMiddleware)
	userHandler.RegisterRoutes(httpServer.App, jwtMiddleware, rolesMiddleware)

	errCh := make(chan error, 1)
	go func() {
		log.Info("http server started", "address", cfg.HTTP.HTTPAddress())
		if err := httpServer.Run(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	healthHandler := health.NewHandler(redis)
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
