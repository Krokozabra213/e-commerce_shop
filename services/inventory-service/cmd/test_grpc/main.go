package main

import (
	"context"
	"log"
	"log/slog"
	"os"

	grpcclient "github.com/Krokozabra213/e-commerce_shop/infra/clients/grpc"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	ctx := context.Background()

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable"
	}

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer pool.Close()

	testProducts := prepareTestData(ctx, pool, logger)

	serverAddr := "localhost:44100"

	logger.Info("connecting to inventory service", slog.String("addr", serverAddr))

	client, err := grpcclient.NewInventoryClient(ctx, serverAddr)
	if err != nil {
		log.Fatalf("failed to create client: %v", err)
	}
	defer func() {
		_ = client.Close()
	}()

	logger.Info("successfully connected to inventory service")

	logger.Info("=== Test 1: Get existing products ===")
	quantities, err := client.GetStock(ctx, testProducts)
	if err != nil {
		logger.Error("failed to get stock", slog.Any("error", err))
	} else {
		logger.Info("received stock", slog.Int("count", len(quantities)))
		for productID, qty := range quantities {
			logger.Info("product", slog.String("id", productID), slog.Int("qty", int(qty)))
		}
	}

	logger.Info("=== Test 2: Get specific product ===")
	if len(testProducts) > 0 {
		stockInfo, err := client.GetStockByProductID(ctx, testProducts[0])
		if err != nil {
			logger.Error("failed to get product", slog.Any("error", err))
		} else {
			logger.Info("product info",
				slog.String("id", testProducts[0]),
				slog.Int("quantity", int(stockInfo.Quantity)),
				slog.Bool("in_stock", stockInfo.InStock),
			)
		}
	}

	logger.Info("=== Test 3: Non-existent product ===")
	_, err = client.GetStockByProductID(ctx, uuid.New().String())
	if err != nil {
		logger.Info("expected error received", slog.String("error", err.Error()))
	}

	cleanupTestData(ctx, pool, testProducts, logger)

	logger.Info("all tests completed")
}

func prepareTestData(ctx context.Context, pool *pgxpool.Pool, logger *slog.Logger) []string {
	logger.Info("preparing test data")

	testProducts := make([]string, 3)

	for i := 0; i < 3; i++ {
		productID := uuid.New().String()
		quantity := (i + 1) * 10

		query := `
			INSERT INTO stocks (product_id, available_quantity, created_at, updated_at)
			VALUES ($1, $2, NOW(), NOW())
			ON CONFLICT (product_id) DO UPDATE SET available_quantity = $2
		`

		_, err := pool.Exec(ctx, query, productID, quantity)
		if err != nil {
			logger.Error("failed to insert test product", slog.Any("error", err))
			continue
		}

		testProducts[i] = productID
		logger.Info("created test product",
			slog.String("product_id", productID),
			slog.Int("quantity", quantity),
		)
	}

	return testProducts
}

func cleanupTestData(ctx context.Context, pool *pgxpool.Pool, productIDs []string, logger *slog.Logger) {
	logger.Info("cleaning up test data")

	for _, productID := range productIDs {
		query := `DELETE FROM stocks WHERE product_id = $1`
		_, err := pool.Exec(ctx, query, productID)
		if err != nil {
			logger.Error("failed to delete test product",
				slog.String("product_id", productID),
				slog.Any("error", err),
			)
		}
	}

	logger.Info("test data cleaned up")
}
