package service

import (
	"context"
	"log/slog"

	"github.com/Krokozabra213/e-commerce_shop/services/user-service/internal/domain"
	"github.com/google/uuid"
)

type UserRepository interface {
	Create(ctx context.Context, user *domain.User) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error)
	List(ctx context.Context, filter domain.ListUsersFilter) ([]*domain.User, int, error)
	UpdateProfile(ctx context.Context, id uuid.UUID, input domain.UpdateProfileInput) (*domain.User, error)
	SoftDelete(ctx context.Context, id uuid.UUID) error
	AddRole(ctx context.Context, userID uuid.UUID, role domain.Role) error
	RemoveRole(ctx context.Context, userID uuid.UUID, role domain.Role) error
	GetRoles(ctx context.Context, userID uuid.UUID) ([]domain.Role, error)
}

type TxManager interface {
	WithinTransaction(ctx context.Context, fn func(ctx context.Context) error) error
}

type Service struct {
	repo      UserRepository
	txManager TxManager
	logger    *slog.Logger
}

func NewService(
	repo UserRepository,
	logger *slog.Logger,
	txManager TxManager,
) *Service {
	return &Service{
		repo:      repo,
		logger:    logger,
		txManager: txManager,
	}
}
