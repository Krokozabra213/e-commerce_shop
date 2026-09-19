package handler

import (
	"context"

	"github.com/Krokozabra213/e-commerce_shop/services/user-service/internal/domain"
	"github.com/Krokozabra213/e-commerce_shop/services/user-service/internal/middleware"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

type RoleChecker interface {
	GetRoles(ctx context.Context, userID uuid.UUID) ([]domain.Role, error)
}

func (h *Handler) RegisterRoutes(router fiber.Router, roles RoleChecker) {
	api := router.Group("/api/v1/users")

	authMW := middleware.NewAuthMiddleware()

	api.Get("/me", authMW, h.GetMyProfile)
	api.Patch("/me", authMW, h.UpdateMyProfile)

	managerMinMW := middleware.NewRequireRolesMiddleware(roles, domain.RoleManager)
	api.Get("/:id", authMW, managerMinMW, h.GetUserByID)
	api.Get("/", authMW, managerMinMW, h.ListUsers)

	adminMinMW := middleware.NewRequireRolesMiddleware(roles, domain.RoleAdmin)
	api.Delete("/:id", authMW, adminMinMW, h.DeleteUser)
	api.Post("/:id/roles/:role", authMW, adminMinMW, h.AddRole)
	api.Delete("/:id/roles/:role", authMW, adminMinMW, h.RemoveRole)
}
