package gate

import (
	"net"
	"testing"
)

func TestValidatePublicHostnameLiteral(t *testing.T) {
	if err := ValidatePublicHostname("8.8.8.8"); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"127.0.0.1", "10.0.0.1", "192.168.1.1", "169.254.1.1", "100.64.0.1", "::1", ""} {
		if err := ValidatePublicHostname(bad); err == nil {
			t.Fatalf("expected block for %q", bad)
		}
	}
	if _, err := resolvePublicHostname("127.0.0.1", true); err != nil {
		t.Fatal(err)
	}
	if _, err := resolvePublicHostname("127.0.0.1", false); err == nil {
		t.Fatal("production path must still block loopback")
	}
}

func TestValidatePublicHostnameLookup(t *testing.T) {
	old := lookupIP
	defer func() { lookupIP = old }()
	lookupIP = func(host string) ([]net.IP, error) {
		if host == "evil.local" {
			return []net.IP{net.ParseIP("10.1.2.3")}, nil
		}
		if host == "ok.example" {
			return []net.IP{net.ParseIP("1.2.3.4")}, nil
		}
		return nil, net.UnknownNetworkError("x")
	}
	if err := ValidatePublicHostname("ok.example"); err != nil {
		t.Fatal(err)
	}
	if err := ValidatePublicHostname("evil.local"); err == nil {
		t.Fatal("expected private block")
	}
}
