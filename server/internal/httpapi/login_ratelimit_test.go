package httpapi

import (
	"net/http/httptest"
	"testing"
	"time"
)

func TestLoginRateLimiterBlocksAfterMaxFailures(t *testing.T) {
	limiter := newLoginRateLimiter(3, time.Minute)
	ip := "203.0.113.1"

	for i := 0; i < 3; i++ {
		ok, _ := limiter.allow(ip)
		if !ok {
			t.Fatalf("attempt %d: expected allow before limit reached", i+1)
		}
		limiter.recordFailure(ip)
	}

	ok, retryAfter := limiter.allow(ip)
	if ok {
		t.Fatal("expected block after max failures")
	}
	if retryAfter <= 0 {
		t.Fatalf("retryAfter = %v, want > 0", retryAfter)
	}
}

func TestLoginRateLimiterResetClearsFailures(t *testing.T) {
	limiter := newLoginRateLimiter(2, time.Minute)
	ip := "203.0.113.2"

	limiter.recordFailure(ip)
	limiter.recordFailure(ip)
	if ok, _ := limiter.allow(ip); ok {
		t.Fatal("expected block before reset")
	}

	limiter.reset(ip)
	if ok, _ := limiter.allow(ip); !ok {
		t.Fatal("expected allow after reset")
	}
}

func TestClientIPIgnoresForwardedForWithoutTrustedProxy(t *testing.T) {
	req := httptest.NewRequest("POST", "/api/v1/login", nil)
	req.Header.Set("X-Forwarded-For", "203.0.113.10, 10.0.0.1")
	req.RemoteAddr = "203.0.113.50:1234"

	if got := clientIPFromRequest(req, nil); got != "203.0.113.50" {
		t.Fatalf("clientIP = %q, want direct remote 203.0.113.50", got)
	}
}

func TestClientIPUsesForwardedForFromTrustedProxy(t *testing.T) {
	req := httptest.NewRequest("POST", "/api/v1/login", nil)
	req.Header.Set("X-Forwarded-For", "203.0.113.10, 10.0.0.1")
	req.RemoteAddr = "127.0.0.1:1234"

	if got := clientIPFromRequest(req, []string{"127.0.0.1"}); got != "203.0.113.10" {
		t.Fatalf("clientIP = %q, want 203.0.113.10", got)
	}
}

func TestClientIPUsesRealIPFromTrustedProxy(t *testing.T) {
	req := httptest.NewRequest("POST", "/api/v1/login", nil)
	req.Header.Set("X-Real-IP", "203.0.113.20")
	req.RemoteAddr = "127.0.0.1:1234"

	if got := clientIPFromRequest(req, []string{"127.0.0.1"}); got != "203.0.113.20" {
		t.Fatalf("clientIP = %q, want 203.0.113.20", got)
	}
}
