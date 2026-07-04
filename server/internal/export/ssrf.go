package export

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"syscall"
	"time"
)

// metadataIPs are well-known cloud metadata endpoints that must never be
// reachable from outbound webhook/API-export requests.
var metadataIPs = map[string]bool{
	"169.254.169.254": true, // AWS / Azure / GCP / OpenStack metadata
	"fd00:ec2::254":   true, // AWS IMDSv6
}

// isForbiddenIP reports whether an IP is internal/loopback/link-local or a
// known cloud metadata endpoint. Any such IP is blocked for outbound requests.
func isForbiddenIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsUnspecified() {
		return true
	}
	if metadataIPs[strings.ToLower(ip.String())] {
		return true
	}
	return false
}

// ValidateExternalURL parses and validates an outbound URL for webhook/API
// export use. It rejects non-http(s) schemes, empty hosts, literal internal
// IPs, and — crucially — hostnames that resolve (via the default resolver) to
// any forbidden IP. The DNS resolution closes the nip.io / DNS-rebinding-via-
// hostname gap that a literal-host check alone leaves open.
//
// This is the validation-time layer; SSRFSafeHTTPClient adds a dial-time layer
// to defeat rebinding between validation and connect.
func ValidateExternalURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("invalid API URL: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("unsupported API URL scheme: %s", u.Scheme)
	}
	host := strings.ToLower(u.Hostname())
	if host == "" {
		return fmt.Errorf("API URL host is required")
	}
	if host == "localhost" || host == "metadata" || host == "metadata.google.internal" {
		return fmt.Errorf("internal API URLs are not allowed")
	}
	// Literal IP host: check directly without DNS.
	if ip := net.ParseIP(host); ip != nil {
		if isForbiddenIP(ip) {
			return fmt.Errorf("internal API URLs are not allowed")
		}
		return nil
	}
	// Hostname: resolve and reject if any resolved IP is forbidden.
	ips, err := lookupIPAddr(context.Background(), host)
	if err != nil {
		return fmt.Errorf("API URL host could not be resolved: %w", err)
	}
	for _, ipAddr := range ips {
		if isForbiddenIP(ipAddr.IP) {
			return fmt.Errorf("API URL host resolves to an internal IP (%s); internal API URLs are not allowed", ipAddr.IP)
		}
	}
	return nil
}

// lookupIPAddr is the resolver used by ValidateExternalURL. It is a variable so
// tests can inject a fake resolver (the real one would need network/DNS access
// and make tests flaky).
var lookupIPAddr = func(ctx context.Context, host string) ([]net.IPAddr, error) {
	return net.DefaultResolver.LookupIPAddr(ctx, host)
}

// ValidateWebhookURL validates a user-configured webhook URL. Webhooks must use
// HTTPS (plain http is rejected) and must pass the same SSRF checks as
// ValidateExternalURL.
func ValidateWebhookURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("invalid webhook URL: %w", err)
	}
	if u.Scheme != "https" {
		return fmt.Errorf("webhook URL must use https (got %q)", u.Scheme)
	}
	return ValidateExternalURL(raw)
}

// SSRFSafeHTTPClient returns an *http.Client whose dialer re-checks the
// resolved IP at connect time via net.Dialer.Control. This blocks DNS
// rebinding: even if validation saw a public IP, a second resolution at dial
// time that returns an internal IP is rejected before the TCP connect.
func SSRFSafeHTTPClient(timeout time.Duration) *http.Client {
	dialer := &net.Dialer{
		Timeout: 30 * time.Second,
		Control: func(network, address string, _ syscall.RawConn) error {
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return err
			}
			ip := net.ParseIP(host)
			if ip == nil {
				return fmt.Errorf("SSRF guard: non-IP address after dial resolution: %s", host)
			}
			if isForbiddenIP(ip) {
				return fmt.Errorf("SSRF guard: dial to internal IP %s blocked", ip)
			}
			return nil
		},
	}
	return &http.Client{
		Timeout:   timeout,
		Transport: &http.Transport{DialContext: dialer.DialContext},
	}
}
