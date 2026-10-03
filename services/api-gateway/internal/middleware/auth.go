package middleware

import (
	"errors"
	"strings"

	"github.com/Krokozabra213/e-commerce_shop/infra/httpx"
	jwtmanager "github.com/Krokozabra213/e-commerce_shop/infra/jwt/manager"
	jwtvalidator "github.com/Krokozabra213/e-commerce_shop/infra/jwt/validator"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

type JWTValidator interface {
	ValidateAccess(tokenString string) (*jwtmanager.AccessClaims, error)
}

type ctxKey int

const (
	ctxKeyUserID ctxKey = iota
	ctxKeyUserRoles
)

func NewJWTMiddleware(validator JWTValidator) fiber.Handler {
	return func(c fiber.Ctx) error {
		accessToken := extractBearerToken(c)
		if accessToken == "" {
			return fiber.NewError(fiber.StatusUnauthorized, "missing access token")
		}

		claims, err := validator.ValidateAccess(accessToken)
		if err != nil {
			if errors.Is(err, jwtvalidator.ErrParseJWT) {
				return fiber.NewError(fiber.StatusUnauthorized, "invalid or expired access token")
			}
			return fiber.NewError(fiber.StatusUnauthorized, "failed to validate access token")
		}

		userID, err := claims.UserIDUUID()
		if err != nil {
			return fiber.NewError(fiber.StatusUnauthorized, "invalid user id in token")
		}

		c.Locals(ctxKeyUserID, userID)

		return c.Next()
	}
}

func extractBearerToken(c fiber.Ctx) string {
	header := c.Get(fiber.HeaderAuthorization)
	if header == "" {
		return ""
	}
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], httpx.AuthSchemeBearer) {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

func UserIDFromCtx(c fiber.Ctx) (uuid.UUID, bool) {
	v, ok := c.Locals(ctxKeyUserID).(uuid.UUID)
	if !ok {
		return uuid.Nil, false
	}
	return v, true
}
