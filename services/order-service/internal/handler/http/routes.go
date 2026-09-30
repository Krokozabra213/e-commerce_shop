package httphandler

import (
	infradomain "github.com/Krokozabra213/e-commerce_shop/infra/domain"
	inframiddleware "github.com/Krokozabra213/e-commerce_shop/infra/middleware"
	"github.com/gofiber/fiber/v3"
)

func (h *OrderHandler) SetupOrderRoutes(api fiber.Router) {
	orders := api.Group("/api/v1/orders")

	authUserMW := inframiddleware.NewAuthUserMiddleware()
	authRolesMW := inframiddleware.NewAuthRolesMiddleware()
	managerMinMW := inframiddleware.NewRequireRolesMiddleware(infradomain.RoleManager)

	orders.Post("/", authUserMW, authRolesMW, h.Create)
	orders.Get("/", authUserMW, authRolesMW, h.GetList)
	orders.Get("/:id", authUserMW, authRolesMW, h.GetByID)

	orders.Post("/:id/ship", authUserMW, authRolesMW, managerMinMW, h.Ship)
	orders.Post("/:id/complete", authUserMW, authRolesMW, managerMinMW, h.Complete)

	orders.Post("/:id/payment/success", authUserMW, authRolesMW, managerMinMW, h.PaymentSuccess)
	orders.Post("/:id/payment/failed", authUserMW, authRolesMW, managerMinMW, h.PaymentFailed)
}
