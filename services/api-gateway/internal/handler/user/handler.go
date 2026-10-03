package user

import (
	"context"
	"errors"

	"github.com/Krokozabra213/e-commerce_shop/infra/apperror"
	httpclient "github.com/Krokozabra213/e-commerce_shop/infra/clients/http"
	"github.com/Krokozabra213/e-commerce_shop/services/api-gateway/internal/middleware"
	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

type UserClient interface {
	GetMyProfile(ctx context.Context, auth *httpclient.AuthContext) (*httpclient.UserResponse, error)
	UpdateMyProfile(ctx context.Context, auth *httpclient.AuthContext, input httpclient.UpdateProfileRequest) (*httpclient.UserResponse, error)
	GetUserByID(ctx context.Context, auth *httpclient.AuthContext, targetID uuid.UUID) (*httpclient.UserResponse, error)
	ListUsers(ctx context.Context, auth *httpclient.AuthContext, limit, offset int) (*httpclient.ListUsersResponse, error)
	DeleteUser(ctx context.Context, auth *httpclient.AuthContext, targetID uuid.UUID) error
	AddRole(ctx context.Context, auth *httpclient.AuthContext, targetID uuid.UUID, role string) error
	RemoveRole(ctx context.Context, auth *httpclient.AuthContext, targetID uuid.UUID, role string) error
}

type Handler struct {
	client UserClient
}

func NewHandler(client UserClient) *Handler {
	return &Handler{client: client}
}

func (h *Handler) GetMyProfile(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok {
		return apperror.NewBusiness(apperror.CodeUnauthorized, "unauthorized")
	}

	resp, err := h.client.GetMyProfile(c.Context(), &httpclient.AuthContext{
		UserID: userID,
	})
	if err != nil {
		return err
	}

	return c.Status(fiber.StatusOK).JSON(resp)
}

func (h *Handler) UpdateMyProfile(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok {
		return apperror.NewBusiness(apperror.CodeUnauthorized, "unauthorized")
	}

	req := new(updateProfileRequest)

	if err := c.Bind().Body(req); err != nil {
		var valErrs validator.ValidationErrors
		if errors.As(err, &valErrs) {
			return apperror.NewBusiness(apperror.CodeValidation, "Проверьте корректность данных")
		}
		return apperror.NewBusiness(apperror.CodeBadRequest, "Неверный формат запроса")
	}

	resp, err := h.client.UpdateMyProfile(c.Context(), &httpclient.AuthContext{
		UserID: userID,
	}, httpclient.UpdateProfileRequest{
		FirstName: req.FirstName,
		LastName:  req.LastName,
		Phone:     req.Phone,
		AvatarURL: req.AvatarURL,
	})
	if err != nil {
		return err
	}

	return c.Status(fiber.StatusOK).JSON(resp)
}

func (h *Handler) GetUserByID(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok {
		return apperror.NewBusiness(apperror.CodeUnauthorized, "unauthorized")
	}

	roles, ok := middleware.UserRolesFromCtx(c)
	if !ok {
		return apperror.NewBusiness(apperror.CodeUnauthorized, "missing user roles")
	}

	targetID, err := parseUUIDParam(c, "id")
	if err != nil {
		return err
	}

	resp, err := h.client.GetUserByID(c.Context(), &httpclient.AuthContext{
		UserID: userID,
		Roles:  roles,
	}, targetID)
	if err != nil {
		return err
	}

	return c.Status(fiber.StatusOK).JSON(resp)
}

func (h *Handler) ListUsers(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok {
		return apperror.NewBusiness(apperror.CodeUnauthorized, "unauthorized")
	}

	roles, ok := middleware.UserRolesFromCtx(c)
	if !ok {
		return apperror.NewBusiness(apperror.CodeUnauthorized, "missing user roles")
	}

	limit, offset := parsePagination(c)

	resp, err := h.client.ListUsers(c.Context(), &httpclient.AuthContext{
		UserID: userID,
		Roles:  roles,
	}, limit, offset)
	if err != nil {
		return err
	}

	return c.Status(fiber.StatusOK).JSON(resp)
}

func (h *Handler) DeleteUser(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok {
		return apperror.NewBusiness(apperror.CodeUnauthorized, "unauthorized")
	}

	roles, ok := middleware.UserRolesFromCtx(c)
	if !ok {
		return apperror.NewBusiness(apperror.CodeUnauthorized, "missing user roles")
	}

	targetID, err := parseUUIDParam(c, "id")
	if err != nil {
		return err
	}

	if err := h.client.DeleteUser(c.Context(), &httpclient.AuthContext{
		UserID: userID,
		Roles:  roles,
	}, targetID); err != nil {
		return err
	}

	return c.SendStatus(fiber.StatusNoContent)
}

func (h *Handler) AddRole(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok {
		return apperror.NewBusiness(apperror.CodeUnauthorized, "unauthorized")
	}

	roles, ok := middleware.UserRolesFromCtx(c)
	if !ok {
		return apperror.NewBusiness(apperror.CodeUnauthorized, "missing user roles")
	}

	targetID, err := parseUUIDParam(c, "id")
	if err != nil {
		return err
	}

	role := c.Params("role")
	if role == "" {
		return apperror.NewBusiness(apperror.CodeBadRequest, "role param is required")
	}

	if err := h.client.AddRole(c.Context(), &httpclient.AuthContext{
		UserID: userID,
		Roles:  roles,
	}, targetID, role); err != nil {
		return err
	}

	return c.SendStatus(fiber.StatusNoContent)
}

func (h *Handler) RemoveRole(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok {
		return apperror.NewBusiness(apperror.CodeUnauthorized, "unauthorized")
	}

	roles, ok := middleware.UserRolesFromCtx(c)
	if !ok {
		return apperror.NewBusiness(apperror.CodeUnauthorized, "missing user roles")
	}

	targetID, err := parseUUIDParam(c, "id")
	if err != nil {
		return err
	}

	role := c.Params("role")
	if role == "" {
		return apperror.NewBusiness(apperror.CodeBadRequest, "role param is required")
	}

	if err := h.client.RemoveRole(c.Context(), &httpclient.AuthContext{
		UserID: userID,
		Roles:  roles,
	}, targetID, role); err != nil {
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

func parsePagination(c fiber.Ctx) (limit, offset int) {
	limit = 20
	offset = 0

	l := fiber.Query[int](c, "limit", 20)
	if l > 0 && l <= 100 {
		limit = l
	}

	o := fiber.Query[int](c, "offset", 0)
	if o >= 0 {
		offset = o
	}

	return limit, offset
}
