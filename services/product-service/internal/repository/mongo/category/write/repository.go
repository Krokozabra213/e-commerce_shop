package categoryWriteRepo

import (
	"context"
	"errors"
	"fmt"
	"time"

	inframongo "github.com/Krokozabra213/e-commerce_shop/infra/mongo"
	"github.com/Krokozabra213/e-commerce_shop/services/product-service/internal/domain"
	mongodto "github.com/Krokozabra213/e-commerce_shop/services/product-service/internal/repository/mongo/dto"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type CategoryWriteRepo struct {
	collection *mongo.Collection
}

func NewCategoryWriteRepo(db *mongo.Database) *CategoryWriteRepo {
	return &CategoryWriteRepo{
		collection: db.Collection("categories"),
	}
}

func (r *CategoryWriteRepo) Create(ctx context.Context, category *domain.Category) (string, error) {
	doc := mongodto.CategoryDocument{
		ID:          primitive.NewObjectID(),
		Slug:        category.Slug,
		Name:        category.Name,
		Description: category.Description,
		CreatedAt:   category.CreatedAt,
		UpdatedAt:   category.UpdatedAt,
	}

	_, err := r.collection.InsertOne(ctx, doc)
	if err != nil {
		if inframongo.IsDuplicateKeyError(err) {
			return "", domain.ErrAlreadyExists
		}
		return "", fmt.Errorf("CategoryWriteRepo.Create InsertOne: %w", err)
	}

	return doc.ID.Hex(), nil
}

func (r *CategoryWriteRepo) Update(ctx context.Context, slug string, input domain.UpdateCategoryInput) (*domain.Category, error) {
	filter := bson.M{
		"slug":      slug,
		"deletedAt": nil,
	}

	set := bson.M{
		"updatedAt": time.Now(),
	}
	if input.Name != nil {
		set["name"] = *input.Name
	}
	if input.Description != nil {
		set["description"] = *input.Description
	}

	opts := options.FindOneAndUpdate().
		SetReturnDocument(options.After)

	var doc mongodto.CategoryDocument
	err := r.collection.FindOneAndUpdate(ctx, filter, bson.M{"$set": set}, opts).Decode(&doc)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("CategoryWriteRepo.Update FindOneAndUpdate: %w", err)
	}

	return doc.ToDomain(), nil
}

func (r *CategoryWriteRepo) Delete(ctx context.Context, slug string) error {
	filter := bson.M{
		"slug":      slug,
		"deletedAt": nil,
	}

	update := bson.M{"$set": bson.M{"deletedAt": time.Now()}}

	result, err := r.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("CategoryWriteRepo.Delete UpdateOne: %w", err)
	}
	if result.MatchedCount == 0 {
		return domain.ErrNotFound
	}
	return nil
}
