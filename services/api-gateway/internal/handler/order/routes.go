package order

import "github.com/gofiber/fiber/v3"

func (h *Handler) RegisterRoutes(router fiber.Router, jwtMW, rolesMW fiber.Handler) {
	orders := router.Group("/api/v1/orders")

	orders.Post("/", jwtMW, h.Create)
	orders.Get("/", jwtMW, h.GetList)
	orders.Get("/:id", jwtMW, rolesMW, h.GetByID)

	orders.Post("/:id/ship", jwtMW, rolesMW, h.Ship)
	orders.Post("/:id/complete", jwtMW, rolesMW, h.Complete)

	orders.Post("/:id/payment/success", jwtMW, rolesMW, h.PaymentSuccess)
	orders.Post("/:id/payment/failed", jwtMW, rolesMW, h.PaymentFailed)
}
