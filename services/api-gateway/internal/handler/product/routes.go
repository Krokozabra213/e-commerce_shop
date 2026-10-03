package product

import "github.com/gofiber/fiber/v3"

func (h *Handler) RegisterRoutes(router fiber.Router, jwtMW, rolesMW fiber.Handler) {
	api := router.Group("/api/v1")

	api.Get("/products", h.ListProducts)
	api.Get("/products/:id", h.GetProduct)

	api.Post("/products", jwtMW, rolesMW, h.CreateProduct)
	api.Put("/products/:id", jwtMW, rolesMW, h.UpdateProduct)
	api.Post("/products/:id/publish", jwtMW, rolesMW, h.TogglePublish)

	api.Delete("/products/:id", jwtMW, rolesMW, h.DeleteProduct)

	api.Get("/categories", h.ListCategories)
	api.Get("/categories/:slug", h.GetCategory)

	api.Post("/categories", jwtMW, rolesMW, h.CreateCategory)
	api.Put("/categories/:slug", jwtMW, rolesMW, h.UpdateCategory)
	api.Delete("/categories/:slug", jwtMW, rolesMW, h.DeleteCategory)
}
