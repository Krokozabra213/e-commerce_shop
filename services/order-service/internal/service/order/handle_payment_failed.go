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

func (s *OrderService) HandlePaymentFailed(ctx context.Context, input svcDTO.PaymentFailedInput) error {
	payload := svcDTO.PaymentFailedPayload{
		OrderID: input.OrderID.String(),
		Reason:  input.Reason,
	}

	payloadMap, err := svcDTO.ToMap(payload)
	if err != nil {
		return apperror.NewInternal("marshal payment failed inbox payload", err, "Что-то пошло не так", nil)
	}

	inboxEvent := &domain.InboxEvent{
		ID:            uuid.New(),
		EventID:       input.EventID,
		CorrelationID: input.CorrelationID,
		EventType:     infrakafka.TopicPaymentFailed,
		Payload:       payloadMap,
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

		targetStatus := domain.OrderStatusCancelled
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

		if err := s.orderRepo.UpdateStatus(ctx, input.OrderID, domain.OrderStatusCancelled); err != nil {
			return apperror.NewInternal("orderRepo.UpdateStatus", err, "Что-то пошло не так", nil)
		}

		saga, err := s.sagaRepo.GetByOrderID(ctx, input.OrderID)
		if err != nil {
			if errors.Is(err, domain.ErrNotFound) {
				return apperror.NewInternal("sagaRepo.GetByOrderID", err, "Саги не существует", nil)
			}
			return apperror.NewInternal("sagaRepo.GetByOrderID", err, "Что-то пошло не так", nil)
		}

		targetStep := domain.SagaStepCompensatingInventory
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

		targetSagaStatus := domain.SagaStatusCompensating
		if can := s.sagaStatusStateMachine.CanTransition(saga.Status.String(), targetSagaStatus.String()); !can {
			return apperror.NewAppErr(
				apperror.CodeConflict,
				"sagaStatusStateMachine.CanTransition",
				fmt.Sprintf("invalid saga status transition: from %s to %s", saga.Status.String(), targetSagaStatus.String()),
				nil,
				apperror.LevelWarn,
				nil,
			)
		}

		sagaCompensating := &domain.SagaState{
			OrderID:     input.OrderID,
			CurrentStep: domain.SagaStepCompensatingInventory,
			Status:      domain.SagaStatusCompensating,
			UpdatedAt:   time.Now(),
		}

		if err := s.sagaRepo.Update(ctx, sagaCompensating); err != nil {
			return apperror.NewInternal("sagaRepo.Update", err, "Что-то пошло не так", nil)
		}

		if err := s.publishCancelInventory(ctx, input.OrderID, input.CorrelationID, input.Reason); err != nil {
			return err
		}

		if can := s.sagaStepStateMachine.CanTransition(
			domain.SagaStepCompensatingInventory.String(),
			domain.SagaStepCompleted.String(),
		); !can {
			return apperror.NewAppErr(
				apperror.CodeConflict,
				"sagaStepStateMachine.CanTransition",
				fmt.Sprintf("invalid saga step transition: from %s to %s", domain.SagaStepCompensatingInventory.String(), domain.SagaStepCompleted.String()),
				nil,
				apperror.LevelWarn,
				nil,
			)
		}

		if can := s.sagaStatusStateMachine.CanTransition(domain.SagaStatusCompensating.String(), domain.SagaStatusCompensated.String()); !can {
			return apperror.NewAppErr(
				apperror.CodeConflict,
				"sagaStatusStateMachine.CanTransition",
				fmt.Sprintf("invalid saga status transition: from %s to %s", domain.SagaStatusCompensating.String(), domain.SagaStatusCompensated.String()),
				nil,
				apperror.LevelWarn,
				nil,
			)
		}

		sagaCompensated := &domain.SagaState{
			OrderID:     input.OrderID,
			CurrentStep: domain.SagaStepCompleted,
			Status:      domain.SagaStatusCompensated,
			UpdatedAt:   time.Now(),
		}

		if err := s.sagaRepo.Update(ctx, sagaCompensated); err != nil {
			return apperror.NewInternal("sagaRepo.Update", err, "Что-то пошло не так", nil)
		}

		return nil
	})
}

func (s *OrderService) publishCancelInventory(
	ctx context.Context,
	orderID uuid.UUID,
	correlationID uuid.UUID,
	reason string,
) error {
	payload := svcDTO.OrderCancellationPayload{
		OrderID:     orderID.String(),
		Reason:      reason,
		CancelledAt: time.Now(),
	}

	payloadMap, err := svcDTO.ToMap(payload)
	if err != nil {
		return fmt.Errorf("marshal order cancellation payload: %w", err)
	}

	return s.createOutboxEvent(
		ctx,
		correlationID,
		orderID,
		infrakafka.TopicOrderCancelInventory,
		payloadMap,
	)
}
