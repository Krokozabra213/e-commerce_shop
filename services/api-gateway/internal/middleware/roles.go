package middleware

import (
	"context"
	"fmt"

	httpclient "github.com/Krokozabra213/e-commerce_shop/infra/clients/http"
	"github.com/gofiber/fiber/v3"
)

type UserRolesClient interface {
	GetMyRoles(ctx context.Context, auth *httpclient.AuthContext) (*httpclient.RolesResponse, error)
}

func NewRolesMiddleware(client UserRolesClient) fiber.Handler {
	return func(c fiber.Ctx) error {
		userID, ok := UserIDFromCtx(c)
		if !ok {
			return fiber.NewError(fiber.StatusUnauthorized, "missing user id in context")
		}

		resp, err := client.GetMyRoles(c.Context(), &httpclient.AuthContext{
			UserID: userID,
		})
		if err != nil {
			return fmt.Errorf("roles middleware: %w", err)
		}

		c.Locals(ctxKeyUserRoles, resp.Roles)

		return c.Next()
	}
}

func UserRolesFromCtx(c fiber.Ctx) ([]string, bool) {
	v, ok := c.Locals(ctxKeyUserRoles).([]string)
	if !ok {
		return nil, false
	}
	return v, true
}
