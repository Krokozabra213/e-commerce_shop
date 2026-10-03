package inframiddleware

import (
	"context"
	"log/slog"

	"github.com/Krokozabra213/e-commerce_shop/infra/logger"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

func UnaryServerRequestIDInterceptor(
	baseLogger *slog.Logger,
) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		var requestID string
		if md, ok := metadata.FromIncomingContext(ctx); ok {
			if vals := md.Get(MetadataRequestID); len(vals) > 0 {
				requestID = vals[0]
			}
		}

		if requestID == "" {
			requestID = uuid.New().String()
		}

		log := baseLogger.With(
			"request_id", requestID,
			"grpc_method", info.FullMethod,
		)

		ctx = logger.WithLogger(ctx, log)
		ctx = logger.WithRequestID(ctx, requestID)

		return handler(ctx, req)
	}
}
