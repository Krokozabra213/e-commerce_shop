package jwtmanager

import (
	"crypto/rsa"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

var (
	ErrEmptyPrivateKey    = errors.New("empty private key")
	ErrSignedAccessToken  = errors.New("failed sign access jwt token")
	ErrSignedRefreshToken = errors.New("failed sign refresh jwt token")
	ErrUserID             = errors.New("invalid user id")
	ErrEmptyIssuer        = errors.New("empty issuer")
)

type AccessClaims struct {
	jwt.RegisteredClaims
}

type RefreshClaims struct {
	jwt.RegisteredClaims
}

func (c *AccessClaims) UserIDUUID() (uuid.UUID, error) {
	id, err := uuid.Parse(c.Subject)
	if err != nil {
		return uuid.Nil, ErrUserID
	}
	return id, nil
}

func (c *RefreshClaims) UserIDUUID() (uuid.UUID, error) {
	id, err := uuid.Parse(c.Subject)
	if err != nil {
		return uuid.Nil, ErrUserID
	}
	return id, nil
}

type RS256JWTManager struct {
	mu         sync.RWMutex
	privateKey *rsa.PrivateKey

	accessTTL  time.Duration
	refreshTTL time.Duration

	issuer string
}

func NewRS256Manager(privateKey *rsa.PrivateKey, accessTTL, refreshTTL time.Duration, issuer string) (*RS256JWTManager, error) {
	if privateKey == nil {
		return nil, ErrEmptyPrivateKey
	}

	return &RS256JWTManager{
		privateKey: privateKey,
		accessTTL:  accessTTL,
		refreshTTL: refreshTTL,
		issuer:     issuer,
	}, nil
}

func (m *RS256JWTManager) snapshotLock() (*rsa.PrivateKey, string) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.privateKey, m.issuer
}

func (m *RS256JWTManager) RotateKey(newPrivateKey *rsa.PrivateKey) error {
	if newPrivateKey == nil {
		return ErrEmptyPrivateKey
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.privateKey = newPrivateKey

	return nil
}

func (m *RS256JWTManager) RotateIssuer(issuer string) error {
	if issuer == "" {
		return ErrEmptyIssuer
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.issuer = issuer

	return nil
}

func (m *RS256JWTManager) GenerateTokens(userID uuid.UUID) (access, refresh string, refreshExp time.Time, err error) {
	access, err = m.GenerateAccess(userID)
	if err != nil {
		return "", "", time.Time{}, err
	}
	refresh, refreshExp, err = m.GenerateRefresh(userID)
	if err != nil {
		return "", "", time.Time{}, err
	}
	return access, refresh, refreshExp, nil
}

func (m *RS256JWTManager) GenerateAccess(userID uuid.UUID) (string, error) {
	privateKey, issuer := m.snapshotLock()

	now := time.Now()
	claims := AccessClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    issuer,
			Subject:   userID.String(),
			ExpiresAt: jwt.NewNumericDate(now.Add(m.accessTTL)), // ← immutable, ок
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)

	signed, err := token.SignedString(privateKey)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrSignedAccessToken, err)
	}
	return signed, nil
}

func (m *RS256JWTManager) GenerateRefresh(userID uuid.UUID) (string, time.Time, error) {
	privateKey, issuer := m.snapshotLock()

	now := time.Now()
	refreshExp := now.Add(m.refreshTTL)
	claims := RefreshClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    issuer,
			Subject:   userID.String(),
			ExpiresAt: jwt.NewNumericDate(refreshExp),
			IssuedAt:  jwt.NewNumericDate(now),
			ID:        uuid.NewString(),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)

	signed, err := token.SignedString(privateKey)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("%w: %v", ErrSignedRefreshToken, err)
	}
	return signed, refreshExp, nil
}
