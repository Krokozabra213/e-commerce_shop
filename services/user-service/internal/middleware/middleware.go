package middleware

import (
	"context"

	"github.com/Krokozabra213/e-commerce_shop/infra/apperror"
	"github.com/Krokozabra213/e-commerce_shop/infra/httpx"
	"github.com/Krokozabra213/e-commerce_shop/services/user-service/internal/domain"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

type RoleChecker interface {
	GetRoles(ctx context.Context, userID uuid.UUID) ([]domain.Role, error)
}

type ctxKey int

const (
	ctxKeyUserID ctxKey = iota
)

func UserIDFromCtx(c fiber.Ctx) (uuid.UUID, bool) {
	id, ok := c.Locals(ctxKeyUserID).(uuid.UUID)
	return id, ok
}

func NewAuthMiddleware() fiber.Handler {
	return func(c fiber.Ctx) error {
		userIDStr := c.Get(httpx.HeaderUserID)
		if userIDStr == "" {
			return apperror.NewBusiness(apperror.CodeUnauthorized, "unauthorized: missing user id")
		}

		userID, err := uuid.Parse(userIDStr)
		if err != nil {
			return apperror.NewBusiness(apperror.CodeUnauthorized, "unauthorized: invalid user id format")
		}
		c.Locals(ctxKeyUserID, userID)

		return c.Next()
	}
}

func NewRequireRolesMiddleware(
	checker RoleChecker,
	minRole domain.Role,
) fiber.Handler {
	return func(c fiber.Ctx) error {
		userID, ok := c.Locals(ctxKeyUserID).(uuid.UUID)
		if !ok {
			return apperror.NewBusiness(apperror.CodeUnauthorized, "unauthorized")
		}

		userRoles, err := checker.GetRoles(c.Context(), userID)
		if err != nil {
			return apperror.NewInternal("checker.GetRoles", err, "Что-то пошло не так", nil)
		}

		if !hasMinRole(userRoles, minRole) {
			return apperror.NewBusiness(apperror.CodeForbidden, "forbidden: insufficient permissions")
		}

		return c.Next()
	}
}

func hasMinRole(userRoles []domain.Role, minRole domain.Role) bool {
	minWeight := minRole.Weight()
	for _, r := range userRoles {
		if r.Weight() >= minWeight {
			return true
		}
	}
	return false
}
