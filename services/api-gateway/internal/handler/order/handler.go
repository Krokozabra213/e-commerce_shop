package order

import (
	"context"
	"errors"

	"github.com/Krokozabra213/e-commerce_shop/infra/apperror"
	httpclient "github.com/Krokozabra213/e-commerce_shop/infra/clients/http"
	"github.com/Krokozabra213/e-commerce_shop/infra/httpx"
	"github.com/Krokozabra213/e-commerce_shop/services/api-gateway/internal/middleware"
	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

type OrderClient interface {
	Create(ctx context.Context, auth *httpclient.AuthContext, idempotencyKey uuid.UUID, items []httpclient.OrderItem) (*httpclient.CreateOrderResponse, error)
	GetByID(ctx context.Context, auth *httpclient.AuthContext, orderID uuid.UUID) (*httpclient.OrderResponse, error)
	GetList(ctx context.Context, auth *httpclient.AuthContext) ([]httpclient.OrderResponse, error)
	Ship(ctx context.Context, auth *httpclient.AuthContext, orderID uuid.UUID) error
	Complete(ctx context.Context, auth *httpclient.AuthContext, orderID uuid.UUID) error
	PaymentSuccess(ctx context.Context, auth *httpclient.AuthContext, orderID, eventID, correlationID uuid.UUID) error
	PaymentFailed(ctx context.Context, auth *httpclient.AuthContext, orderID, eventID, correlationID uuid.UUID, reason string) error
}

type Handler struct {
	client OrderClient
}

func NewHandler(client OrderClient) *Handler {
	return &Handler{client: client}
}

func (h *Handler) Create(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok {
		return apperror.NewBusiness(apperror.CodeUnauthorized, "unauthorized")
	}

	idempotencyKeyStr := c.Get(httpx.HeaderIdempotencyKey)
	if idempotencyKeyStr == "" {
		return apperror.NewBusiness(apperror.CodeBadRequest, "idempotency key is required")
	}

	idempotencyKey, err := uuid.Parse(idempotencyKeyStr)
	if err != nil {
		return apperror.NewBusiness(apperror.CodeBadRequest, "invalid idempotency key format")
	}

	req := new(createOrderRequest)

	if err := c.Bind().Body(req); err != nil {
		var valErrs validator.ValidationErrors
		if errors.As(err, &valErrs) {
			return apperror.NewBusiness(apperror.CodeValidation, "Проверьте корректность данных")
		}
		return apperror.NewBusiness(apperror.CodeBadRequest, "Неверный формат запроса")
	}

	resp, err := h.client.Create(c.Context(), &httpclient.AuthContext{
		UserID: userID,
	}, idempotencyKey, req.Items)
	if err != nil {
		return err
	}

	return c.Status(fiber.StatusCreated).JSON(resp)
}

func (h *Handler) GetByID(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok {
		return apperror.NewBusiness(apperror.CodeUnauthorized, "unauthorized")
	}

	roles, ok := middleware.UserRolesFromCtx(c)
	if !ok {
		return apperror.NewBusiness(apperror.CodeUnauthorized, "missing user roles")
	}

	orderID, err := parseUUIDParam(c, "id")
	if err != nil {
		return err
	}

	resp, err := h.client.GetByID(c.Context(), &httpclient.AuthContext{
		UserID: userID,
		Roles:  roles,
	}, orderID)
	if err != nil {
		return err
	}

	return c.Status(fiber.StatusOK).JSON(resp)
}

func (h *Handler) GetList(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok {
		return apperror.NewBusiness(apperror.CodeUnauthorized, "unauthorized")
	}

	resp, err := h.client.GetList(c.Context(), &httpclient.AuthContext{
		UserID: userID,
	})
	if err != nil {
		return err
	}

	return c.Status(fiber.StatusOK).JSON(resp)
}

func (h *Handler) Ship(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok {
		return apperror.NewBusiness(apperror.CodeUnauthorized, "unauthorized")
	}

	roles, ok := middleware.UserRolesFromCtx(c)
	if !ok {
		return apperror.NewBusiness(apperror.CodeUnauthorized, "missing user roles")
	}

	orderID, err := parseUUIDParam(c, "id")
	if err != nil {
		return err
	}

	if err := h.client.Ship(c.Context(), &httpclient.AuthContext{
		UserID: userID,
		Roles:  roles,
	}, orderID); err != nil {
		return err
	}

	return c.SendStatus(fiber.StatusNoContent)
}

func (h *Handler) Complete(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok {
		return apperror.NewBusiness(apperror.CodeUnauthorized, "unauthorized")
	}

	roles, ok := middleware.UserRolesFromCtx(c)
	if !ok {
		return apperror.NewBusiness(apperror.CodeUnauthorized, "missing user roles")
	}

	orderID, err := parseUUIDParam(c, "id")
	if err != nil {
		return err
	}

	if err := h.client.Complete(c.Context(), &httpclient.AuthContext{
		UserID: userID,
		Roles:  roles,
	}, orderID); err != nil {
		return err
	}

	return c.SendStatus(fiber.StatusNoContent)
}

func (h *Handler) PaymentSuccess(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok {
		return apperror.NewBusiness(apperror.CodeUnauthorized, "unauthorized")
	}

	roles, ok := middleware.UserRolesFromCtx(c)
	if !ok {
		return apperror.NewBusiness(apperror.CodeUnauthorized, "missing user roles")
	}

	orderID, err := parseUUIDParam(c, "id")
	if err != nil {
		return err
	}

	req := new(paymentSuccessRequest)

	if err := c.Bind().Body(req); err != nil {
		var valErrs validator.ValidationErrors
		if errors.As(err, &valErrs) {
			return apperror.NewBusiness(apperror.CodeValidation, "Проверьте корректность данных")
		}
		return apperror.NewBusiness(apperror.CodeBadRequest, "Неверный формат запроса")
	}

	if err := h.client.PaymentSuccess(c.Context(), &httpclient.AuthContext{
		UserID: userID,
		Roles:  roles,
	}, orderID, req.EventID, req.CorrelationID); err != nil {
		return err
	}

	return c.SendStatus(fiber.StatusNoContent)
}

func (h *Handler) PaymentFailed(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok {
		return apperror.NewBusiness(apperror.CodeUnauthorized, "unauthorized")
	}

	roles, ok := middleware.UserRolesFromCtx(c)
	if !ok {
		return apperror.NewBusiness(apperror.CodeUnauthorized, "missing user roles")
	}

	orderID, err := parseUUIDParam(c, "id")
	if err != nil {
		return err
	}

	req := new(paymentFailedRequest)

	if err := c.Bind().Body(req); err != nil {
		var valErrs validator.ValidationErrors
		if errors.As(err, &valErrs) {
			return apperror.NewBusiness(apperror.CodeValidation, "Проверьте корректность данных")
		}
		return apperror.NewBusiness(apperror.CodeBadRequest, "Неверный формат запроса")
	}

	if err := h.client.PaymentFailed(c.Context(), &httpclient.AuthContext{
		UserID: userID,
		Roles:  roles,
	}, orderID, req.EventID, req.CorrelationID, req.Reason); err != nil {
		return err
	}

	return c.SendStatus(fiber.StatusNoContent)
}

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
