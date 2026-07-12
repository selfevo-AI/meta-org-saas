package middleware

import (
	"net/http/httptest"
	"testing"
)

func TestClientIPResolverIgnoresForwardedHeadersFromUntrustedPeer(t *testing.T) {
	resolver, err := NewClientIPResolver(nil)
	if err != nil {
		t.Fatalf("new resolver: %v", err)
	}
	request := httptest.NewRequest("GET", "/", nil)
	request.RemoteAddr = "203.0.113.10:1234"
	request.Header.Set("X-Forwarded-For", "198.51.100.20")

	if got := resolver.Resolve(request); got != "203.0.113.10" {
		t.Fatalf("client IP = %q, want peer IP", got)
	}
}

func TestClientIPResolverUsesForwardedAddressFromTrustedProxy(t *testing.T) {
	resolver, err := NewClientIPResolver([]string{"10.0.0.0/8"})
	if err != nil {
		t.Fatalf("new resolver: %v", err)
	}
	request := httptest.NewRequest("GET", "/", nil)
	request.RemoteAddr = "10.1.2.3:4321"
	request.Header.Set("X-Forwarded-For", "198.51.100.20, 10.1.2.3")

	if got := resolver.Resolve(request); got != "198.51.100.20" {
		t.Fatalf("client IP = %q, want forwarded client", got)
	}
}

func TestClientIPResolverRejectsInvalidTrustedProxyCIDR(t *testing.T) {
	if _, err := NewClientIPResolver([]string{"not-a-cidr"}); err == nil {
		t.Fatal("expected invalid trusted proxy CIDR to fail")
	}
}
