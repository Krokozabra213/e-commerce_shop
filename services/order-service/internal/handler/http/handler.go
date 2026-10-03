package httphandler

import (
	"context"

	"github.com/Krokozabra213/e-commerce_shop/infra/apperror"
	infradomain "github.com/Krokozabra213/e-commerce_shop/infra/domain"
	"github.com/Krokozabra213/e-commerce_shop/infra/httpx"
	inframiddleware "github.com/Krokozabra213/e-commerce_shop/infra/middleware"
	"github.com/Krokozabra213/e-commerce_shop/services/order-service/internal/domain"
	svcDTO "github.com/Krokozabra213/e-commerce_shop/services/order-service/internal/service/dto"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

type OrderService interface {
	CreateOrder(ctx context.Context, input svcDTO.CreateOrderInput) (*svcDTO.CreateOrderOutput, error)
	GetOrderForUser(ctx context.Context, orderID uuid.UUID, currentUserID uuid.UUID, isAdmin bool) (*domain.Order, error)
	GetUserOrders(ctx context.Context, currentUserID uuid.UUID) ([]domain.Order, error)

	// Админ-действия
	ShipOrder(ctx context.Context, id uuid.UUID) error
	CompleteOrder(ctx context.Context, id uuid.UUID) error
	HandlePaymentSucceeded(ctx context.Context, input svcDTO.PaymentSucceededInput) error // MVP: синхронно
	HandlePaymentFailed(ctx context.Context, input svcDTO.PaymentFailedInput) error       // MVP: синхронно
}

type OrderHandler struct {
	svc OrderService
}

func NewOrderHandler(svc OrderService) *OrderHandler {
	return &OrderHandler{svc: svc}
}

func (h *OrderHandler) Create(c fiber.Ctx) error {
	userID, ok := inframiddleware.UserIDFromCtx(c)
	if !ok {
		return apperror.NewBusiness(apperror.CodeUnauthorized, "Не авторизан")
	}

	idempotencyKeyStr := c.Get(httpx.HeaderIdempotencyKey)
	if idempotencyKeyStr == "" {
		return apperror.NewBusiness(apperror.CodeBadRequest, "idempotency key is required")
	}

	idempotencyKey, err := uuid.Parse(idempotencyKeyStr)
	if err != nil {
		return apperror.NewBusiness(apperror.CodeBadRequest, "idempotency key is invalid")
	}

	var req createOrderRequest

	if err := c.Bind().JSON(&req); err != nil {
		return apperror.NewBusiness(apperror.CodeBadRequest, "invalid request body")
	}

	items := make([]svcDTO.OrderItemInput, len(req.Items))
	for i, item := range req.Items {
		items[i] = svcDTO.OrderItemInput{
			ProductID: item.ProductID,
			Quantity:  item.Quantity,
		}
	}

	input := svcDTO.CreateOrderInput{
		UserID:         userID,
		IdempotencyKey: idempotencyKey,
		Items:          items,
	}

	output, err := h.svc.CreateOrder(c.Context(), input)
	if err != nil {
		return err
	}

	resp := toCreateOrderResponse(output)

	return c.Status(fiber.StatusCreated).JSON(resp)
}

func (h *OrderHandler) GetByID(c fiber.Ctx) error {
	ctx := c.Context()

	orderID, err := parseUUIDParam(c, "id")
	if err != nil {
		return apperror.NewBusiness(apperror.CodeBadRequest, "invalid id")
	}

	userID, ok := inframiddleware.UserIDFromCtx(c)
	if !ok {
		return apperror.NewBusiness(apperror.CodeUnauthorized, "Не авторизан")
	}

	roles, _ := inframiddleware.UserRolesFromCtx(c)
	isAdmin := inframiddleware.HasMinRole(roles, infradomain.RoleAdmin)

	order, err := h.svc.GetOrderForUser(ctx, orderID, userID, isAdmin)
	if err != nil {
		return err
	}

	resp := toOrderResponse(order)
	return c.Status(fiber.StatusOK).JSON(resp)
}

func (h *OrderHandler) GetList(c fiber.Ctx) error {
	ctx := c.Context()
	userID, ok := inframiddleware.UserIDFromCtx(c)
	if !ok {
		return apperror.NewBusiness(apperror.CodeUnauthorized, "Не авторизан")
	}

	orders, err := h.svc.GetUserOrders(ctx, userID)
	if err != nil {
		return err
	}

	resp := toOrderResponses(orders)

	return c.Status(fiber.StatusOK).JSON(resp)
}

func (h *OrderHandler) Ship(c fiber.Ctx) error {
	ctx := c.Context()
	orderID, err := parseUUIDParam(c, "id")
	if err != nil {
		return apperror.NewBusiness(apperror.CodeBadRequest, "invalid id")
	}
	err = h.svc.ShipOrder(ctx, orderID)
	if err != nil {
		return err
	}

	return c.SendStatus(fiber.StatusNoContent)
}

func (h *OrderHandler) Complete(c fiber.Ctx) error {
	ctx := c.Context()
	orderID, err := parseUUIDParam(c, "id")
	if err != nil {
		return apperror.NewBusiness(apperror.CodeBadRequest, "invalid id")
	}
	err = h.svc.CompleteOrder(ctx, orderID)
	if err != nil {
		return err
	}

	return c.SendStatus(fiber.StatusNoContent)
}

func (h *OrderHandler) PaymentSuccess(c fiber.Ctx) error {
	ctx := c.Context()
	orderID, err := parseUUIDParam(c, "id")
	if err != nil {
		return apperror.NewBusiness(apperror.CodeBadRequest, "invalid id")
	}

	var req paymentSucceededRequest

	if err := c.Bind().JSON(&req); err != nil {
		return apperror.NewBusiness(apperror.CodeBadRequest, "invalid request body")
	}

	input := svcDTO.PaymentSucceededInput{
		EventID:       req.EventID,
		CorrelationID: req.CorrelationID,
		OrderID:       orderID,
	}

	err = h.svc.HandlePaymentSucceeded(ctx, input)
	if err != nil {
		return err
	}

	return c.SendStatus(fiber.StatusNoContent)
}

func (h *OrderHandler) PaymentFailed(c fiber.Ctx) error {
	ctx := c.Context()
	orderID, err := parseUUIDParam(c, "id")
	if err != nil {
		return apperror.NewBusiness(apperror.CodeBadRequest, "invalid id")
	}

	var req paymentFailedRequest

	if err := c.Bind().JSON(&req); err != nil {
		return apperror.NewBusiness(apperror.CodeBadRequest, "invalid request body")
	}

	input := svcDTO.PaymentFailedInput{
		EventID:       req.EventID,
		CorrelationID: req.CorrelationID,
		OrderID:       orderID,
		Reason:        req.Reason,
	}

	err = h.svc.HandlePaymentFailed(ctx, input)
	if err != nil {
		return err
	}

	return c.SendStatus(fiber.StatusNoContent)
}
