package authhandler

import "github.com/gofiber/fiber/v3"

type GetPublicKeyResponse struct {
	PublicKey string `json:"public_key"`
}

func (h *Handler) GetPublicKey(c fiber.Ctx) error {
	result, err := h.authService.GetPublicKey(c.Context())
	if err != nil {
		return err
	}

	return c.Status(fiber.StatusOK).JSON(GetPublicKeyResponse{
		PublicKey: result.PublicKey,
	})
}
