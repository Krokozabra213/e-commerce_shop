package httphandler

import (
	infradomain "github.com/Krokozabra213/e-commerce_shop/infra/domain"
	inframiddleware "github.com/Krokozabra213/e-commerce_shop/infra/middleware"
	"github.com/gofiber/fiber/v3"
)

func (h *StockHandler) SetupStockRoutes(api fiber.Router) {
	inventory := api.Group("/api/v1/inventory")
	inventory.Get("/:productId", h.GetStock)

	authUserMW := inframiddleware.NewAuthUserMiddleware()
	authRolesMW := inframiddleware.NewAuthRolesMiddleware()
	ManagerMinMW := inframiddleware.NewRequireRolesMiddleware(infradomain.RoleManager)

	inventory.Post("/", authUserMW, authRolesMW, ManagerMinMW, h.CreateStock)
	inventory.Post("/:productId/add", authUserMW, authRolesMW, ManagerMinMW, h.AddStock)
}
