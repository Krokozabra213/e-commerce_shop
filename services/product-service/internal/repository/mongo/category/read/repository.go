package categoryReadRepo

import (
	"context"
	"errors"
	"fmt"

	mongodto "github.com/Krokozabra213/e-commerce_shop/services/product-service/internal/repository/mongo/dto"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/Krokozabra213/e-commerce_shop/services/product-service/internal/domain"
)

type CategoryReadRepo struct {
	collection *mongo.Collection
}

func NewCategoryReadRepo(db *mongo.Database) *CategoryReadRepo {
	return &CategoryReadRepo{
		collection: db.Collection("categories"),
	}
}

func (r *CategoryReadRepo) GetBySlug(ctx context.Context, slug string) (*domain.Category, error) {
	filter := bson.M{
		"slug":      slug,
		"deletedAt": nil,
	}

	var doc mongodto.CategoryDocument
	if err := r.collection.FindOne(ctx, filter).Decode(&doc); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("CategoryReadRepo.GetBySlug FindOne: %w", err)
	}

	return doc.ToDomain(), nil
}

func (r *CategoryReadRepo) List(ctx context.Context) ([]domain.Category, error) {
	filter := bson.M{"deletedAt": nil}

	opts := options.Find().
		SetSort(bson.D{{Key: "createdAt", Value: -1}})

	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("CategoryReadRepo.List Find: %w", err)
	}
	defer func() {
		_ = cursor.Close(ctx)
	}()

	var docs []mongodto.CategoryDocument
	if err := cursor.All(ctx, &docs); err != nil {
		return nil, fmt.Errorf("CategoryReadRepo.List cursor.All: %w", err)
	}

	categories := make([]domain.Category, 0, len(docs))
	for _, doc := range docs {
		categories = append(categories, *doc.ToDomain())
	}

	return categories, nil
}
