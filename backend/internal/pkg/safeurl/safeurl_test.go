package safeurl

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type fakeResolver map[string][]string

func (r fakeResolver) LookupIPAddr(_ context.Context, host string) ([]net.IPAddr, error) {
	ips, ok := r[host]
	if !ok {
		return nil, errors.New("no such host")
	}
	addrs := make([]net.IPAddr, 0, len(ips))
	for _, ip := range ips {
		addrs = append(addrs, net.IPAddr{IP: net.ParseIP(ip)})
	}
	return addrs, nil
}

func TestIsDisallowedIP(t *testing.T) {
	disallowed := []string{
		"127.0.0.1", "127.8.8.8", "10.0.0.1", "172.16.5.5", "172.31.255.255", "192.168.1.1",
		"169.254.169.254", "169.254.1.1", "0.0.0.0", "100.64.0.1", "100.127.255.255",
		"192.0.0.1", "198.18.0.1", "198.19.255.255", "224.0.0.1",
		"::1", "::", "fc00::1", "fd12::1", "fe80::1", "ff02::1",
		"::ffff:127.0.0.1", "::ffff:10.1.1.1", "64:ff9b::7f00:1", "64:ff9b::a00:1",
	}
	for _, raw := range disallowed {
		if !IsDisallowedIP(net.ParseIP(raw)) {
			t.Errorf("%s should be disallowed", raw)
		}
	}
	allowed := []string{"8.8.8.8", "1.1.1.1", "93.184.216.34", "172.32.0.1", "100.128.0.1", "2606:4700::1111", "64:ff9b::808:808"}
	for _, raw := range allowed {
		if IsDisallowedIP(net.ParseIP(raw)) {
			t.Errorf("%s should be allowed", raw)
		}
	}
	if !IsDisallowedIP(nil) {
		t.Error("nil ip should be disallowed")
	}
}

func TestValidateRejectsMalformedAndInternal(t *testing.T) {
	policy := Policy{Resolver: fakeResolver{
		"api.example.com":      {"93.184.216.34"},
		"rebind.example.com":   {"93.184.216.34", "10.0.0.5"},
		"internal.example.com": {"192.168.0.10"},
	}}
	cases := []struct {
		raw  string
		want error
	}{
		{"", ErrInvalidURL},
		{"ftp://api.example.com", ErrInvalidURL},
		{"api.example.com/path", ErrInvalidURL},
		{"http://", ErrInvalidURL},
		{"http://user:pass@api.example.com", ErrInvalidURL},
		{"http://localhost:8080", ErrDisallowedDestination},
		{"http://LOCALHOST", ErrDisallowedDestination},
		{"http://app.localhost", ErrDisallowedDestination},
		{"http://metadata.google.internal/computeMetadata/v1", ErrDisallowedDestination},
		{"http://security-kernel.internal:8090", ErrDisallowedDestination},
		{"http://127.0.0.1:5432", ErrDisallowedDestination},
		{"http://[::1]:8080", ErrDisallowedDestination},
		{"http://169.254.169.254/latest/meta-data", ErrDisallowedDestination},
		{"http://10.1.2.3", ErrDisallowedDestination},
		{"http://internal.example.com", ErrDisallowedDestination},
		{"http://rebind.example.com", ErrDisallowedDestination},
		{"http://unknown.example.com", ErrUnresolvableDestination},
		{"https://api.example.com/v1", nil},
		{"http://93.184.216.34:9000", nil},
	}
	for _, tc := range cases {
		t.Run(tc.raw, func(t *testing.T) {
			_, err := policy.Validate(context.Background(), tc.raw)
			if tc.want == nil && err != nil {
				t.Fatalf("Validate(%q) error = %v, want nil", tc.raw, err)
			}
			if tc.want != nil && !errors.Is(err, tc.want) {
				t.Fatalf("Validate(%q) error = %v, want %v", tc.raw, err, tc.want)
			}
		})
	}
}

func TestValidateAllowPrivateStillRejectsMalformed(t *testing.T) {
	policy := Policy{AllowPrivate: true}
	if _, err := policy.Validate(context.Background(), "http://127.0.0.1:8080"); err != nil {
		t.Fatalf("loopback should pass with AllowPrivate: %v", err)
	}
	if _, err := policy.Validate(context.Background(), "http://user:pw@127.0.0.1"); !errors.Is(err, ErrInvalidURL) {
		t.Fatalf("credentials must still be rejected: %v", err)
	}
	if _, err := policy.Validate(context.Background(), "gopher://127.0.0.1"); !errors.Is(err, ErrInvalidURL) {
		t.Fatalf("scheme must still be enforced: %v", err)
	}
}

func TestClientRefusesLoopbackAtDialTime(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	strict := Policy{}.Client(5 * time.Second)
	if _, err := strict.Get(server.URL); err == nil || !errors.Is(err, ErrDisallowedDestination) {
		t.Fatalf("strict client should refuse loopback, got %v", err)
	}

	permissive := Policy{AllowPrivate: true}.Client(5 * time.Second)
	resp, err := permissive.Get(server.URL)
	if err != nil {
		t.Fatalf("permissive client error = %v", err)
	}
	resp.Body.Close()
}

func TestClientRefusesHostResolvingInward(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	_, port, _ := net.SplitHostPort(server.Listener.Addr().String())

	// The name looks public but DNS answers with the loopback address the test
	// server listens on: the dialer must refuse rather than connect.
	policy := Policy{Resolver: fakeResolver{"edge.example.com": {"127.0.0.1"}}}
	client := policy.Client(5 * time.Second)
	_, err := client.Get("http://edge.example.com:" + port + "/")
	if err == nil || !errors.Is(err, ErrDisallowedDestination) {
		t.Fatalf("expected disallowed destination, got %v", err)
	}
}

func TestClientRevalidatesRedirects(t *testing.T) {
	internal := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer internal.Close()
	public := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, internal.URL, http.StatusFound)
	}))
	defer public.Close()
	_, publicPort, _ := net.SplitHostPort(public.Listener.Addr().String())

	// Permit the first hop by resolving the public name to the loopback test
	// listener through a permissive dial, then make sure the redirect to a raw
	// loopback URL is still refused by CheckRedirect.
	policy := Policy{AllowPrivate: true}
	client := policy.Client(5 * time.Second)
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		_, err := Policy{}.Validate(req.Context(), req.URL.String())
		return err
	}
	_, err := client.Get("http://127.0.0.1:" + publicPort + "/")
	if err == nil || !errors.Is(err, ErrDisallowedDestination) {
		t.Fatalf("redirect to loopback should be refused, got %v", err)
	}
}
