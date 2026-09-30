package productWriteRepo

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

type ProductWriteRepo struct {
	collection *mongo.Collection
}

func NewProductWriteRepo(db *mongo.Database) *ProductWriteRepo {
	return &ProductWriteRepo{
		collection: db.Collection("products"),
	}
}

func (r *ProductWriteRepo) Create(ctx context.Context, product *domain.Product) (string, error) {
	doc := mongodto.ProductDocument{
		ID:           primitive.NewObjectID(),
		Name:         product.Name,
		Description:  product.Description,
		Price:        product.Price,
		CategorySlug: product.CategorySlug,
		Published:    product.Published,
		CreatedAt:    product.CreatedAt,
		UpdatedAt:    product.UpdatedAt,
		DeletedAt:    nil,
	}

	_, err := r.collection.InsertOne(ctx, doc)
	if err != nil {
		if inframongo.IsDuplicateKeyError(err) {
			return "", domain.ErrAlreadyExists
		}
		return "", fmt.Errorf("ProductWriteRepo.Create InsertOne: %w", err)
	}

	return doc.ID.Hex(), nil
}

func (r *ProductWriteRepo) Update(ctx context.Context, id string, input *domain.UpdateProductInput) (*domain.Product, error) {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, domain.ErrNotFound
	}

	filter := bson.M{
		"_id":       oid,
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

	if input.Price != nil {
		set["price"] = *input.Price
	}

	if input.CategorySlug != nil {
		set["categorySlug"] = *input.CategorySlug
	}

	if input.Published != nil {
		set["published"] = *input.Published
	}

	opts := options.FindOneAndUpdate().
		SetReturnDocument(options.After)

	var doc mongodto.ProductDocument
	err = r.collection.FindOneAndUpdate(ctx, filter, bson.M{"$set": set}, opts).Decode(&doc)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("ProductWriteRepo.Update FindOneAndUpdate: %w", err)
	}

	return doc.ToDomain(), nil
}

func (r *ProductWriteRepo) Delete(ctx context.Context, id string) error {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return domain.ErrNotFound
	}

	filter := bson.M{
		"_id":       oid,
		"deletedAt": nil,
	}

	update := bson.M{
		"$set": bson.M{
			"deletedAt": time.Now(),
		},
	}

	result, err := r.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("ProductWriteRepo.Delete UpdateOne: %w", err)
	}

	if result.MatchedCount == 0 {
		return domain.ErrNotFound
	}

	return nil
}

func (r *ProductWriteRepo) TogglePublish(ctx context.Context, id string) (*domain.Product, error) {
	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, domain.ErrNotFound
	}

	filter := bson.M{
		"_id":       objID,
		"deletedAt": nil,
	}

	pipeline := bson.A{
		bson.M{"$set": bson.M{
			"published": bson.M{"$not": "$published"},
			"updatedAt": time.Now(),
		}},
	}

	opts := options.FindOneAndUpdate().
		SetReturnDocument(options.After)

	var doc mongodto.ProductDocument
	err = r.collection.FindOneAndUpdate(ctx, filter, pipeline, opts).Decode(&doc)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("ProductWriteRepo.TogglePublish FindOneAndUpdate: %w", err)
	}

	return doc.ToDomain(), nil
}

func (r *ProductWriteRepo) GetPricesByIDs(ctx context.Context, ids []string) ([]domain.ProductPrice, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	objectIDs := make([]primitive.ObjectID, 0, len(ids))
	for _, id := range ids {
		oid, err := primitive.ObjectIDFromHex(id)
		if err != nil {
			continue
		}
		objectIDs = append(objectIDs, oid)
	}

	if len(objectIDs) == 0 {
		return nil, nil
	}

	filter := bson.M{
		"_id":       bson.M{"$in": objectIDs},
		"deletedAt": nil,
	}

	opts := options.Find().SetProjection(bson.M{
		"_id":   1,
		"price": 1,
	})

	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("ProductWriteRepo.GetPricesByIDs Find: %w", err)
	}
	defer func() {
		_ = cursor.Close(ctx)
	}()

	var results []domain.ProductPrice
	for cursor.Next(ctx) {
		var doc mongodto.ProductPrice
		if err := cursor.Decode(&doc); err != nil {
			return nil, fmt.Errorf("ProductWriteRepo.GetPricesByIDs Decode: %w", err)
		}
		results = append(results, domain.ProductPrice{
			ProductID: doc.ID.Hex(),
			Price:     doc.Price,
		})
	}

	if err := cursor.Err(); err != nil {
		return nil, fmt.Errorf("ProductWriteRepo.GetPricesByIDs cursor: %w", err)
	}

	return results, nil
}
