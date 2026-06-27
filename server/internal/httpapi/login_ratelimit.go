package httpapi

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	loginRateLimitMax     = 5
	loginRateLimitWindow  = 15 * time.Minute
	loginRateLimitedMsg   = "Too many login attempts. Please try again later."
)

type loginRateLimiter struct {
	mu       sync.Mutex
	attempts map[string][]time.Time
	max      int
	window   time.Duration
}

func newLoginRateLimiter(max int, window time.Duration) *loginRateLimiter {
	return &loginRateLimiter{
		attempts: make(map[string][]time.Time),
		max:      max,
		window:   window,
	}
}

func (l *loginRateLimiter) pruneLocked(now time.Time, ip string) []time.Time {
	cutoff := now.Add(-l.window)
	attempts := l.attempts[ip]
	recent := attempts[:0]
	for _, t := range attempts {
		if t.After(cutoff) {
			recent = append(recent, t)
		}
	}
	if len(recent) == 0 {
		delete(l.attempts, ip)
		return nil
	}
	l.attempts[ip] = recent
	return recent
}

func (l *loginRateLimiter) allow(ip string) (ok bool, retryAfter time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	recent := l.pruneLocked(now, ip)
	if len(recent) >= l.max {
		retryAfter = recent[0].Add(l.window).Sub(now)
		if retryAfter < time.Second {
			retryAfter = time.Second
		}
		return false, retryAfter
	}
	return true, 0
}

func (l *loginRateLimiter) recordFailure(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	recent := l.pruneLocked(now, ip)
	l.attempts[ip] = append(recent, now)
}

func (l *loginRateLimiter) reset(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.attempts, ip)
}

func (s *Server) clientIP(r *http.Request) string {
	return clientIPFromRequest(r, s.trustedProxies)
}

func clientIPFromRequest(r *http.Request, trustedProxies []string) string {
	remote := remoteHost(r)
	if !isTrustedProxy(remote, trustedProxies) {
		return remote
	}
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return strings.TrimSpace(strings.Split(xff, ",")[0])
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}
	return remote
}

func remoteHost(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func isTrustedProxy(remote string, trustedProxies []string) bool {
	if len(trustedProxies) == 0 {
		return false
	}
	remoteIP := net.ParseIP(remote)
	if remoteIP == nil {
		return false
	}
	for _, trusted := range trustedProxies {
		if ip := net.ParseIP(strings.TrimSpace(trusted)); ip != nil && ip.Equal(remoteIP) {
			return true
		}
	}
	return false
}
