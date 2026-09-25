package categoryService

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Krokozabra213/e-commerce_shop/infra/apperror"
	"github.com/Krokozabra213/e-commerce_shop/services/product-service/internal/domain"
)

type CategoryWriteRepository interface {
	Create(ctx context.Context, category *domain.Category) (string, error)
	Update(ctx context.Context, slug string, input domain.UpdateCategoryInput) (*domain.Category, error)
	Delete(ctx context.Context, slug string) error
}

type CategoryReadRepository interface {
	GetBySlug(ctx context.Context, slug string) (*domain.Category, error)
	List(ctx context.Context) ([]domain.Category, error)
}

type ProductReadRepository interface {
	CountByCategory(ctx context.Context, categorySlug string) (int64, error)
}

type Service struct {
	writeRepo   CategoryWriteRepository
	readRepo    CategoryReadRepository
	productRepo ProductReadRepository
}

func NewService(
	writeRepo CategoryWriteRepository,
	readRepo CategoryReadRepository,
	productRepo ProductReadRepository,
) *Service {
	return &Service{
		writeRepo:   writeRepo,
		readRepo:    readRepo,
		productRepo: productRepo,
	}
}

func (s *Service) Create(ctx context.Context, input domain.CreateCategoryInput) (*domain.Category, error) {
	now := time.Now()
	category := &domain.Category{
		Name:        input.Name,
		Slug:        input.Slug,
		Description: input.Description,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	id, err := s.writeRepo.Create(ctx, category)
	if err != nil {
		if errors.Is(err, domain.ErrAlreadyExists) {
			return nil, apperror.NewBusiness(apperror.CodeAlreadyExists, "Такая категория уже существует")
		}
		return nil, apperror.NewInternal("writeRepo.Create", err, "Что-то пошло не так", nil)
	}

	category.ID = id

	return category, nil
}

func (s *Service) GetBySlug(ctx context.Context, slug string) (*domain.Category, error) {
	category, err := s.readRepo.GetBySlug(ctx, slug)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, apperror.NewBusiness(apperror.CodeNotFound, "category not found")
		}
		return nil, apperror.NewInternal("readRepo.GetBySlug", err, "Что-то пошло не так", nil)
	}

	return category, nil
}

func (s *Service) List(ctx context.Context) ([]domain.Category, error) {
	categories, err := s.readRepo.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list categories: %w", err)
	}

	return categories, nil
}

func (s *Service) Update(ctx context.Context, slug string, input domain.UpdateCategoryInput) (*domain.Category, error) {
	updated, err := s.writeRepo.Update(ctx, slug, input)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, apperror.NewBusiness(apperror.CodeNotFound, "category not found")
		}
		return nil, apperror.NewInternal("writeRepo.Update", err, "Что-то пошло не так", nil)
	}

	return updated, nil
}

func (s *Service) Delete(ctx context.Context, slug string) error {
	count, err := s.productRepo.CountByCategory(ctx, slug)
	if err != nil {
		return apperror.NewInternal("productRepo.CountByCategory", err, "failed to count products", nil)
	}

	if count > 0 {
		return apperror.NewBusiness(
			apperror.CodeConflict,
			fmt.Sprintf("category has %d products", count),
		)
	}

	if err := s.writeRepo.Delete(ctx, slug); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return apperror.NewBusiness(apperror.CodeNotFound, "category not found")
		}
		return apperror.NewInternal("writeRepo.Delete", err, "failed to delete category", nil)
	}

	return nil
}
