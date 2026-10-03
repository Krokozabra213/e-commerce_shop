package inframiddleware

import (
	"context"

	"github.com/Krokozabra213/e-commerce_shop/infra/logger"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

const MetadataRequestID = "x-request-id"

func UnaryClientRequestIDInterceptor(
	ctx context.Context,
	method string,
	req, reply any,
	cc *grpc.ClientConn,
	invoker grpc.UnaryInvoker,
	opts ...grpc.CallOption,
) error {
	if id := logger.RequestIDFromContext(ctx); id != "" {
		if md, ok := metadata.FromOutgoingContext(ctx); ok {
			if len(md.Get(MetadataRequestID)) == 0 {
				ctx = metadata.AppendToOutgoingContext(ctx, MetadataRequestID, id)
			}
		} else {
			ctx = metadata.AppendToOutgoingContext(ctx, MetadataRequestID, id)
		}
	}
	return invoker(ctx, method, req, reply, cc, opts...)
}
