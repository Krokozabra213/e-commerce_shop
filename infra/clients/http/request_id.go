package httpclient

import (
	"net/http"

	"github.com/Krokozabra213/e-commerce_shop/infra/httpx"
	"github.com/Krokozabra213/e-commerce_shop/infra/logger"
)

type requestIDRoundTripper struct {
	base http.RoundTripper
}

func WithRequestID(base http.RoundTripper) http.RoundTripper {
	if base == nil {
		base = http.DefaultTransport
	}
	return &requestIDRoundTripper{base: base}
}

func (t *requestIDRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Header.Get(httpx.HeaderRequestID) == "" {
		if id := logger.RequestIDFromContext(req.Context()); id != "" {
			req.Header.Set(httpx.HeaderRequestID, id)
		}
	}
	return t.base.RoundTrip(req)
}
