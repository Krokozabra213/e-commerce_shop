package middleware

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"time"

	infracfg "github.com/Krokozabra213/e-commerce_shop/infra/config"
	"github.com/Krokozabra213/e-commerce_shop/infra/httpx"
	"github.com/Krokozabra213/e-commerce_shop/infra/ratelimit"
)

type SubjectExtractor func(r *http.Request) (subject string, isAuth bool)

type Limiter interface {
	Allow(ctx context.Context, key string, rule infracfg.RateLimitRule) (*ratelimit.Result, error)
}

func RateLimit(l Limiter, extractor SubjectExtractor, cfg infracfg.RateLimitConfig, log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !cfg.Enabled {
				next.ServeHTTP(w, r)
				return
			}

			subject, isAuth := extractor(r)

			var rule infracfg.RateLimitRule
			var keyPrefix string

			if isAuth {
				rule = cfg.Authenticated
				keyPrefix = "user"
			} else {
				rule = cfg.Anonymous
				keyPrefix = "anon"
			}

			redisKey := fmt.Sprintf("api-gateway:%s:%s", keyPrefix, subject)

			result, err := l.Allow(r.Context(), redisKey, rule)
			if err != nil {
				if cfg.FailOpen {
					log.Warn("ratelimit: redis error, failing open",
						slog.String("error", err.Error()),
						slog.String("subject", subject),
					)
					next.ServeHTTP(w, r)
					return
				}
				http.Error(w, "service unavailable", http.StatusServiceUnavailable)
				return
			}

			w.Header().Set(httpx.HeaderRateLimitLimit, strconv.Itoa(result.Limit))
			w.Header().Set(httpx.HeaderRateLimitRemaining, strconv.Itoa(result.Remaining))

			if !result.Allowed {
				retryAfterSec := int(result.RetryAfter.Seconds()) + 1
				w.Header().Set(httpx.HeaderRateLimitRetryAfter, strconv.Itoa(retryAfterSec))
				w.Header().Set(httpx.HeaderRateLimitReset,
					strconv.FormatInt(time.Now().Add(result.RetryAfter).Unix(), 10),
				)
				http.Error(w, "too many requests", http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func ExtractorFromContext() SubjectExtractor {
	return func(r *http.Request) (string, bool) {
		claims, ok := ClaimsFromContext(r.Context())
		if ok && claims != nil {
			return claims.Subject, true
		}

		return clientIP(r), false
	}
}

func clientIP(r *http.Request) string {
	// X-Real-IP ставится nginx перед API Gateway
	if ip := r.Header.Get(httpx.HeaderRealIP); ip != "" {
		return ip
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
