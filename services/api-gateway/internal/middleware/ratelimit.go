package middleware

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"strconv"
	"strings"
	"time"

	infracfg "github.com/Krokozabra213/e-commerce_shop/infra/config"
	"github.com/Krokozabra213/e-commerce_shop/infra/httpx"
	"github.com/Krokozabra213/e-commerce_shop/infra/ratelimit"
	"github.com/gofiber/fiber/v3"
)

type Limiter interface {
	Allow(ctx context.Context, key string, rule infracfg.RateLimitRule) (*ratelimit.Result, error)
}

// NewRateLimitMiddleware создаёт лимитер. trustedProxies — список IP/CIDR прокси
// (например, pod-сеть ingress-nginx), чьему заголовку X-Real-IP можно доверять.
// Если список пуст, клиент определяется только по TCP-адресу соединения.
func NewRateLimitMiddleware(
	limiter Limiter,
	cfg infracfg.RateLimitConfig,
	trustedProxies []string,
	log *slog.Logger,
) fiber.Handler {
	trustedNets := parseTrustedProxies(trustedProxies, log)

	return func(c fiber.Ctx) error {
		if !cfg.Enabled {
			return c.Next()
		}

		subject, isAuth := extractSubject(c, trustedNets)

		var rule infracfg.RateLimitRule
		var keyPrefix string

		if isAuth {
			rule = cfg.Authenticated
			keyPrefix = "user"
		} else {
			rule = cfg.Anonymous
			keyPrefix = "anon"
		}

		redisKey := fmt.Sprintf("%s:%s", keyPrefix, subject)

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

func extractSubject(c fiber.Ctx, trusted []*net.IPNet) (string, bool) {
	userID, ok := UserIDFromCtx(c)
	if ok {
		return userID.String(), true
	}

	return clientIP(peerIP(c), func(key string) string { return c.Get(key) }, trusted), false
}

// peerIP возвращает адрес, с которого реально пришло TCP-соединение. Подделать его
// нельзя, в отличие от заголовков (ProxyHeader в конфиге Fiber не задан).
func peerIP(c fiber.Ctx) string {
	if ip := parseIP(c.IP()); ip != nil {
		return ip.String()
	}
	return c.IP()
}

// clientIP определяет адрес клиента для ключа лимита. Заголовкам X-Real-IP и
// X-Forwarded-For доверяем только тогда, когда соединение пришло от доверенного
// прокси; иначе используем TCP-адрес, иначе лимит обходится подменой заголовка.
func clientIP(peer string, get func(string) string, trusted []*net.IPNet) string {
	if !isTrustedProxy(peer, trusted) {
		return peer
	}

	// X-Real-IP проставляет наш прокси (ingress-nginx), поэтому ему верим первым.
	if ip := parseIP(get(httpx.HeaderRealIP)); ip != nil {
		return ip.String()
	}

	// X-Forwarded-For читаем справа налево, пропуская доверенные прокси: так мы
	// берём адрес, добавленный прокси, а не подставленный клиентом.
	if xff := get(fiber.HeaderXForwardedFor); xff != "" {
		if ip, ok := rightmostUntrustedIP(xff, trusted); ok {
			return ip
		}
	}

	return peer
}

func parseTrustedProxies(raw []string, log *slog.Logger) []*net.IPNet {
	nets := make([]*net.IPNet, 0, len(raw))

	for _, entry := range raw {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}

		if _, n, err := net.ParseCIDR(entry); err == nil {
			nets = append(nets, n)
			continue
		}

		if ip := net.ParseIP(entry); ip != nil {
			bits := 32
			if ip.To4() == nil {
				bits = 128
			}
			nets = append(nets, &net.IPNet{IP: ip, Mask: net.CIDRMask(bits, bits)})
			continue
		}

		if log != nil {
			log.Warn("ratelimit: invalid trusted proxy, ignored",
				slog.String("entry", entry),
			)
		}
	}

	return nets
}

func isTrustedProxy(peer string, trusted []*net.IPNet) bool {
	ip := parseIP(peer)
	if ip == nil {
		return false
	}
	return ipInAnyNet(ip, trusted)
}

func rightmostUntrustedIP(xff string, trusted []*net.IPNet) (string, bool) {
	parts := strings.Split(xff, ",")

	for i := len(parts) - 1; i >= 0; i-- {
		ip := parseIP(parts[i])
		if ip == nil {
			continue
		}
		if !ipInAnyNet(ip, trusted) {
			return ip.String(), true
		}
	}

	return "", false
}

func ipInAnyNet(ip net.IP, nets []*net.IPNet) bool {
	for _, n := range nets {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

// parseIP разбирает IP-адрес, допуская необязательный порт ("1.2.3.4:5678").
func parseIP(s string) net.IP {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}

	if host, _, err := net.SplitHostPort(s); err == nil {
		s = host
	}

	return net.ParseIP(s)
}
