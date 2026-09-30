package productService

import (
	"context"
	"errors"
	"time"

	"github.com/Krokozabra213/e-commerce_shop/infra/apperror"
	"github.com/Krokozabra213/e-commerce_shop/services/product-service/internal/domain"
)

type ProductWriteRepository interface {
	Create(ctx context.Context, product *domain.Product) (string, error)
	Update(ctx context.Context, id string, input *domain.UpdateProductInput) (*domain.Product, error)
	Delete(ctx context.Context, id string) error
	TogglePublish(ctx context.Context, id string) (*domain.Product, error)
	GetPricesByIDs(ctx context.Context, ids []string) ([]domain.ProductPrice, error)
}

type ProductReadRepository interface {
	GetByID(ctx context.Context, id string) (*domain.Product, error)
	List(ctx context.Context, filter domain.ListProductsFilter) ([]domain.Product, *domain.Cursor, error)
}

type CategoryReadRepository interface {
	GetBySlug(ctx context.Context, slug string) (*domain.Category, error)
}

type Service struct {
	writeRepo    ProductWriteRepository
	readRepo     ProductReadRepository
	categoryRepo CategoryReadRepository
}

func NewService(
	writeRepo ProductWriteRepository,
	readRepo ProductReadRepository,
	categoryRepo CategoryReadRepository,
) *Service {
	return &Service{
		writeRepo:    writeRepo,
		readRepo:     readRepo,
		categoryRepo: categoryRepo,
	}
}

func (s *Service) Create(ctx context.Context, input *domain.CreateProductInput) (*domain.Product, error) {
	if _, err := s.categoryRepo.GetBySlug(ctx, input.CategorySlug); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, apperror.NewBusiness(apperror.CodeNotFound, "category not found")
		}
		return nil, apperror.NewInternal("categoryRepo.GetBySlug", err, "Что-то пошло не так", nil)
	}

	now := time.Now()
	product := &domain.Product{
		Name:         input.Name,
		Description:  input.Description,
		Price:        input.Price,
		CategorySlug: input.CategorySlug,
		Published:    input.Published,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	id, err := s.writeRepo.Create(ctx, product)
	if err != nil {
		if errors.Is(err, domain.ErrAlreadyExists) {
			return nil, apperror.NewBusiness(apperror.CodeAlreadyExists, "product already exists")
		}
		return nil, apperror.NewInternal("writeRepo.Create", err, "Что-то пошло не так", nil)
	}
	product.ID = id
	return product, nil
}

func (s *Service) GetByID(ctx context.Context, id string) (*domain.Product, error) {
	product, err := s.readRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, apperror.NewBusiness(apperror.CodeNotFound, "product not found")
		}
		return nil, apperror.NewInternal("readRepo.GetByID", err, "Что-то пошло не так", nil)
	}

	if product.DeletedAt != nil {
		return nil, apperror.NewBusiness(apperror.CodeNotFound, "product not found")
	}

	return product, nil
}

func (s *Service) List(ctx context.Context, filter *domain.ListProductsFilter) ([]domain.Product, *domain.Cursor, error) {
	products, nextCursor, err := s.readRepo.List(ctx, *filter)
	if err != nil {
		return nil, nil, apperror.NewInternal("readRepo.List", err, "Что-то пошло не так", nil)
	}

	return products, nextCursor, nil
}

func (s *Service) Update(ctx context.Context, id string, input *domain.UpdateProductInput) (*domain.Product, error) {
	if input.CategorySlug != nil {
		if _, err := s.categoryRepo.GetBySlug(ctx, *input.CategorySlug); err != nil {
			if errors.Is(err, domain.ErrNotFound) {
				return nil, apperror.NewBusiness(apperror.CodeNotFound, "category not found")
			}
			return nil, apperror.NewInternal("categoryRepo.GetBySlug", err, "failed to check category", nil)
		}
	}

	updated, err := s.writeRepo.Update(ctx, id, input)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, apperror.NewBusiness(apperror.CodeNotFound, "product not found")
		}
		return nil, apperror.NewInternal("writeRepo.Update", err, "failed to update product", nil)
	}

	return updated, nil
}

func (s *Service) Delete(ctx context.Context, id string) error {
	if err := s.writeRepo.Delete(ctx, id); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return apperror.NewBusiness(apperror.CodeNotFound, "product not found")
		}
		return apperror.NewInternal("writeRepo.Delete", err, "failed to delete product", nil)
	}
	return nil
}

func (s *Service) TogglePublish(ctx context.Context, id string) (*domain.Product, error) {
	product, err := s.writeRepo.TogglePublish(ctx, id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, apperror.NewBusiness(apperror.CodeNotFound, "product not found")
		}
		return nil, apperror.NewInternal("writeRepo.TogglePublish", err, "failed to toggle publish", nil)
	}
	return product, nil
}

func (s *Service) GetPricesByIDs(ctx context.Context, ids []string) ([]domain.ProductPrice, error) {
	prices, err := s.writeRepo.GetPricesByIDs(ctx, ids)
	if err != nil {
		return nil, apperror.NewInternal("writeRepo.GetPricesByIDs", err, "Что-то пошло не так", nil)
	}

	return prices, nil
}
