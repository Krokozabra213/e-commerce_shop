package auth

import (
	"time"

	"github.com/Krokozabra213/e-commerce_shop/infra/httpx"
	"github.com/gofiber/fiber/v3"
)

func setRefreshTokenCookie(c fiber.Ctx, token string, ttl time.Duration) {
	c.Cookie(&fiber.Cookie{
		Name:     httpx.CookieRefreshToken,
		Value:    token,
		HTTPOnly: true,
		// todo: поменять secure на true после тестов
		Secure:   false,
		SameSite: "Strict",
		MaxAge:   int(ttl.Seconds()),
		Path:     "/api/v1/auth",
	})
}

func clearRefreshTokenCookie(c fiber.Ctx) {
	c.Cookie(&fiber.Cookie{
		Name:     httpx.CookieRefreshToken,
		Value:    "",
		HTTPOnly: true,
		// todo: поменять secure на true после тестов
		Secure:   false,
		SameSite: "Strict",
		MaxAge:   -1,
		Path:     "/api/v1/auth",
	})
}
