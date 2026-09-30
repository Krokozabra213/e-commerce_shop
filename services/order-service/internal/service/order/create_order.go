package orderService

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/Krokozabra213/e-commerce_shop/infra/apperror"
	infrakafka "github.com/Krokozabra213/e-commerce_shop/infra/kafka"
	"github.com/Krokozabra213/e-commerce_shop/services/order-service/internal/domain"
	svcDTO "github.com/Krokozabra213/e-commerce_shop/services/order-service/internal/service/dto"
	"github.com/google/uuid"
)

const maxPrice = 1_000_000_000_000

func (s *OrderService) CreateOrder(ctx context.Context, input svcDTO.CreateOrderInput) (*svcDTO.CreateOrderOutput, error) {

	productIDs := extractProductIDs(input.Items)
	productPrices, err := s.productSvc.GetPrices(ctx, productIDs)
	if err != nil {
		return nil, apperror.NewInternal(
			"productSvc.GetPrices",
			err,
			"Не удалось получить цены товаров",
			nil,
		)
	}

	totalPrice, orderItems, err := s.calculateOrderTotal(input.Items, productPrices)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	correlationID := uuid.New()

	order := &domain.Order{
		ID:             uuid.New(),
		UserID:         input.UserID,
		IdempotencyKey: input.IdempotencyKey,
		Status:         domain.OrderStatusNew,
		TotalPrice:     totalPrice,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	var output *svcDTO.CreateOrderOutput

	txErr := s.txManager.WithinTransaction(ctx, func(ctx context.Context) error {
		if err := s.orderRepo.Create(ctx, order); err != nil {
			if errors.Is(err, domain.ErrAlreadyExists) {
				existingOrder, getErr := s.orderRepo.GetByIdempotencyKey(ctx, input.IdempotencyKey)
				if getErr != nil {
					if errors.Is(getErr, domain.ErrNotFound) {
						return apperror.NewAppErr(
							apperror.CodeConflict,
							"orderRepo.GetByIdempotencyKey",
							"Order is being created, please retry",
							getErr,
							apperror.LevelDebug,
							nil,
						)
					}

					return apperror.NewInternal(
						"orderRepo.GetByIdempotencyKey",
						getErr,
						"Что-то пошло не так",
						nil,
					)
				}

				output = &svcDTO.CreateOrderOutput{
					OrderID:    existingOrder.ID,
					Status:     existingOrder.Status.String(),
					TotalPrice: existingOrder.TotalPrice,
				}
				return nil
			}

			return apperror.NewInternal("orderRepo.Create", err, "Что-то пошло не так", nil)
		}

		for i := range orderItems {
			orderItems[i].OrderID = order.ID
		}

		if err := s.orderRepo.CreateItems(ctx, orderItems); err != nil {
			return apperror.NewInternal("orderRepo.CreateItems", err, "Что-то пошло не так", nil)
		}

		saga := &domain.SagaState{
			OrderID:       order.ID,
			CorrelationID: correlationID,
			CurrentStep:   domain.SagaStepReservingInventory,
			Status:        domain.SagaStatusInProgress,
			CreatedAt:     now,
			UpdatedAt:     now,
		}

		if err := s.sagaRepo.Create(ctx, saga); err != nil {
			return apperror.NewInternal("sagaRepo.Create", err, "Что-то пошло не так", nil)
		}

		if err := s.publishReserveInventory(ctx, order.ID, correlationID, input.Items); err != nil {
			return err
		}

		output = &svcDTO.CreateOrderOutput{
			OrderID:    order.ID,
			Status:     string(order.Status),
			TotalPrice: order.TotalPrice,
		}

		return nil
	})

	if txErr != nil {
		return nil, txErr
	}

	return output, nil
}

func (s *OrderService) publishReserveInventory(
	ctx context.Context,
	orderID uuid.UUID,
	correlationID uuid.UUID,
	items []svcDTO.OrderItemInput,
) error {
	payloadItems := make([]svcDTO.OrderItemPayload, len(items))
	for i, item := range items {
		payloadItems[i] = svcDTO.OrderItemPayload{
			ProductID: item.ProductID,
			Quantity:  int32(item.Quantity),
		}
	}

	payload := svcDTO.OrderCreatedPayload{
		OrderID:   orderID.String(),
		Items:     payloadItems,
		CreatedAt: time.Now(),
	}

	payloadMap, err := svcDTO.ToMap(payload)
	if err != nil {
		return fmt.Errorf("marshal outbox payload: %w", err)
	}

	return s.createOutboxEvent(
		ctx,
		correlationID,
		orderID,
		infrakafka.TopicOrderCreated,
		payloadMap,
	)
}

func extractProductIDs(items []svcDTO.OrderItemInput) []string {
	ids := make([]string, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ProductID)
	}
	return ids
}

func (s *OrderService) calculateOrderTotal(
	items []svcDTO.OrderItemInput,
	prices map[string]int64,
) (int64, []domain.OrderItem, error) {
	var totalPrice int64
	orderItems := make([]domain.OrderItem, 0, len(items))
	now := time.Now()

	for _, item := range items {
		price, ok := prices[item.ProductID]
		if !ok {
			return 0, nil, apperror.NewAppErr(
				apperror.CodeBadRequest,
				"orderService.calculateOrderTotal",
				fmt.Sprintf("product %s not found", item.ProductID),
				nil,
				apperror.LevelDebug,
				nil,
			)
		}

		if price > 0 && item.Quantity > 0 {
			if price > math.MaxInt64/int64(item.Quantity) {
				return 0, nil, apperror.NewAppErr(
					apperror.CodeBadRequest,
					"orderService.calculateOrderTotal",
					fmt.Sprintf("price overflow for product %s", item.ProductID),
					nil,
					apperror.LevelDebug,
					nil,
				)
			}
		}

		itemTotal := price * int64(item.Quantity)

		if totalPrice > math.MaxInt64-itemTotal {
			return 0, nil, apperror.NewAppErr(
				apperror.CodeBadRequest,
				"orderService.calculateOrderTotal",
				"total price overflow",
				nil,
				apperror.LevelDebug,
				nil,
			)
		}

		totalPrice += itemTotal

		if totalPrice > maxPrice {
			return 0, nil, apperror.NewAppErr(
				apperror.CodeBadRequest,
				"orderService.calculateOrderTotal",
				"total price exceeds maximum allowed",
				nil,
				apperror.LevelDebug,
				nil,
			)
		}

		orderItems = append(orderItems, domain.OrderItem{
			ID:        uuid.New(),
			ProductID: item.ProductID,
			Quantity:  item.Quantity,
			Price:     price,
			CreatedAt: now,
		})
	}

	return totalPrice, orderItems, nil
}
