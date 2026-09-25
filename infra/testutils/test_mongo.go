package testutils

import (
	"context"
	"fmt"
	"testing"

	"github.com/testcontainers/testcontainers-go/modules/mongodb"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type TestMongoDB struct {
	Container *mongodb.MongoDBContainer
	Client    *mongo.Client
	ConnStr   string
	DBName    string
}

func (td *TestMongoDB) Close(ctx context.Context) error {
	if td.Client != nil {
		_ = td.Client.Disconnect(ctx)
	}
	if td.Container != nil {
		return td.Container.Terminate(ctx)
	}
	return nil
}

func SetupTestMongoDBShared(ctx context.Context, setupIndexesFunc func(context.Context, *mongo.Database) error) (*TestMongoDB, error) {
	const dbName = "testdb"

	container, err := mongodb.Run(ctx, "mongo:7")
	if err != nil {
		return nil, fmt.Errorf("failed to start mongo container: %w", err)
	}

	connStr, err := container.ConnectionString(ctx)
	if err != nil {
		_ = container.Terminate(ctx)
		return nil, fmt.Errorf("failed to get connection string: %w", err)
	}

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(connStr))
	if err != nil {
		_ = container.Terminate(ctx)
		return nil, fmt.Errorf("failed to create mongo client: %w", err)
	}

	if err := client.Ping(ctx, nil); err != nil {
		_ = client.Disconnect(ctx)
		_ = container.Terminate(ctx)
		return nil, fmt.Errorf("failed to ping mongo: %w", err)
	}

	db := client.Database(dbName)

	if setupIndexesFunc != nil {
		if err := setupIndexesFunc(ctx, db); err != nil {
			_ = client.Disconnect(ctx)
			_ = container.Terminate(ctx)
			return nil, fmt.Errorf("failed to setup indexes: %w", err)
		}
	}

	return &TestMongoDB{
		Container: container,
		Client:    client,
		ConnStr:   connStr,
		DBName:    dbName,
	}, nil
}

func TruncateCollections(t *testing.T, db *mongo.Database, collections ...string) {
	t.Helper()
	for _, name := range collections {
		_, err := db.Collection(name).DeleteMany(context.Background(), bson.M{})
		if err != nil {
			t.Fatalf("failed to clear collection %s: %v", name, err)
		}
	}
}
