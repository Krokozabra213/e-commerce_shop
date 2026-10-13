package middleware

import (
	"net"
	"testing"

	"github.com/Krokozabra213/e-commerce_shop/infra/httpx"
	"github.com/gofiber/fiber/v3"
)

func headerGetter(headers map[string]string) func(string) string {
	return func(key string) string {
		return headers[key]
	}
}

func TestClientIP(t *testing.T) {
	t.Parallel()

	trusted := parseTrustedProxies([]string{"10.42.0.0/16", "127.0.0.1"}, nil)

	tests := []struct {
		name    string
		peer    string
		headers map[string]string
		want    string
	}{
		{
			name:    "untrusted peer ignores spoofed X-Real-IP",
			peer:    "203.0.113.10",
			headers: map[string]string{httpx.HeaderRealIP: "1.2.3.4"},
			want:    "203.0.113.10",
		},
		{
			name:    "untrusted peer ignores spoofed X-Forwarded-For",
			peer:    "203.0.113.10",
			headers: map[string]string{fiber.HeaderXForwardedFor: "1.2.3.4"},
			want:    "203.0.113.10",
		},
		{
			name:    "trusted proxy peer uses X-Real-IP",
			peer:    "10.42.0.7",
			headers: map[string]string{httpx.HeaderRealIP: "198.51.100.23"},
			want:    "198.51.100.23",
		},
		{
			name: "trusted proxy peer takes rightmost untrusted X-Forwarded-For",
			peer: "10.42.0.7",
			headers: map[string]string{
				fiber.HeaderXForwardedFor: "1.2.3.4, 198.51.100.23, 10.42.0.9",
			},
			want: "198.51.100.23",
		},
		{
			name: "trusted proxy peer with invalid headers falls back to peer",
			peer: "10.42.0.7",
			headers: map[string]string{
				httpx.HeaderRealIP:        "not-an-ip",
				fiber.HeaderXForwardedFor: "also-garbage",
			},
			want: "10.42.0.7",
		},
		{
			name:    "trusted proxy peer without headers falls back to peer",
			peer:    "10.42.0.7",
			headers: map[string]string{},
			want:    "10.42.0.7",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := clientIP(tt.peer, headerGetter(tt.headers), trusted)
			if got != tt.want {
				t.Fatalf("clientIP() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestClientIP_NoTrustedProxies(t *testing.T) {
	t.Parallel()

	trusted := parseTrustedProxies(nil, nil)
	headers := map[string]string{
		httpx.HeaderRealIP:        "198.51.100.23",
		fiber.HeaderXForwardedFor: "198.51.100.23",
	}

	got := clientIP("10.42.0.7", headerGetter(headers), trusted)
	if got != "10.42.0.7" {
		t.Fatalf("clientIP() = %q, want peer address", got)
	}
}

func TestParseTrustedProxies(t *testing.T) {
	t.Parallel()

	nets := parseTrustedProxies([]string{"10.42.0.0/16", " 127.0.0.1 ", "", "not-a-cidr"}, nil)
	if len(nets) != 2 {
		t.Fatalf("parseTrustedProxies() returned %d nets, want 2", len(nets))
	}

	if !nets[0].Contains(net.ParseIP("10.42.1.5")) {
		t.Errorf("expected 10.42.1.5 to be inside 10.42.0.0/16")
	}
	if nets[0].Contains(net.ParseIP("10.43.1.5")) {
		t.Errorf("expected 10.43.1.5 to be outside 10.42.0.0/16")
	}
	if !nets[1].Contains(net.ParseIP("127.0.0.1")) {
		t.Errorf("expected 127.0.0.1 to be inside plain-IP trusted entry")
	}
}

func TestRightmostUntrustedIP(t *testing.T) {
	t.Parallel()

	trusted := parseTrustedProxies([]string{"10.42.0.0/16"}, nil)

	ip, ok := rightmostUntrustedIP("1.2.3.4, 198.51.100.23, 10.42.0.9", trusted)
	if !ok || ip != "198.51.100.23" {
		t.Fatalf("rightmostUntrustedIP() = (%q, %v), want (%q, true)", ip, ok, "198.51.100.23")
	}

	if _, ok = rightmostUntrustedIP("10.42.0.1, 10.42.0.2", trusted); ok {
		t.Errorf("expected no untrusted IP for all-trusted chain")
	}

	if _, ok = rightmostUntrustedIP("garbage, ,", trusted); ok {
		t.Errorf("expected no IP for malformed chain")
	}
}
