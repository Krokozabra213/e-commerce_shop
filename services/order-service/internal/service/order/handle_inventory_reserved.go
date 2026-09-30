package orderService

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Krokozabra213/e-commerce_shop/infra/apperror"
	infrakafka "github.com/Krokozabra213/e-commerce_shop/infra/kafka"
	"github.com/Krokozabra213/e-commerce_shop/services/order-service/internal/domain"
	svcDTO "github.com/Krokozabra213/e-commerce_shop/services/order-service/internal/service/dto"
	"github.com/google/uuid"
)

func (s *OrderService) HandleInventoryReserved(ctx context.Context, input *svcDTO.InventoryReservedInput) error {
	inboxEvent := &domain.InboxEvent{
		ID:            uuid.New(),
		EventID:       input.EventID,
		CorrelationID: input.CorrelationID,
		EventType:     infrakafka.TopicInventoryReserved,
		ProcessedAt:   time.Now(),
	}

	return s.txManager.WithinTransaction(ctx, func(ctx context.Context) error {

		if err := s.inboxRepo.Create(ctx, inboxEvent); err != nil {
			if errors.Is(err, domain.ErrAlreadyExists) {
				return nil
			}
			return apperror.NewInternal("inboxRepo.Create", err, "Что-то пошло не так", nil)
		}

		order, err := s.orderRepo.GetByID(ctx, input.OrderID)
		if err != nil {
			if errors.Is(err, domain.ErrNotFound) {
				return apperror.NewInternal("orderRepo.GetByID", err, "Заказа не существует", nil)
			}
			return apperror.NewInternal("orderRepo.GetByID", err, "Что-то пошло не так", nil)
		}

		targetStatus := domain.OrderStatusReserved
		if can := s.orderStateMachine.CanTransition(order.Status.String(), targetStatus.String()); !can {
			return apperror.NewAppErr(
				apperror.CodeConflict,
				"orderStateMachine.CanTransition",
				fmt.Sprintf("invalid order status transition: from %s to %s", order.Status.String(), targetStatus.String()),
				nil,
				apperror.LevelWarn,
				nil,
			)
		}

		if err := s.orderRepo.UpdateStatus(ctx, input.OrderID, domain.OrderStatusReserved); err != nil {
			return apperror.NewInternal("orderRepo.UpdateStatus", err, "Что-то пошло не так", nil)
		}

		saga, err := s.sagaRepo.GetByOrderID(ctx, input.OrderID)
		if err != nil {
			if errors.Is(err, domain.ErrNotFound) {
				return apperror.NewInternal("sagaRepo.GetByOrderID", err, "Саги не существует", nil)
			}
			return apperror.NewInternal("sagaRepo.GetByOrderID", err, "Что-то пошло не так", nil)
		}

		targetStep := domain.SagaStepChargingPayment
		if can := s.sagaStepStateMachine.CanTransition(saga.CurrentStep.String(), targetStep.String()); !can {
			return apperror.NewAppErr(
				apperror.CodeConflict,
				"sagaStepStateMachine.CanTransition",
				fmt.Sprintf("invalid saga step transition: from %s to %s", saga.CurrentStep.String(), targetStep.String()),
				nil,
				apperror.LevelWarn,
				nil,
			)
		}

		sagaUpdate := &domain.SagaState{
			OrderID:     input.OrderID,
			CurrentStep: domain.SagaStepChargingPayment,
			Status:      domain.SagaStatusInProgress,
			UpdatedAt:   time.Now(),
		}

		if err := s.sagaRepo.Update(ctx, sagaUpdate); err != nil {
			return apperror.NewInternal("sagaRepo.Update", err, "Что-то пошло не так", nil)
		}

		if err := s.publishChargePayment(ctx, input.OrderID, input.CorrelationID, order.UserID, order.TotalPrice); err != nil {
			return err
		}

		return nil
	})
}

func (s *OrderService) publishChargePayment(
	ctx context.Context,
	orderID uuid.UUID,
	correlationID uuid.UUID,
	orderUserID uuid.UUID,
	orderTotalPrice int64,
) error {
	payload := svcDTO.PaymentChargePayload{
		OrderID:     orderID.String(),
		UserID:      orderUserID.String(),
		Amount:      orderTotalPrice,
		RequestedAt: time.Now(),
	}

	payloadMap, err := svcDTO.ToMap(payload)
	if err != nil {
		return fmt.Errorf("marshal payment charge payload: %w", err)
	}

	return s.createOutboxEvent(
		ctx,
		correlationID,
		orderID,
		infrakafka.TopicOrderCreatePayment,
		payloadMap,
	)
}
