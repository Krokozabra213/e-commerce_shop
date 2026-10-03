package inventory

import "github.com/gofiber/fiber/v3"

func (h *Handler) RegisterRoutes(router fiber.Router, jwtMW, rolesMW fiber.Handler) {
	inventory := router.Group("/api/v1/inventory")

	inventory.Get("/:productId", h.GetStock)

	inventory.Post("/", jwtMW, rolesMW, h.CreateStock)
	inventory.Post("/:productId/add", jwtMW, rolesMW, h.AddStock)
}
