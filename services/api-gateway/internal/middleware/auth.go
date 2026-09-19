package middleware

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/Krokozabra213/e-commerce_shop/infra/httpx"
	jwtmanager "github.com/Krokozabra213/e-commerce_shop/infra/jwt/manager"
	"github.com/golang-jwt/jwt/v5"
)

type Validator interface {
	ValidateAccess(tokenString string) (*jwtmanager.AccessClaims, error)
}

func Auth(
	v Validator,
	log *slog.Logger,
) func(http.Handler) http.Handler {

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			accessToken := extractBearerToken(r)
			if accessToken == "" {
				http.Error(w, "missing authorization header", http.StatusUnauthorized)
				return
			}

			claims, err := v.ValidateAccess(accessToken)
			if err == nil {
				ctx := ContextWithClaims(r.Context(), claims)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			if isExpiredError(err) {
				log.Debug("auth: token expired",
					slog.String("remote", r.RemoteAddr),
				)
				respondTokenExpired(w)
				return
			}

			log.Warn("auth: invalid token",
				slog.String("error", err.Error()),
				slog.String("remote", r.RemoteAddr),
			)
			respondUnauthorized(w, "invalid_token", "Token validation failed")
		})
	}
}

func extractBearerToken(r *http.Request) string {
	header := r.Header.Get(httpx.HeaderAuthorization)
	if header == "" {
		return ""
	}
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], httpx.AuthSchemeBearer) {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

func isExpiredError(err error) bool {
	return errors.Is(err, jwt.ErrTokenExpired)
}

type ErrorResponse struct {
	Error       string `json:"error"`
	Description string `json:"error_description,omitempty"`
}

func respondUnauthorized(w http.ResponseWriter, code, description string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)

	_ = json.NewEncoder(w).Encode(ErrorResponse{
		Error:       code,
		Description: description,
	})
}

func respondTokenExpired(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Token-Status", "expired")
	w.WriteHeader(http.StatusUnauthorized)

	_ = json.NewEncoder(w).Encode(ErrorResponse{
		Error:       "token_expired",
		Description: "Access token has expired. Please refresh.",
	})
}
