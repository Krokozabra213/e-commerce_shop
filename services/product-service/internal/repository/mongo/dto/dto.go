package mongodto

import (
	"time"

	"github.com/Krokozabra213/e-commerce_shop/services/product-service/internal/domain"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type CategoryDocument struct {
	ID          primitive.ObjectID `bson:"_id"`
	Slug        string             `bson:"slug"`
	Name        string             `bson:"name"`
	Description string             `bson:"description"`
	CreatedAt   time.Time          `bson:"createdAt"`
	UpdatedAt   time.Time          `bson:"updatedAt"`
	DeletedAt   *time.Time         `bson:"deletedAt,omitempty"`
}

func (d *CategoryDocument) ToDomain() *domain.Category {
	return &domain.Category{
		ID:          d.ID.Hex(),
		Slug:        d.Slug,
		Name:        d.Name,
		Description: d.Description,
		CreatedAt:   d.CreatedAt,
		UpdatedAt:   d.UpdatedAt,
	}
}

type ProductDocument struct {
	ID           primitive.ObjectID `bson:"_id,omitempty"`
	Name         string             `bson:"name"`
	Description  *string            `bson:"description,omitempty"`
	Price        int64              `bson:"price"`
	CategorySlug string             `bson:"categorySlug"`
	Published    bool               `bson:"published"`
	CreatedAt    time.Time          `bson:"createdAt"`
	UpdatedAt    time.Time          `bson:"updatedAt"`
	DeletedAt    *time.Time         `bson:"deletedAt,omitempty"`
}

func (d *ProductDocument) ToDomain() *domain.Product {
	return &domain.Product{
		ID:           d.ID.Hex(),
		Name:         d.Name,
		Description:  d.Description,
		Price:        d.Price,
		CategorySlug: d.CategorySlug,
		Published:    d.Published,
		CreatedAt:    d.CreatedAt,
		UpdatedAt:    d.UpdatedAt,
		DeletedAt:    d.DeletedAt,
	}
}

type ProductPrice struct {
	ID    primitive.ObjectID `bson:"_id"`
	Price int64              `bson:"price"`
}
