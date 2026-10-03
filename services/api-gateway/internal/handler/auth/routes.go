package auth

import "github.com/gofiber/fiber/v3"

func (h *Handler) RegisterRoutes(router fiber.Router, jwtMW fiber.Handler) {
	auth := router.Group("/api/v1/auth")

	auth.Post("/register", h.Register)
	auth.Post("/login", h.Login)
	auth.Post("/refresh", h.Refresh)
	auth.Get("/verify-email", h.VerifyEmail)

	auth.Post("/logout", jwtMW, h.Logout)
	auth.Post("/resend-verification", jwtMW, h.SendVerificationEmail)

	oauth := router.Group("/api/v1/auth/oauth")
	oauth.Get("/:provider/login", h.OAuthLogin)
	oauth.Get("/:provider/callback", h.OAuthCallback)
}
