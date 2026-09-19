package middleware

import (
	"context"

	jwtmanager "github.com/Krokozabra213/e-commerce_shop/infra/jwt/manager"
)

type contextKey struct{}

var claimsKey = contextKey{}

func ContextWithClaims(ctx context.Context, claims *jwtmanager.AccessClaims) context.Context {
	return context.WithValue(ctx, claimsKey, claims)
}

func ClaimsFromContext(ctx context.Context) (*jwtmanager.AccessClaims, bool) {
	claims, ok := ctx.Value(claimsKey).(*jwtmanager.AccessClaims)
	return claims, ok
}
