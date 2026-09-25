package grpcclient

import (
	"context"
	"fmt"

	inventoryv1 "github.com/Krokozabra213/e-commerce_shop/api/gen/go/proto/inventory/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type InventoryClient struct {
	api  inventoryv1.InventoryServiceAPIClient
	conn *grpc.ClientConn
}

func NewInventoryClient(ctx context.Context, addr string) (*InventoryClient, error) {
	conn, err := grpc.NewClient(
		addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
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

func (c *InventoryClient) GetStock(ctx context.Context, productIDs []string) (map[string]int32, error) {
	resp, err := c.api.GetStock(ctx, &inventoryv1.GetStockRequest{
		ProductIds: productIDs,
	})
	if err != nil {
		return nil, fmt.Errorf("inventory client: get stock: %w", err)
	}

	return resp.Quantities, nil
}

func (c *InventoryClient) GetStockByProductID(ctx context.Context, productID string) (*StockInfo, error) {
	resp, err := c.api.GetStockByProductID(ctx, &inventoryv1.GetStockByProductIDRequest{
		ProductId: productID,
	})
	if err != nil {
		return nil, fmt.Errorf("inventory client: get stock by product id: %w", err)
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
