package inframiddleware

import (
	"fmt"
	"strings"

	"github.com/Krokozabra213/e-commerce_shop/infra/apperror"
	infradomain "github.com/Krokozabra213/e-commerce_shop/infra/domain"
	"github.com/Krokozabra213/e-commerce_shop/infra/httpx"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

type ctxKey int

const (
	ctxKeyUserID ctxKey = iota
	ctxKeyUserRoles
)

func UserIDFromCtx(c fiber.Ctx) (uuid.UUID, bool) {
	id, ok := c.Locals(ctxKeyUserID).(uuid.UUID)
	return id, ok
}

func UserRolesFromCtx(c fiber.Ctx) ([]infradomain.Role, bool) {
	roles, ok := c.Locals(ctxKeyUserRoles).([]infradomain.Role)
	if !ok {
		return nil, false
	}
	return roles, true
}

func NewAuthUserMiddleware() fiber.Handler {
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

func NewAuthRolesMiddleware() fiber.Handler {
	return func(c fiber.Ctx) error {
		rolesStr := c.Get(httpx.HeaderUserRoles)
		if rolesStr == "" {
			return apperror.NewBusiness(apperror.CodeUnauthorized, "unauthorized: missing user roles")
		}

		rolesSliceString := strings.Split(rolesStr, ",")
		rolesSlice := make([]infradomain.Role, 0, len(rolesSliceString))

		for _, role := range rolesSliceString {
			roleTrimmed := strings.TrimSpace(role)
			if roleTrimmed == "" {
				continue
			}

			roleDomain := infradomain.Role(roleTrimmed)
			if !roleDomain.IsValid() {
				return apperror.NewBusiness(
					apperror.CodeUnauthorized,
					fmt.Sprintf("unauthorized: invalid role %q", roleTrimmed),
				)
			}
			rolesSlice = append(rolesSlice, roleDomain)
		}

		if len(rolesSlice) == 0 {
			return apperror.NewBusiness(apperror.CodeUnauthorized, "unauthorized: no valid roles found")
		}

		c.Locals(ctxKeyUserRoles, rolesSlice)

		return c.Next()
	}
}

func NewRequireRolesMiddleware(minRole infradomain.Role) fiber.Handler {
	return func(c fiber.Ctx) error {
		userRoles, ok := UserRolesFromCtx(c)
		if !ok {
			return apperror.NewBusiness(apperror.CodeUnauthorized, "unauthorized: roles not found in context")
		}

		if !hasMinRole(userRoles, minRole) {
			return apperror.NewBusiness(apperror.CodeForbidden, "forbidden: insufficient permissions")
		}

		return c.Next()
	}
}

func hasMinRole(userRoles []infradomain.Role, minRole infradomain.Role) bool {
	minWeight := minRole.Weight()
	for _, r := range userRoles {
		if r.Weight() >= minWeight {
			return true
		}
	}
	return false
}

func HasMinRole(userRoles []infradomain.Role, minRole infradomain.Role) bool {
	minWeight := minRole.Weight()
	for _, r := range userRoles {
		if r.Weight() >= minWeight {
			return true
		}
	}
	return false
}
