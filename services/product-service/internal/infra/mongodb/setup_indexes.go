package productMongo

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func SetupIndexes(ctx context.Context, db *mongo.Database) error {
	if err := setupProductIndexes(ctx, db.Collection("products")); err != nil {
		return fmt.Errorf("setup product indexes: %w", err)
	}

	if err := setupCategoryIndexes(ctx, db.Collection("categories")); err != nil {
		return fmt.Errorf("setup category indexes: %w", err)
	}

	return nil
}

func setupProductIndexes(ctx context.Context, coll *mongo.Collection) error {
	indexes := []mongo.IndexModel{
		// 1. Основной индекс для сортировки по новизне
		// Покрывает: GET /products?sort=latest, GET /products?category=X&sort=latest
		{
			Keys: bson.D{
				{Key: "published", Value: 1},
				{Key: "deletedAt", Value: 1},
				{Key: "categorySlug", Value: 1},
				{Key: "createdAt", Value: -1},
				{Key: "_id", Value: 1},
			},
			Options: options.Index().SetName("idx_main_latest"),
		},

		// 2. Индекс для сортировки по цене (ASC/DESC через reverse scan)
		// Покрывает: GET /products?sort=price_asc, GET /products?sort=price_desc
		{
			Keys: bson.D{
				{Key: "published", Value: 1},
				{Key: "deletedAt", Value: 1},
				{Key: "categorySlug", Value: 1},
				{Key: "price", Value: 1},
				{Key: "_id", Value: 1},
			},
			Options: options.Index().SetName("idx_main_price"),
		},

		// 3. Индекс для фильтра по диапазону цен + сортировка по новизне
		// Покрывает: GET /products?category=X&price_min=Y&price_max=Z&sort=latest
		{
			Keys: bson.D{
				{Key: "published", Value: 1},
				{Key: "deletedAt", Value: 1},
				{Key: "categorySlug", Value: 1},
				{Key: "price", Value: 1},
				{Key: "createdAt", Value: -1},
				{Key: "_id", Value: 1},
			},
			Options: options.Index().SetName("idx_category_price_latest"),
		},

		// 4. Текстовый поиск (только опубликованные)
		// Покрывает: GET /products?search=iPhone
		{
			Keys: bson.D{
				{Key: "name", Value: "text"},
				{Key: "description", Value: "text"},
			},
			Options: options.Index().
				SetName("idx_text_search").
				SetWeights(bson.D{
					{Key: "name", Value: 10},
					{Key: "description", Value: 1},
				}).
				SetDefaultLanguage("russian").
				SetPartialFilterExpression(bson.M{
					"published": true,
					"deletedAt": nil,
				}),
		},
	}

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	_, err := coll.Indexes().CreateMany(ctx, indexes)
	return err
}

func setupCategoryIndexes(ctx context.Context, coll *mongo.Collection) error {
	index := mongo.IndexModel{
		Keys:    bson.D{{Key: "slug", Value: 1}},
		Options: options.Index().SetName("idx_slug_unique").SetUnique(true),
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	_, err := coll.Indexes().CreateOne(ctx, index)
	if err != nil {
		return fmt.Errorf("create category indexes: %w", err)
	}

	return nil
}
