package grpcserver

import (
	"fmt"
	"log/slog"
	"net"

	productv1 "github.com/Krokozabra213/e-commerce_shop/api/gen/go/proto/product/v1"
	infracfg "github.com/Krokozabra213/e-commerce_shop/infra/config"
	inframiddleware "github.com/Krokozabra213/e-commerce_shop/infra/middleware"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

type GRPCApp struct {
	gRPCServer *grpc.Server
	cfg        *infracfg.GRPCConfig
	log        *slog.Logger
}

func New(
	cfg *infracfg.GRPCConfig,
	log *slog.Logger,
	productHandler productv1.ProductServiceAPIServer,
) *GRPCApp {
	errorInterceptor := inframiddleware.NewErrorInterceptor(log)
	panicInterceptor := inframiddleware.PanicRecoveryInterceptor(log)
	loggingInterceptor := inframiddleware.LoggingInterceptor(log)

	opts := []grpc.ServerOption{
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
		grpc.ChainUnaryInterceptor(
			panicInterceptor,
			loggingInterceptor,
			errorInterceptor.Unary(),
		),
	}

	gRPCServer := grpc.NewServer(opts...)

	productv1.RegisterProductServiceAPIServer(gRPCServer, productHandler)

	reflection.Register(gRPCServer)

	return &GRPCApp{
		gRPCServer: gRPCServer,
		cfg:        cfg,
		log:        log,
	}
}

func (a *GRPCApp) RunGRPC() error {
	const op = "grpcapp.Run"

	log := a.log.With(
		slog.String("op", op),
		slog.String("host", a.cfg.Host),
		slog.String("port", a.cfg.Port),
	)

	l, err := net.Listen("tcp", a.cfg.GRPCAddress())
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	log.Info("grpc server is running", slog.String("addr", l.Addr().String()))

	if err := a.gRPCServer.Serve(l); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	return nil
}

func (a *GRPCApp) MustRun() {
	if err := a.RunGRPC(); err != nil {
		panic(err)
	}
}

func (a *GRPCApp) Stop() {
	const op = "grpcapp.Stop"

	a.log.With(slog.String("op", op)).
		Info("stopping gRPC server", slog.String("address", a.cfg.GRPCAddress()))

	a.gRPCServer.GracefulStop()
}
