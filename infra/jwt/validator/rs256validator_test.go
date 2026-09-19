package jwtvalidator_test

import (
	"crypto/rand"
	"crypto/rsa"
	"sync"
	"testing"
	"time"

	jwtvalidator "github.com/Krokozabra213/e-commerce_shop/infra/jwt/validator"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	jwtmanager "github.com/Krokozabra213/e-commerce_shop/infra/jwt/manager"
)

func generateTestKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	return key
}

func signToken(t *testing.T, claims jwt.Claims, key *rsa.PrivateKey) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	signed, err := token.SignedString(key)
	require.NoError(t, err)
	return signed
}

func signTokenWithAlg(t *testing.T, claims jwt.Claims, method jwt.SigningMethod, key interface{}) string {
	t.Helper()
	token := jwt.NewWithClaims(method, claims)
	signed, err := token.SignedString(key)
	require.NoError(t, err)
	return signed
}

func newTestValidator(t *testing.T, pubKey *rsa.PublicKey) *jwtvalidator.RS256Validator {
	t.Helper()
	v, err := jwtvalidator.NewRS256Validator(pubKey, 30*time.Second, "test-issuer")
	require.NoError(t, err)
	return v
}

func TestNewRS256Validator_NilKey(t *testing.T) {
	_, err := jwtvalidator.NewRS256Validator(nil, 30*time.Second, "issuer")
	assert.ErrorIs(t, err, jwtvalidator.ErrEmptyPublicKey)
}

func TestNewRS256Validator_Success(t *testing.T) {
	key := generateTestKey(t)
	v, err := jwtvalidator.NewRS256Validator(&key.PublicKey, 30*time.Second, "issuer")
	require.NoError(t, err)
	assert.NotNil(t, v)
}

func TestValidateAccess_Success(t *testing.T) {
	key := generateTestKey(t)
	v := newTestValidator(t, &key.PublicKey)
	userID := uuid.New()

	claims := jwtmanager.AccessClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "test-issuer",
			Subject:   userID.String(),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(15 * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	tokenString := signToken(t, &claims, key)

	got, err := v.ValidateAccess(tokenString)
	require.NoError(t, err)
	assert.Equal(t, "test-issuer", got.Issuer)
	assert.Equal(t, userID.String(), got.Subject)
}

func TestValidateAccess_Expired(t *testing.T) {
	key := generateTestKey(t)
	v, err := jwtvalidator.NewRS256Validator(&key.PublicKey, 0, "test-issuer")
	require.NoError(t, err)

	claims := jwtmanager.AccessClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "test-issuer",
			Subject:   uuid.New().String(),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Hour)), // истёк час назад
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
		},
	}
	tokenString := signToken(t, &claims, key)

	_, err = v.ValidateAccess(tokenString)
	assert.Error(t, err, "истёкший токен должен отклоняться")
}

func TestValidateAccess_WrongSignature(t *testing.T) {
	key1 := generateTestKey(t)
	key2 := generateTestKey(t) // ДРУГОЙ ключ
	v := newTestValidator(t, &key1.PublicKey)

	claims := jwtmanager.AccessClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "test-issuer",
			Subject:   uuid.New().String(),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(15 * time.Minute)),
		},
	}
	tokenString := signToken(t, &claims, key2)

	_, err := v.ValidateAccess(tokenString)
	assert.Error(t, err, "токен с чужой подписью должен отклоняться")
}

func TestValidateAccess_WrongIssuer(t *testing.T) {
	key := generateTestKey(t)
	v := newTestValidator(t, &key.PublicKey)

	claims := jwtmanager.AccessClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "evil-issuer",
			Subject:   uuid.New().String(),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(15 * time.Minute)),
		},
	}
	tokenString := signToken(t, &claims, key)

	_, err := v.ValidateAccess(tokenString)
	assert.Error(t, err, "токен с чужим issuer должен отклоняться")
}

func TestValidateAccess_WrongAlgorithm(t *testing.T) {
	key := generateTestKey(t)
	v := newTestValidator(t, &key.PublicKey)

	claims := jwtmanager.AccessClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "test-issuer",
			Subject:   uuid.New().String(),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(15 * time.Minute)),
		},
	}

	tokenString := signTokenWithAlg(t, &claims, jwt.SigningMethodRS384, key)

	_, err := v.ValidateAccess(tokenString)
	assert.Error(t, err, "токен с алгоритмом RS384 должен отклоняться валидатором RS256")
}

func TestValidateRefresh_Success(t *testing.T) {
	key := generateTestKey(t)
	v := newTestValidator(t, &key.PublicKey)
	userID := uuid.New()

	claims := jwtmanager.RefreshClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "test-issuer",
			Subject:   userID.String(),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ID:        uuid.NewString(),
		},
	}
	tokenString := signToken(t, &claims, key)

	got, err := v.ValidateRefresh(tokenString)
	require.NoError(t, err)
	assert.NotEmpty(t, got.ID)
	assert.Equal(t, userID.String(), got.Subject)
}

func TestValidateRefresh_MissingJTI(t *testing.T) {
	key := generateTestKey(t)
	v := newTestValidator(t, &key.PublicKey)

	claims := jwtmanager.RefreshClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "test-issuer",
			Subject:   uuid.New().String(),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
		},
	}
	tokenString := signToken(t, &claims, key)

	_, err := v.ValidateRefresh(tokenString)
	assert.ErrorIs(t, err, jwtvalidator.ErrJWTID, "refresh без jti должен отклоняться")
}

func TestValidateAccess_Leeway(t *testing.T) {
	key := generateTestKey(t)
	v, err := jwtvalidator.NewRS256Validator(&key.PublicKey, 60*time.Second, "test-issuer")
	require.NoError(t, err)

	claims := jwtmanager.AccessClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "test-issuer",
			Subject:   uuid.New().String(),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-30 * time.Second)),
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-1 * time.Hour)),
		},
	}
	tokenString := signToken(t, &claims, key)

	got, err := v.ValidateAccess(tokenString)
	require.NoError(t, err, "токен в пределах leeway должен проходить")
	assert.NotNil(t, got)
}

func TestValidateAccess_BeyondLeeway(t *testing.T) {
	key := generateTestKey(t)
	v, err := jwtvalidator.NewRS256Validator(&key.PublicKey, 30*time.Second, "test-issuer")
	require.NoError(t, err)

	claims := jwtmanager.AccessClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "test-issuer",
			Subject:   uuid.New().String(),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-2 * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-1 * time.Hour)),
		},
	}
	tokenString := signToken(t, &claims, key)

	_, err = v.ValidateAccess(tokenString)
	assert.Error(t, err, "токен за пределами leeway должен отклоняться")
}

func TestRotatePublicKey(t *testing.T) {
	oldKey := generateTestKey(t)
	newKey := generateTestKey(t)
	v := newTestValidator(t, &oldKey.PublicKey)

	claims := jwtmanager.AccessClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "test-issuer",
			Subject:   uuid.New().String(),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(15 * time.Minute)),
		},
	}

	tokenString := signToken(t, &claims, oldKey)

	_, err := v.ValidateAccess(tokenString)
	require.NoError(t, err)

	err = v.RotatePublicKey(&newKey.PublicKey)
	require.NoError(t, err)

	_, err = v.ValidateAccess(tokenString)
	assert.Error(t, err, "после ротации старый токен должен отклоняться")

	newToken := signToken(t, &claims, newKey)
	_, err = v.ValidateAccess(newToken)
	require.NoError(t, err)
}

func TestRotatePublicKey_Nil(t *testing.T) {
	key := generateTestKey(t)
	v := newTestValidator(t, &key.PublicKey)

	err := v.RotatePublicKey(nil)
	assert.ErrorIs(t, err, jwtvalidator.ErrEmptyPublicKey)
}

func TestRotateIssuer(t *testing.T) {
	key := generateTestKey(t)
	v := newTestValidator(t, &key.PublicKey)

	claims := jwtmanager.AccessClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "new-issuer",
			Subject:   uuid.New().String(),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(15 * time.Minute)),
		},
	}
	tokenString := signToken(t, &claims, key)

	_, err := v.ValidateAccess(tokenString)
	assert.Error(t, err)

	err = v.RotateIssuer("new-issuer")
	require.NoError(t, err)

	_, err = v.ValidateAccess(tokenString)
	require.NoError(t, err)
}

func TestRotateIssuer_Empty(t *testing.T) {
	key := generateTestKey(t)
	v := newTestValidator(t, &key.PublicKey)

	err := v.RotateIssuer("")
	assert.ErrorIs(t, err, jwtvalidator.ErrEmptyIssuer)
}

func TestConcurrent_ValidateAndRotate(t *testing.T) {
	key := generateTestKey(t)
	v := newTestValidator(t, &key.PublicKey)

	claims := jwtmanager.AccessClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "test-issuer",
			Subject:   uuid.New().String(),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(15 * time.Minute)),
		},
	}
	tokenString := signToken(t, &claims, key)

	var wg sync.WaitGroup
	const goroutines = 100

	for i := 0; i < goroutines/2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = v.ValidateAccess(tokenString)
		}()
	}

	for i := 0; i < goroutines/4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			newKey := generateTestKey(t)
			_ = v.RotatePublicKey(&newKey.PublicKey)
		}()
	}

	for i := 0; i < goroutines/4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = v.RotateIssuer("issuer-" + uuid.NewString())
		}()
	}

	wg.Wait()
}
