package user

import "github.com/gofiber/fiber/v3"

func (h *Handler) RegisterRoutes(router fiber.Router, jwtMW, rolesMW fiber.Handler) {
	users := router.Group("/api/v1/users")

	users.Get("/me", jwtMW, h.GetMyProfile)
	users.Patch("/me", jwtMW, h.UpdateMyProfile)

	users.Get("/:id", jwtMW, rolesMW, h.GetUserByID)
	users.Get("/", jwtMW, rolesMW, h.ListUsers)
	users.Delete("/:id", jwtMW, rolesMW, h.DeleteUser)
	users.Post("/:id/roles/:role", jwtMW, rolesMW, h.AddRole)
	users.Delete("/:id/roles/:role", jwtMW, rolesMW, h.RemoveRole)
}
