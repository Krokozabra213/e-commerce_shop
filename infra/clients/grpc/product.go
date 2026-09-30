package grpcclient

import (
	"context"
	"fmt"

	productv1 "github.com/Krokozabra213/e-commerce_shop/api/gen/go/proto/product/v1"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type ProductClient struct {
	api  productv1.ProductServiceAPIClient
	conn *grpc.ClientConn
}

func NewProductClient(ctx context.Context, addr string) (*ProductClient, error) {
	conn, err := grpc.NewClient(
		addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
	)
	if err != nil {
		return nil, fmt.Errorf("product client: grpc new client: %w", err)
	}

	return &ProductClient{
		api:  productv1.NewProductServiceAPIClient(conn),
		conn: conn,
	}, nil
}

func (c *ProductClient) Close() error {
	return c.conn.Close()
}

func (c *ProductClient) GetPrices(ctx context.Context, ids []string) (map[string]int64, error) {
	resp, err := c.api.GetPricesByProductIDs(ctx, &productv1.GetPricesByProductIDsRequest{
		ProductIds: ids,
	})
	if err != nil {
		return nil, fmt.Errorf("product client: get prices: %w", err)
	}

	return resp.Prices, nil
}
