package handler

import (
	"context"
	"strconv"

	"github.com/Krokozabra213/e-commerce_shop/infra/apperror"
	"github.com/Krokozabra213/e-commerce_shop/services/user-service/internal/domain"
	"github.com/Krokozabra213/e-commerce_shop/services/user-service/internal/middleware"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

type UserService interface {
	AddRole(ctx context.Context, userID uuid.UUID, role domain.Role) error
	DeleteUser(ctx context.Context, id uuid.UUID) error
	GetMyProfile(ctx context.Context, userID uuid.UUID) (*domain.User, error)
	GetUserByID(ctx context.Context, id uuid.UUID) (*domain.User, error)
	ListUsers(ctx context.Context, filter domain.ListUsersFilter) ([]*domain.User, int, error)
	RemoveRole(ctx context.Context, userID uuid.UUID, role domain.Role) error
	UpdateMyProfile(ctx context.Context, userID uuid.UUID, input domain.UpdateProfileInput) (*domain.User, error)
	GetRoles(ctx context.Context, userID uuid.UUID) ([]domain.Role, error)
}

type Handler struct {
	service UserService
}

func NewHandler(service UserService) *Handler {
	return &Handler{
		service: service,
	}
}

func (h *Handler) GetMyRoles(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok {
		return apperror.NewBusiness(apperror.CodeUnauthorized, "unauthorized")
	}

	roles, err := h.service.GetRoles(c.Context(), userID)
	if err != nil {
		return err
	}
	rolesStr := make([]string, 0, len(roles))
	for _, role := range roles {
		rolesStr = append(rolesStr, role.String())
	}
	return c.JSON(RolesResponse{Roles: rolesStr})
}

func (h *Handler) GetMyProfile(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok {
		return apperror.NewBusiness(apperror.CodeUnauthorized, "unauthorized")
	}

	user, err := h.service.GetMyProfile(c.Context(), userID)
	if err != nil {
		return err
	}

	return c.JSON(NewUserResponse(user))
}

func (h *Handler) UpdateMyProfile(c fiber.Ctx) error {
	userID, ok := middleware.UserIDFromCtx(c)
	if !ok {
		return apperror.NewBusiness(apperror.CodeUnauthorized, "unauthorized")
	}

	var req UpdateProfileRequest
	if err := c.Bind().Body(&req); err != nil {
		return apperror.NewBusiness(apperror.CodeBadRequest, "invalid request body")
	}

	user, err := h.service.UpdateMyProfile(c.Context(), userID, req.ToDomain())
	if err != nil {
		return err
	}

	return c.JSON(NewUserResponse(user))
}

func (h *Handler) GetUserByID(c fiber.Ctx) error {
	targetID, err := parseUUIDParam(c, "id")
	if err != nil {
		return err
	}

	user, err := h.service.GetUserByID(c.Context(), targetID)
	if err != nil {
		return err
	}

	return c.JSON(NewUserResponse(user))
}

func (h *Handler) ListUsers(c fiber.Ctx) error {
	filter := parsePagination(c)

	users, total, err := h.service.ListUsers(c.Context(), filter)
	if err != nil {
		return err
	}

	items := make([]UserResponse, len(users))
	for i, u := range users {
		items[i] = NewUserResponse(u)
	}

	return c.JSON(ListUsersResponse{
		Users:  items,
		Total:  total,
		Limit:  filter.Limit,
		Offset: filter.Offset,
	})
}

func (h *Handler) DeleteUser(c fiber.Ctx) error {
	targetID, err := parseUUIDParam(c, "id")
	if err != nil {
		return err
	}

	if err := h.service.DeleteUser(c.Context(), targetID); err != nil {
		return err
	}

	return c.SendStatus(fiber.StatusNoContent)
}

func (h *Handler) AddRole(c fiber.Ctx) error {
	targetID, err := parseUUIDParam(c, "id")
	if err != nil {
		return err
	}

	role := domain.Role(c.Params("role"))
	if !role.IsValid() {
		return apperror.NewBusiness(apperror.CodeValidation, "invalid role")
	}

	if err := h.service.AddRole(c.Context(), targetID, role); err != nil {
		return err
	}

	return c.SendStatus(fiber.StatusNoContent)
}

func (h *Handler) RemoveRole(c fiber.Ctx) error {
	targetID, err := parseUUIDParam(c, "id")
	if err != nil {
		return err
	}

	role := domain.Role(c.Params("role"))
	if !role.IsValid() {
		return apperror.NewBusiness(apperror.CodeValidation, "invalid role")
	}

	if err := h.service.RemoveRole(c.Context(), targetID, role); err != nil {
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

func parsePagination(c fiber.Ctx) domain.ListUsersFilter {
	limit, _ := strconv.Atoi(c.Query("limit", "20"))
	offset, _ := strconv.Atoi(c.Query("offset", "0"))

	return domain.ListUsersFilter{
		Limit:  limit,
		Offset: offset,
	}
}
