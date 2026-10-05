package httpx

import (
	"context"
	"log/slog"

	infracfg "github.com/Krokozabra213/e-commerce_shop/infra/config"
	"github.com/go-playground/validator/v10"
	fiberotel "github.com/gofiber/contrib/v3/otel"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"
	"github.com/gofiber/fiber/v3/middleware/recover"
	"github.com/gofiber/fiber/v3/middleware/requestid"
	"github.com/google/uuid"
)

type Server struct {
	App    *fiber.App
	cfg    infracfg.HTTPConfig
	logger *slog.Logger
}

type ErrorHandler interface {
	Handle(ctx fiber.Ctx, err error) error
}

type StructValidator struct {
	validate *validator.Validate
}

func (v *StructValidator) Validate(out any) error {
	return v.validate.Struct(out)
}

func NewFiberServer(cfg infracfg.HTTPConfig, logger *slog.Logger, errorHandler ErrorHandler, middlewares ...func(c fiber.Ctx) error) *Server {
	fiberApp := fiber.New(fiber.Config{
		ReadTimeout:     cfg.ReadTimeout,
		WriteTimeout:    cfg.WriteTimeout,
		ReadBufferSize:  cfg.MaxHeaderBytes * 1024 * 1024,
		BodyLimit:       cfg.BodyLimitBytes * 1024 * 1024,
		ErrorHandler:    errorHandler.Handle,
		StructValidator: &StructValidator{validate: validator.New()},
	})

	fiberApp.Use(cors.New(cors.Config{
		AllowOrigins:     cfg.AllowedOrigins,
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		AllowCredentials: true,
	}))
	fiberApp.Use(recover.New())

	fiberApp.Use(fiberotel.Middleware())

	fiberApp.Use(requestid.New(requestid.Config{
		Header: "X-Request-Id",
		Generator: func() string {
			return uuid.New().String()
		},
	}))

	for _, mw := range middlewares {
		fiberApp.Use(mw)
	}

	return &Server{
		App:    fiberApp,
		cfg:    cfg,
		logger: logger,
	}
}

func (a *Server) Run() error {
	a.logger.Info("server starting", "addr", a.cfg.HTTPAddress())
	return a.App.Listen(a.cfg.HTTPAddress())
}

func (a *Server) Shutdown(ctx context.Context) error {
	a.logger.Info("shutting down server...")
	return a.App.ShutdownWithContext(ctx)
}
