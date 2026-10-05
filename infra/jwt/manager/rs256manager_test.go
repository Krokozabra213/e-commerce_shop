package jwtmanager_test

import (
	"crypto/rand"
	"crypto/rsa"
	"sync"
	"testing"
	"time"

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

func newTestManager(t *testing.T) (*jwtmanager.RS256JWTManager, *rsa.PrivateKey) {
	t.Helper()
	key := generateTestKey(t)
	mgr, err := jwtmanager.NewRS256Manager(key, 15*time.Minute, 24*time.Hour, "test-issuer")
	require.NoError(t, err)
	return mgr, key
}

func parseToken(t *testing.T, tokenString string, pubKey *rsa.PublicKey, claims jwt.Claims) {
	t.Helper()
	_, err := jwt.ParseWithClaims(tokenString, claims, func(_ *jwt.Token) (any, error) {
		return pubKey, nil
	})
	require.NoError(t, err)
}

func TestNewRS256Manager_NilKey(t *testing.T) {
	_, err := jwtmanager.NewRS256Manager(nil, 15*time.Minute, 24*time.Hour, "issuer")
	assert.ErrorIs(t, err, jwtmanager.ErrEmptyPrivateKey)
}

func TestNewRS256Manager_Success(t *testing.T) {
	key := generateTestKey(t)
	mgr, err := jwtmanager.NewRS256Manager(key, 15*time.Minute, 24*time.Hour, "issuer")
	require.NoError(t, err)
	assert.NotNil(t, mgr)
}

func TestGenerateAccess_Success(t *testing.T) {
	mgr, key := newTestManager(t)
	userID := uuid.New()

	tokenString, err := mgr.GenerateAccess(userID)
	require.NoError(t, err)
	assert.NotEmpty(t, tokenString)

	claims := &jwtmanager.AccessClaims{}
	parseToken(t, tokenString, &key.PublicKey, claims)

	assert.Equal(t, "test-issuer", claims.Issuer)
	assert.Equal(t, userID.String(), claims.Subject)
	assert.NotNil(t, claims.ExpiresAt)
	assert.NotNil(t, claims.IssuedAt)

	expectedExp := time.Now().Add(15 * time.Minute)
	assert.WithinDuration(t, expectedExp, claims.ExpiresAt.Time, 5*time.Second)
}

func TestGenerateRefresh_Success(t *testing.T) {
	mgr, key := newTestManager(t)
	userID := uuid.New()

	tokenString, refreshExp, err := mgr.GenerateRefresh(userID)
	require.NoError(t, err)
	assert.NotEmpty(t, tokenString)
	assert.NotEmpty(t, refreshExp)

	claims := &jwtmanager.RefreshClaims{}
	parseToken(t, tokenString, &key.PublicKey, claims)

	assert.Equal(t, "test-issuer", claims.Issuer)
	assert.Equal(t, userID.String(), claims.Subject)
	assert.NotEmpty(t, claims.ID, "refresh token must have jti")

	expectedExp := time.Now().Add(24 * time.Hour)
	assert.WithinDuration(t, expectedExp, claims.ExpiresAt.Time, 5*time.Second)
}

func TestGenerateRefresh_UniqueJTI(t *testing.T) {
	mgr, _ := newTestManager(t)
	userID := uuid.New()

	token1, refreshExp, err := mgr.GenerateRefresh(userID)
	require.NoError(t, err)
	require.NotEmpty(t, refreshExp)

	token2, refreshExp, err := mgr.GenerateRefresh(userID)
	require.NoError(t, err)
	require.NotEmpty(t, refreshExp)

	parser := jwt.NewParser()

	c1 := &jwtmanager.RefreshClaims{}
	_, _, err = parser.ParseUnverified(token1, c1)
	require.NoError(t, err)

	c2 := &jwtmanager.RefreshClaims{}
	_, _, err = parser.ParseUnverified(token2, c2)
	require.NoError(t, err)

	assert.NotEmpty(t, c1.ID)
	assert.NotEmpty(t, c2.ID)
	assert.NotEqual(t, c1.ID, c2.ID, "каждый refresh-токен должен иметь уникальный jti")
}

func TestGenerateTokens_Success(t *testing.T) {
	mgr, _ := newTestManager(t)
	userID := uuid.New()

	access, refresh, refreshExp, err := mgr.GenerateTokens(userID)
	require.NoError(t, err)
	assert.NotEmpty(t, access)
	assert.NotEmpty(t, refreshExp)
	assert.NotEmpty(t, refresh)
	assert.NotEqual(t, access, refresh)
}

func TestAccessClaims_UserIDUUID_Success(t *testing.T) {
	expected := uuid.New()
	claims := &jwtmanager.AccessClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject: expected.String(),
		},
	}

	got, err := claims.UserIDUUID()
	require.NoError(t, err)
	assert.Equal(t, expected, got)
}

func TestAccessClaims_UserIDUUID_Invalid(t *testing.T) {
	claims := &jwtmanager.AccessClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject: "not-a-uuid",
		},
	}

	_, err := claims.UserIDUUID()
	assert.ErrorIs(t, err, jwtmanager.ErrUserID)
}

func TestRefreshClaims_UserIDUUID_Success(t *testing.T) {
	expected := uuid.New()
	claims := &jwtmanager.RefreshClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject: expected.String(),
		},
	}

	got, err := claims.UserIDUUID()
	require.NoError(t, err)
	assert.Equal(t, expected, got)
}

func TestRotateKey_Success(t *testing.T) {
	mgr, oldKey := newTestManager(t)
	newKey := generateTestKey(t)
	userID := uuid.New()

	tokenBefore, err := mgr.GenerateAccess(userID)
	require.NoError(t, err)

	err = mgr.RotateKey(newKey)
	require.NoError(t, err)

	tokenAfter, err := mgr.GenerateAccess(userID)
	require.NoError(t, err)

	claims1 := &jwtmanager.AccessClaims{}
	parseToken(t, tokenBefore, &oldKey.PublicKey, claims1)

	claims2 := &jwtmanager.AccessClaims{}
	parseToken(t, tokenAfter, &newKey.PublicKey, claims2)

	claims3 := &jwtmanager.AccessClaims{}
	_, err = jwt.ParseWithClaims(tokenAfter, claims3, func(_ *jwt.Token) (interface{}, error) {
		return &oldKey.PublicKey, nil
	})
	assert.Error(t, err, "токен с новым ключом не должен валидироваться старым")
}

func TestRotateKey_NilKey(t *testing.T) {
	mgr, _ := newTestManager(t)
	err := mgr.RotateKey(nil)
	assert.ErrorIs(t, err, jwtmanager.ErrEmptyPrivateKey)
}

func TestRotateIssuer_Success(t *testing.T) {
	mgr, key := newTestManager(t)
	userID := uuid.New()

	err := mgr.RotateIssuer("new-issuer")
	require.NoError(t, err)

	tokenString, err := mgr.GenerateAccess(userID)
	require.NoError(t, err)

	claims := &jwtmanager.AccessClaims{}
	parseToken(t, tokenString, &key.PublicKey, claims)

	assert.Equal(t, "new-issuer", claims.Issuer)
}

func TestRotateIssuer_Empty(t *testing.T) {
	mgr, _ := newTestManager(t)
	err := mgr.RotateIssuer("")
	assert.ErrorIs(t, err, jwtmanager.ErrEmptyIssuer)
}

func TestConcurrent_GenerateAndRotate(t *testing.T) {
	mgr, _ := newTestManager(t)
	userID := uuid.New()

	var wg sync.WaitGroup
	const goroutines = 100

	for range goroutines / 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _, _, err := mgr.GenerateTokens(userID)
			assert.NoError(t, err)
		}()
	}

	for range goroutines / 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			newKey := generateTestKey(t)
			err := mgr.RotateKey(newKey)
			assert.NoError(t, err)
		}()
	}

	wg.Wait()
}
