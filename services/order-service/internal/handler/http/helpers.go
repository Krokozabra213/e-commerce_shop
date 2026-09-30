package httphandler

import (
	"github.com/Krokozabra213/e-commerce_shop/infra/apperror"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

func parseUUIDParam(c fiber.Ctx, name string) (uuid.UUID, error) {
	raw := c.Params(name)
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, apperror.NewBusiness(
			apperror.CodeBadRequest,
			"invalid "+name+" format",
		)
	}
	return id, nil
}
