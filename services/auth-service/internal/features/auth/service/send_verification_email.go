package authservice

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/Krokozabra213/e-commerce_shop/infra/apperror"
	"github.com/Krokozabra213/e-commerce_shop/infra/logger"
	"github.com/Krokozabra213/e-commerce_shop/services/auth-service/internal/domain"
	"github.com/google/uuid"
)

func (s *Service) SendVerificationEmail(ctx context.Context, input SendVerificationEmailInput) error {
	log := logger.FromContext(ctx)

	user, err := s.userRepo.GetByID(ctx, input.UserID)
	if err != nil {
		if errors.Is(err, domain.NotFoundError) {
			return apperror.NewBusiness(apperror.CodeNotFound, "Пользователь не найден")
		}
		return apperror.NewInternal("userRepo.GetByID", err, "Ошибка получения пользователя", nil)
	}

	if user.EmailConfirmedAt != nil {
		return nil
	}

	verificationToken, tokenHash, err := s.generateVerificationToken()
	if err != nil {
		return apperror.NewInternal("generateVerificationToken", err, "Ошибка генерации токена", nil)
	}

	log.DebugContext(ctx, "verification token generated",
		slog.String("user_id", user.ID.String()),
		slog.String("token", verificationToken),
	)

	now := time.Now()
	emailToken := &domain.EmailVerificationToken{
		ID:        uuid.New(),
		UserID:    user.ID,
		TokenHash: tokenHash,
		ExpiresAt: now.Add(s.emailVerifConfig.EmailVerificationTTL),
		CreatedAt: now,
	}

	if err := s.emailVerificationRepo.Create(ctx, emailToken); err != nil {
		return apperror.NewInternal("emailVerificationRepo.Create", err, "Не удалось отправить письмо", nil)
	}

	// TODO: добавить grpc отправку в notification service
	return nil
}

func (s *Service) generateVerificationToken() (token, tokenHash string, err error) {
	bytes := make([]byte, s.emailVerifConfig.VerificationTokenLength)
	if _, err := rand.Read(bytes); err != nil {
		return "", "", fmt.Errorf("crypto rand: %w", err)
	}

	token = base64.RawURLEncoding.EncodeToString(bytes)
	tokenHash = s.tokenHasher.Hash(token)

	return token, tokenHash, nil
}
