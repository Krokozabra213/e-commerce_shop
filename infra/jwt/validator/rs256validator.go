package jwtvalidator

import (
	"crypto/rsa"
	"errors"
	"fmt"
	"sync"
	"time"

	jwtmanager "github.com/Krokozabra213/e-commerce_shop/infra/jwt/manager"
	"github.com/golang-jwt/jwt/v5"
)

var (
	ErrEmptyPublicKey = errors.New("empty public key")
	ErrEmptyIssuer    = errors.New("empty issuer")
	ErrParseJWT       = errors.New("failed to parse jwt token")
	ErrJWTID          = errors.New("invalid jwt id")
)

type RS256Validator struct {
	mu        sync.RWMutex
	publicKey *rsa.PublicKey
	leeway    time.Duration
	issuer    string
}

func NewRS256Validator(publicKey *rsa.PublicKey, leeway time.Duration, issuer string) (*RS256Validator, error) {
	if publicKey == nil {
		return nil, ErrEmptyPublicKey
	}
	return &RS256Validator{
		publicKey: publicKey,
		leeway:    leeway,
		issuer:    issuer,
	}, nil
}

func (v *RS256Validator) RotateIssuer(issuer string) error {
	if issuer == "" {
		return ErrEmptyIssuer
	}

	v.mu.Lock()
	defer v.mu.Unlock()

	v.issuer = issuer

	return nil
}

func (v *RS256Validator) RotatePublicKey(key *rsa.PublicKey) error {
	if key == nil {
		return ErrEmptyPublicKey
	}

	v.mu.Lock()
	defer v.mu.Unlock()

	v.publicKey = key
	return nil
}

func (v *RS256Validator) parse(tokenString string, claims jwt.Claims) error {
	pubKey, issuer := v.lockSnapshot()

	keyFunc := func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}

		if token.Method.Alg() != jwt.SigningMethodRS256.Alg() {
			return nil, fmt.Errorf("unexpected signing algorithm: got %q, want %q",
				token.Method.Alg(), jwt.SigningMethodRS256.Alg())
		}

		return pubKey, nil
	}

	_, err := jwt.ParseWithClaims(tokenString, claims, keyFunc,
		jwt.WithLeeway(v.leeway),
		jwt.WithIssuer(issuer),
	)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrParseJWT, err)
	}

	return nil
}

func (v *RS256Validator) ValidateAccess(tokenString string) (*jwtmanager.AccessClaims, error) {
	claims := &jwtmanager.AccessClaims{}

	if err := v.parse(tokenString, claims); err != nil {
		return nil, err
	}

	return claims, nil
}

func (v *RS256Validator) ValidateRefresh(tokenString string) (*jwtmanager.RefreshClaims, error) {
	claims := &jwtmanager.RefreshClaims{}

	if err := v.parse(tokenString, claims); err != nil {
		return nil, err
	}

	if claims.ID == "" {
		return nil, ErrJWTID
	}

	return claims, nil
}

func (v *RS256Validator) lockSnapshot() (*rsa.PublicKey, string) {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.publicKey, v.issuer
}
