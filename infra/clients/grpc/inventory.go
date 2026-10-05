package grpcclient

import (
	"context"
	"fmt"

	inventoryv1 "github.com/Krokozabra213/e-commerce_shop/api/gen/go/proto/inventory/v1"
	inframiddleware "github.com/Krokozabra213/e-commerce_shop/infra/middleware"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health/grpc_health_v1"
)

type InventoryClient struct {
	api  inventoryv1.InventoryServiceAPIClient
	conn *grpc.ClientConn
}

func NewInventoryClient(_ context.Context, addr string) (*InventoryClient, error) {
	conn, err := grpc.NewClient(
		addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
		grpc.WithUnaryInterceptor(inframiddleware.UnaryClientRequestIDInterceptor),
	)
	if err != nil {
		return nil, fmt.Errorf("inventory client: grpc new client: %w", err)
	}

	return &InventoryClient{
		api:  inventoryv1.NewInventoryServiceAPIClient(conn),
		conn: conn,
	}, nil
}

func (c *InventoryClient) Close() error {
	return c.conn.Close()
}

func (c *InventoryClient) HealthCheck(ctx context.Context) error {
	client := grpc_health_v1.NewHealthClient(c.conn)

	resp, err := client.Check(ctx, &grpc_health_v1.HealthCheckRequest{
		Service: "",
	})
	if err != nil {
		return fmt.Errorf("grpc health check failed: %w", err)
	}

	if resp.Status != grpc_health_v1.HealthCheckResponse_SERVING {
		return fmt.Errorf("grpc service is not serving: %s", resp.Status.String())
	}

	return nil
}

func (c *InventoryClient) GetStock(ctx context.Context, productIDs []string) (map[string]int32, error) {
	resp, err := c.api.GetStock(ctx, &inventoryv1.GetStockRequest{
		ProductIds: productIDs,
	})
	if err != nil {
		return nil, ParseGRPCError("inventory-service", err)
	}

	return resp.Quantities, nil
}

func (c *InventoryClient) GetStockByProductID(ctx context.Context, productID string) (*StockInfo, error) {
	resp, err := c.api.GetStockByProductID(ctx, &inventoryv1.GetStockByProductIDRequest{
		ProductId: productID,
	})
	if err != nil {
		return nil, ParseGRPCError("inventory-service", err)
	}

	return &StockInfo{
		Quantity: resp.Quantity,
		InStock:  resp.InStock,
	}, nil
}

type StockInfo struct {
	Quantity int32
	InStock  bool
}
