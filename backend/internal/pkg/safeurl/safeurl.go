// Package safeurl validates operator-supplied outbound URLs and builds HTTP
// clients that refuse to connect to internal networks, so a tenant cannot use
// the server as a proxy into the database, the security kernel, or a cloud
// metadata endpoint.
package safeurl

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var (
	ErrInvalidURL              = errors.New("invalid outbound url")
	ErrDisallowedDestination   = errors.New("outbound destination is not allowed")
	ErrUnresolvableDestination = errors.New("outbound destination could not be resolved")
)

// Policy decides which destinations outbound requests may reach.
type Policy struct {
	// AllowPrivate permits loopback, private, link-local and metadata
	// addresses. It is meant for development and test environments only.
	AllowPrivate bool
	// Resolver overrides DNS lookups (tests). nil uses net.DefaultResolver.
	Resolver interface {
		LookupIPAddr(ctx context.Context, host string) ([]net.IPAddr, error)
	}
}

// blockedHostnames are names that bypass IP classification on some
// platforms; they are refused outright unless private access is allowed.
var blockedHostnames = map[string]bool{
	"localhost":                true,
	"metadata":                 true,
	"metadata.google.internal": true,
	"instance-data":            true,
}

// Validate parses raw, requires an absolute http(s) URL with a host and no
// embedded credentials, and refuses destinations the policy does not allow.
// Hostnames are resolved so a public name pointing at an internal address is
// caught at configuration time; Transport re-checks at dial time.
func (p Policy) Validate(ctx context.Context, raw string) (*url.URL, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidURL, err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("%w: scheme must be http or https", ErrInvalidURL)
	}
	if parsed.Host == "" || parsed.Hostname() == "" {
		return nil, fmt.Errorf("%w: host is required", ErrInvalidURL)
	}
	if parsed.User != nil {
		return nil, fmt.Errorf("%w: credentials in the url are not allowed", ErrInvalidURL)
	}
	if p.AllowPrivate {
		return parsed, nil
	}
	host := strings.ToLower(strings.TrimSuffix(parsed.Hostname(), "."))
	if blockedHostnames[host] || strings.HasSuffix(host, ".localhost") || strings.HasSuffix(host, ".internal") {
		return nil, fmt.Errorf("%w: %s", ErrDisallowedDestination, host)
	}
	if ip := net.ParseIP(host); ip != nil {
		if IsDisallowedIP(ip) {
			return nil, fmt.Errorf("%w: %s", ErrDisallowedDestination, host)
		}
		return parsed, nil
	}
	addrs, err := p.resolver().LookupIPAddr(ctx, host)
	if err != nil || len(addrs) == 0 {
		return nil, fmt.Errorf("%w: %s", ErrUnresolvableDestination, host)
	}
	for _, addr := range addrs {
		if IsDisallowedIP(addr.IP) {
			return nil, fmt.Errorf("%w: %s resolves to %s", ErrDisallowedDestination, host, addr.IP)
		}
	}
	return parsed, nil
}

// IsDisallowedIP reports whether ip is loopback, private, link-local,
// unspecified, multicast, or a well-known cloud metadata address.
func IsDisallowedIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsUnspecified() || ip.IsMulticast() || ip.IsInterfaceLocalMulticast() {
		return true
	}
	if v4 := ip.To4(); v4 != nil {
		switch {
		case v4[0] == 100 && v4[1]&0xc0 == 64: // 100.64.0.0/10 carrier-grade NAT
			return true
		case v4[0] == 192 && v4[1] == 0 && v4[2] == 0: // 192.0.0.0/24 IETF protocol assignments
			return true
		case v4[0] == 198 && v4[1]&0xfe == 18: // 198.18.0.0/15 benchmarking
			return true
		}
		return false
	}
	// NAT64 (64:ff9b::/96) embeds an IPv4 address we must classify; plain
	// IPv4-mapped addresses were already unwrapped by To4 above.
	if len(ip) == net.IPv6len && ip[0] == 0x00 && ip[1] == 0x64 && ip[2] == 0xff && ip[3] == 0x9b {
		return IsDisallowedIP(net.IPv4(ip[12], ip[13], ip[14], ip[15]))
	}
	if ip[0]&0xfe == 0xfc { // fc00::/7 unique local
		return true
	}
	return false
}

// Transport returns an http.Transport whose dialer resolves the host itself
// and refuses disallowed addresses at connect time, so a DNS answer that
// changes after Validate cannot redirect the request inward.
func (p Policy) Transport() *http.Transport {
	dialer := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}
		if p.AllowPrivate {
			return dialer.DialContext(ctx, network, address)
		}
		lower := strings.ToLower(strings.TrimSuffix(host, "."))
		if blockedHostnames[lower] || strings.HasSuffix(lower, ".localhost") || strings.HasSuffix(lower, ".internal") {
			return nil, fmt.Errorf("%w: %s", ErrDisallowedDestination, host)
		}
		var candidates []net.IP
		if ip := net.ParseIP(host); ip != nil {
			candidates = []net.IP{ip}
		} else {
			addrs, err := p.resolver().LookupIPAddr(ctx, host)
			if err != nil {
				return nil, fmt.Errorf("%w: %s", ErrUnresolvableDestination, host)
			}
			for _, addr := range addrs {
				candidates = append(candidates, addr.IP)
			}
		}
		var lastErr error
		for _, ip := range candidates {
			if IsDisallowedIP(ip) {
				lastErr = fmt.Errorf("%w: %s resolves to %s", ErrDisallowedDestination, host, ip)
				continue
			}
			conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
			if err == nil {
				return conn, nil
			}
			lastErr = err
		}
		if lastErr == nil {
			lastErr = fmt.Errorf("%w: %s", ErrUnresolvableDestination, host)
		}
		return nil, lastErr
	}
	return transport
}

// Client returns an http.Client on Transport() that also re-validates every
// redirect target, since a permitted host may 302 to an internal one.
func (p Policy) Client(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout:   timeout,
		Transport: p.Transport(),
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return errors.New("stopped after 5 redirects")
			}
			_, err := p.Validate(req.Context(), req.URL.String())
			return err
		},
	}
}

func (p Policy) resolver() interface {
	LookupIPAddr(ctx context.Context, host string) ([]net.IPAddr, error)
} {
	if p.Resolver != nil {
		return p.Resolver
	}
	return net.DefaultResolver
}
