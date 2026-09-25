package inframiddleware

import (
	"context"
	"log/slog"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

func LoggingInterceptor(log *slog.Logger) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		start := time.Now()

		log.InfoContext(ctx, "gRPC request started",
			slog.String("method", info.FullMethod),
			slog.Any("request", req),
		)

		resp, err := handler(ctx, req)

		duration := time.Since(start)
		code := status.Code(err)

		attrs := []slog.Attr{
			slog.String("method", info.FullMethod),
			slog.String("status", code.String()),
			slog.String("duration", duration.String()),
		}

		if err != nil {
			attrs = append(attrs, slog.String("error", err.Error()))
			log.LogAttrs(ctx, slog.LevelError, "gRPC request failed", attrs...)
		} else {
			log.LogAttrs(ctx, slog.LevelInfo, "gRPC request completed", attrs...)
		}

		return resp, err
	}
}
