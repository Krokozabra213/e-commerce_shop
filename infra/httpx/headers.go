package httpx

const (
	HeaderRequestID = "X-Request-ID"

	HeaderRealIP    = "X-Real-IP"
	HeaderUserID    = "X-User-Id"
	HeaderUserRoles = "X-User-Roles"

	HeaderRateLimitLimit      = "X-RateLimit-Limit"
	HeaderRateLimitRemaining  = "X-RateLimit-Remaining"
	HeaderRateLimitRetryAfter = "X-RateLimit-RetryAfter"
	HeaderRateLimitReset      = "X-RateLimit-Reset"

	HeaderAuthorization = "Authorization"
	AuthSchemeBearer    = "Bearer"
)
