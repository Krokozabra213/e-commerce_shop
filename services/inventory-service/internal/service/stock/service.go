package stockService

import (
	"context"
	"errors"
	"time"

	"github.com/Krokozabra213/e-commerce_shop/infra/apperror"
	"github.com/Krokozabra213/e-commerce_shop/services/inventory-service/internal/domain"
	"github.com/Krokozabra213/e-commerce_shop/services/inventory-service/internal/service"
)

//go:generate mockgen -source=service.go -destination=mocks/mock_service.go -package=mocks -typed

type StockRepository interface {
	Create(ctx context.Context, stock *domain.Stock) error
	GetByProductID(ctx context.Context, productID string) (*domain.Stock, error)
	IncreaseQuantity(ctx context.Context, productID string, quantity int) error
	GetByProductIDs(ctx context.Context, productIDs []string) (map[string]int, error)
}

type OutboxRepository interface {
	Create(ctx context.Context, event *domain.OutboxEvent) error
}

type TxManager interface {
	WithinTransaction(ctx context.Context, fn func(ctx context.Context) error) error
}

type StockService struct {
	stockRepo  StockRepository
	outboxRepo OutboxRepository
	txManager  TxManager
}

func NewStockService(
	stockRepo StockRepository,
	outboxRepo OutboxRepository,
	txManager TxManager,
) *StockService {
	return &StockService{
		stockRepo:  stockRepo,
		outboxRepo: outboxRepo,
		txManager:  txManager,
	}
}

func (s *StockService) Create(ctx context.Context, input service.ProductItem) error {
	now := time.Now()
	stock := &domain.Stock{
		ProductID:         input.ProductID,
		AvailableQuantity: input.Quantity,
		CreatedAt:         now,
		UpdatedAt:         now,
	}

	if err := s.stockRepo.Create(ctx, stock); err != nil {
		if errors.Is(err, domain.ErrAlreadyExists) {
			return apperror.NewBusiness(apperror.CodeConflict, "Запись о товаре уже существует")
		}
		return apperror.NewInternal("stockRepo.Create", err, "Что-то пошло не так", nil)
	}

	return nil
}

func (s *StockService) GetByProductID(ctx context.Context, productID string) (*domain.Stock, error) {
	stock, err := s.stockRepo.GetByProductID(ctx, productID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, apperror.NewBusiness(apperror.CodeNotFound, "Запись о товаре не найдена")
		}
		return nil, apperror.NewInternal("stockRepo.GetByProductID", err, "Что-то пошло не так", nil)
	}

	return stock, nil
}

func (s *StockService) GetQuantities(ctx context.Context, productIDs []string) (map[string]int, error) {
	quantities, err := s.stockRepo.GetByProductIDs(ctx, productIDs)
	if err != nil {
		return nil, apperror.NewInternal("stockRepo.GetByProductIDs", err, "Что-то пошло не так", nil)
	}

	return quantities, nil
}

func (s *StockService) AddStock(ctx context.Context, input service.ProductItem) error {
	if err := s.stockRepo.IncreaseQuantity(ctx, input.ProductID, input.Quantity); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return apperror.NewBusiness(apperror.CodeNotFound, "Запись о товаре не найдена")
		}
		return apperror.NewInternal("stockRepo.IncreaseQuantity", err, "Что-то пошло не так", nil)
	}

	return nil
}
