package productReadRepo

import (
	"context"
	"errors"
	"fmt"

	"github.com/Krokozabra213/e-commerce_shop/services/product-service/internal/domain"
	mongodto "github.com/Krokozabra213/e-commerce_shop/services/product-service/internal/repository/mongo/dto"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type ProductReadRepo struct {
	collection *mongo.Collection
}

func NewProductReadRepo(db *mongo.Database) *ProductReadRepo {
	return &ProductReadRepo{
		collection: db.Collection("products"),
	}
}

func (r *ProductReadRepo) CountByCategory(ctx context.Context, categorySlug string) (int64, error) {
	filter := bson.M{
		"categorySlug": categorySlug,
		"deletedAt":    nil,
	}

	count, err := r.collection.CountDocuments(ctx, filter)
	if err != nil {
		return 0, fmt.Errorf("ProductReadRepo.CountByCategory: %w", err)
	}
	return count, nil
}

func (r *ProductReadRepo) GetByID(ctx context.Context, id string) (*domain.Product, error) {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, domain.ErrNotFound
	}

	filter := bson.M{
		"_id":       oid,
		"deletedAt": nil,
	}

	var doc mongodto.ProductDocument
	if err := r.collection.FindOne(ctx, filter).Decode(&doc); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("ProductReadRepo.GetByID FindOne: %w", err)
	}

	return doc.ToDomain(), nil
}

func (r *ProductReadRepo) List(ctx context.Context, filter domain.ListProductsFilter) ([]domain.Product, *domain.Cursor, error) {
	mongoFilter := bson.M{
		"published": true,
		"deletedAt": nil,
	}

	if filter.CategorySlug != nil && *filter.CategorySlug != "" {
		mongoFilter["categorySlug"] = *filter.CategorySlug
	}

	if filter.PriceMin != nil || filter.PriceMax != nil {
		priceFilter := bson.M{}
		if filter.PriceMin != nil {
			priceFilter["$gte"] = *filter.PriceMin
		}
		if filter.PriceMax != nil {
			priceFilter["$lte"] = *filter.PriceMax
		}
		mongoFilter["price"] = priceFilter
	}

	useTextSearch := filter.Search != nil && *filter.Search != ""
	if useTextSearch {
		mongoFilter["$text"] = bson.M{"$search": *filter.Search}
	}

	var sortOrder bson.D
	if useTextSearch {
		sortOrder = bson.D{
			{Key: "score", Value: bson.M{"$meta": "textScore"}},
			{Key: "_id", Value: 1},
		}
	} else {
		switch filter.Sort {
		case domain.SortPriceAsc:
			sortOrder = bson.D{
				{Key: "price", Value: 1},
				{Key: "_id", Value: 1},
			}
		case domain.SortPriceDesc:
			sortOrder = bson.D{
				{Key: "price", Value: -1},
				{Key: "_id", Value: -1},
			}
		default: // SortLatest
			sortOrder = bson.D{
				{Key: "createdAt", Value: -1},
				{Key: "_id", Value: 1},
			}
		}
	}

	if filter.Cursor != nil && !useTextSearch {
		c := filter.Cursor

		cursorOID, err := primitive.ObjectIDFromHex(c.ID)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid cursor ID: %w", err)
		}

		var cursorFilter bson.M
		switch filter.Sort {
		case domain.SortPriceAsc:
			cursorFilter = bson.M{
				"$or": []bson.M{
					{"price": bson.M{"$gt": *c.Price}},
					{
						"price": *c.Price,
						"_id":   bson.M{"$gt": cursorOID},
					},
				},
			}
		case domain.SortPriceDesc:
			cursorFilter = bson.M{
				"$or": []bson.M{
					{"price": bson.M{"$lt": *c.Price}},
					{
						"price": *c.Price,
						"_id":   bson.M{"$lt": cursorOID},
					},
				},
			}
		default: // SortLatest
			cursorFilter = bson.M{
				"$or": []bson.M{
					{"createdAt": bson.M{"$lt": *c.CreatedAt}},
					{
						"createdAt": *c.CreatedAt,
						"_id":       bson.M{"$gt": cursorOID},
					},
				},
			}
		}

		mongoFilter = bson.M{"$and": []bson.M{mongoFilter, cursorFilter}}
	}

	limit := int64(filter.Limit)
	if limit <= 0 {
		limit = 20
	}

	opts := options.Find().SetSort(sortOrder).SetLimit(limit + 1)

	if useTextSearch {
		opts.SetProjection(bson.M{
			"score": bson.M{"$meta": "textScore"},
		})
	}

	cursor, err := r.collection.Find(ctx, mongoFilter, opts)
	if err != nil {
		return nil, nil, fmt.Errorf("ProductReadRepo.List Find: %w", err)
	}
	defer cursor.Close(ctx)

	var docs []mongodto.ProductDocument
	if err := cursor.All(ctx, &docs); err != nil {
		return nil, nil, fmt.Errorf("ProductReadRepo.List cursor.All: %w", err)
	}

	hasMore := int64(len(docs)) > limit
	if hasMore {
		docs = docs[:limit]
	}

	products := make([]domain.Product, 0, len(docs))
	for i := range docs {
		products = append(products, *docs[i].ToDomain())
	}

	var nextCursor *domain.Cursor
	if hasMore && len(docs) > 0 && !useTextSearch {
		last := docs[len(docs)-1]
		nextCursor = &domain.Cursor{
			CreatedAt: &last.CreatedAt,
			Price:     &last.Price,
			ID:        last.ID.Hex(),
		}
	}

	return products, nextCursor, nil
}
