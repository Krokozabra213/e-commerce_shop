package inframongo

import (
	"context"
	"errors"
	"fmt"
	"time"

	infracfg "github.com/Krokozabra213/e-commerce_shop/infra/config"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.opentelemetry.io/contrib/instrumentation/go.mongodb.org/mongo-driver/mongo/otelmongo"
)

func NewMongoClient(cfg *infracfg.MongoDBConfig) (*mongo.Client, error) {
	ctx, cancel := context.WithTimeout(context.Background(), cfg.ConnectTimeout)
	defer cancel()

	clientOpts := options.Client().ApplyURI(cfg.URI)

	clientOpts.SetMonitor(otelmongo.NewMonitor())

	clientOpts.SetMaxPoolSize(cfg.MaxPoolSize)
	clientOpts.SetMinPoolSize(cfg.MinPoolSize)
	clientOpts.SetMaxConnIdleTime(cfg.MaxConnIdleTime)

	clientOpts.SetConnectTimeout(cfg.ConnectTimeout)
	clientOpts.SetSocketTimeout(cfg.SocketTimeout)
	clientOpts.SetServerSelectionTimeout(cfg.ServerTimeout)

	clientOpts.SetRetryWrites(cfg.RetryWrites)
	clientOpts.SetRetryReads(true)

	if len(cfg.Compressors) > 0 {
		clientOpts.SetCompressors(cfg.Compressors)
	}

	clientOpts.SetAppName(cfg.AppName)

	client, err := mongo.Connect(ctx, clientOpts)
	if err != nil {
		return nil, fmt.Errorf("mongodb: connect: %w", err)
	}

	if err := client.Ping(ctx, nil); err != nil {
		_ = client.Disconnect(context.Background())
		return nil, fmt.Errorf("mongodb: ping: %w", err)
	}

	return client, nil
}

func GracefulDisconnect(client *mongo.Client, timeout time.Duration) error {
	if client == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	return client.Disconnect(ctx)
}

func IsDuplicateKeyError(err error) bool {
	var writeErr mongo.WriteException
	if errors.As(err, &writeErr) {
		for _, we := range writeErr.WriteErrors {
			if we.Code == 11000 {
				return true
			}
		}
	}
	return false
}
