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

func (s *OrderService) HandlePaymentSucceeded(ctx context.Context, input svcDTO.PaymentSucceededInput) error {
	inboxEvent := &domain.InboxEvent{
		ID:            uuid.New(),
		EventID:       input.EventID,
		CorrelationID: input.CorrelationID,
		EventType:     infrakafka.TopicPaymentSucceded,
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
			return apperror.NewInternal("orderRepo.GetByID", err, "Что-то пошло не так", nil)
		}

		targetStatus := domain.OrderStatusPaid
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

		if err := s.orderRepo.UpdateStatus(ctx, input.OrderID, targetStatus); err != nil {
			return apperror.NewInternal("orderRepo.UpdateStatus", err, "Что-то пошло не так", nil)
		}

		saga, err := s.sagaRepo.GetByOrderID(ctx, input.OrderID)
		if err != nil {
			return apperror.NewInternal("sagaRepo.GetByOrderID", err, "Что-то пошло не так", nil)
		}

		targetStep := domain.SagaStepCompleted
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
		targetSagaStatus := domain.SagaStatusCompleted
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

		sagaUpdate := &domain.SagaState{
			OrderID:     input.OrderID,
			CurrentStep: targetStep,
			Status:      targetSagaStatus,
			UpdatedAt:   time.Now(),
		}

		if err := s.sagaRepo.Update(ctx, sagaUpdate); err != nil {
			return apperror.NewInternal("sagaRepo.Update", err, "Что-то пошло не так", nil)
		}

		return nil
	})
}
