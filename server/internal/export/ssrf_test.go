package export

import (
	"context"
	"net"
	"testing"
)

func TestIsForbiddenIP(t *testing.T) {
	cases := []struct {
		ip   string
		want bool
	}{
		{"127.0.0.1", true},
		{"::1", true},
		{"10.0.0.1", true},
		{"192.168.1.1", true},
		{"172.16.0.1", true},
		{"169.254.169.254", true}, // metadata
		{"169.254.0.1", true},     // link-local
		{"0.0.0.0", true},
		{"224.0.0.1", true}, // link-local multicast
		{"8.8.8.8", false},
		{"93.184.216.34", false}, // example.com
	}
	for _, c := range cases {
		got := isForbiddenIP(net.ParseIP(c.ip))
		if got != c.want {
			t.Errorf("isForbiddenIP(%s) = %v, want %v", c.ip, got, c.want)
		}
	}
}

// TestT10_SSRFDNSRebinding verifies P1-5: a hostname that resolves to an
// internal/metadata IP is rejected by ValidateExternalURL even though the
// literal host string is not itself internal (the nip.io / DNS-rebinding-via-
// hostname gap). Uses an injectable resolver so the test is deterministic and
// does not need network access.
func TestT10_SSRFDNSRebinding(t *testing.T) {
	orig := lookupIPAddr
	defer func() { lookupIPAddr = orig }()
	lookupIPAddr = func(_ context.Context, host string) ([]net.IPAddr, error) {
		switch host {
		case "metadata.nip.io":
			return []net.IPAddr{{IP: net.ParseIP("169.254.169.254")}}, nil
		case "internal.test":
			return []net.IPAddr{{IP: net.ParseIP("10.0.0.5")}}, nil
		case "public.example":
			return []net.IPAddr{{IP: net.ParseIP("93.184.216.34")}}, nil
		case "mixed.test":
			return []net.IPAddr{
				{IP: net.ParseIP("93.184.216.34")},
				{IP: net.ParseIP("127.0.0.1")}, // one forbidden among public
			}, nil
		default:
			return nil, &net.DNSError{Err: "no such host", Name: host}
		}
	}

	for _, c := range []struct {
		url     string
		wantOK  bool
	}{
		{"https://metadata.nip.io/hook", false},
		{"https://internal.test/hook", false},
		{"https://mixed.test/hook", false},
		{"https://public.example/hook", true},
	} {
		err := ValidateExternalURL(c.url)
		if c.wantOK && err != nil {
			t.Errorf("ValidateExternalURL(%s): expected pass, got %v", c.url, err)
		}
		if !c.wantOK && err == nil {
			t.Errorf("ValidateExternalURL(%s): expected rejection (resolves to internal IP), got nil", c.url)
		}
	}

	// Webhook validation additionally requires HTTPS.
	if err := ValidateWebhookURL("http://public.example/hook"); err == nil {
		t.Error("ValidateWebhookURL http:// : expected rejection (HTTPS-only), got nil")
	}
	if err := ValidateWebhookURL("https://public.example/hook"); err != nil {
		t.Errorf("ValidateWebhookURL https public: %v", err)
	}
	if err := ValidateWebhookURL("https://metadata.nip.io/hook"); err == nil {
		t.Error("ValidateWebhookURL metadata host: expected rejection, got nil")
	}
}
