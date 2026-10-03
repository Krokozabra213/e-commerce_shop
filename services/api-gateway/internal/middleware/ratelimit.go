package middleware

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"strconv"
	"time"

	infracfg "github.com/Krokozabra213/e-commerce_shop/infra/config"
	"github.com/Krokozabra213/e-commerce_shop/infra/httpx"
	"github.com/Krokozabra213/e-commerce_shop/infra/ratelimit"
	"github.com/gofiber/fiber/v3"
)

type Limiter interface {
	Allow(ctx context.Context, key string, rule infracfg.RateLimitRule) (*ratelimit.Result, error)
}

func NewRateLimitMiddleware(limiter Limiter, cfg infracfg.RateLimitConfig, log *slog.Logger) fiber.Handler {
	return func(c fiber.Ctx) error {
		if !cfg.Enabled {
			return c.Next()
		}

		subject, isAuth := extractSubject(c)

		var rule infracfg.RateLimitRule
		var keyPrefix string

		if isAuth {
			rule = cfg.Authenticated
			keyPrefix = "user"
		} else {
			rule = cfg.Anonymous
			keyPrefix = "anon"
		}

		redisKey := fmt.Sprintf("%s:%s:%s", cfg.KeyPrefix, keyPrefix, subject)

		result, err := limiter.Allow(c.Context(), redisKey, rule)
		if err != nil {
			if cfg.FailOpen {
				log.Warn("ratelimit: redis error, failing open",
					slog.String("error", err.Error()),
					slog.String("subject", subject),
				)
				return c.Next()
			}
			return fiber.NewError(fiber.StatusServiceUnavailable, "service unavailable")
		}

		c.Set(httpx.HeaderRateLimitLimit, strconv.Itoa(result.Limit))
		c.Set(httpx.HeaderRateLimitRemaining, strconv.Itoa(result.Remaining))

		if !result.Allowed {
			retryAfterSec := int(result.RetryAfter.Seconds()) + 1
			c.Set(httpx.HeaderRateLimitRetryAfter, strconv.Itoa(retryAfterSec))
			c.Set(httpx.HeaderRateLimitReset,
				strconv.FormatInt(time.Now().Add(result.RetryAfter).Unix(), 10),
			)
			return fiber.NewError(fiber.StatusTooManyRequests, "too many requests")
		}

		return c.Next()
	}
}

func extractSubject(c fiber.Ctx) (string, bool) {
	userID, ok := UserIDFromCtx(c)
	if ok {
		return userID.String(), true
	}

	return clientIP(c), false
}

func clientIP(c fiber.Ctx) string {
	// X-Real-IP ставится nginx перед API Gateway
	if ip := c.Get(httpx.HeaderRealIP); ip != "" {
		return ip
	}

	// X-Forwarded-For
	if xff := c.Get("X-Forwarded-For"); xff != "" {
		// Берём первый IP из цепочки
		if idx := len(xff); idx > 0 {
			for i := 0; i < len(xff); i++ {
				if xff[i] == ',' {
					return xff[:i]
				}
			}
			return xff
		}
	}

	host, _, err := net.SplitHostPort(c.IP())
	if err != nil {
		return c.IP()
	}
	return host
}
