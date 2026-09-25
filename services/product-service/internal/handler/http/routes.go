package httphandler

import (
	infradomain "github.com/Krokozabra213/e-commerce_shop/infra/domain"
	inframiddleware "github.com/Krokozabra213/e-commerce_shop/infra/middleware"
	"github.com/gofiber/fiber/v3"
)

func (h *Handler) RegisterRoutes(router fiber.Router) {
	api := router.Group("/api/v1")

	api.Get("/products", h.ListProductsAdapter)
	api.Get("/products/:id", h.GetProductAdapter)

	api.Get("/categories", h.ListCategories)
	api.Get("/categories/:slug", h.GetCategoryAdapter)

	authUserMW := inframiddleware.NewAuthUserMiddleware()
	authRolesMW := inframiddleware.NewAuthRolesMiddleware()
	ManagerMinMW := inframiddleware.NewRequireRolesMiddleware(infradomain.RoleManager)
	AdminMinMW := inframiddleware.NewRequireRolesMiddleware(infradomain.RoleAdmin)

	api.Post("/products", authUserMW, authRolesMW, ManagerMinMW, h.CreateProduct)
	api.Put("/products/:id", authUserMW, authRolesMW, ManagerMinMW, h.UpdateProductAdapter)
	api.Post("/products/:id/publish", authUserMW, authRolesMW, ManagerMinMW, h.TogglePublishAdapter)

	api.Delete("/products/:id", authUserMW, authRolesMW, AdminMinMW, h.DeleteProductAdapter)

	api.Post("/categories", authUserMW, authRolesMW, AdminMinMW, h.CreateCategory)
	api.Put("/categories/:slug", authUserMW, authRolesMW, AdminMinMW, h.UpdateCategoryAdapter)
	api.Delete("/categories/:slug", authUserMW, authRolesMW, AdminMinMW, h.DeleteCategoryAdapter)
}
