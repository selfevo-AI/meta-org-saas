package middleware

import (
	"fmt"
	"net"
	"net/http"
	"strings"
)

type ClientIPResolver struct {
	trustedProxies []*net.IPNet
}

func NewClientIPResolver(cidrs []string) (*ClientIPResolver, error) {
	resolver := &ClientIPResolver{}
	for _, raw := range cidrs {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		_, network, err := net.ParseCIDR(raw)
		if err != nil {
			return nil, fmt.Errorf("parse trusted proxy CIDR %q: %w", raw, err)
		}
		resolver.trustedProxies = append(resolver.trustedProxies, network)
	}
	return resolver, nil
}

func (r *ClientIPResolver) Resolve(request *http.Request) string {
	peer := remoteIP(request.RemoteAddr)
	if peer == nil {
		return "unknown"
	}
	if !r.isTrusted(peer) {
		return peer.String()
	}
	// The peer is a trusted proxy. Proxies append the address they received
	// the request from to X-Forwarded-For, so the rightmost entry that is not
	// itself a trusted proxy is the real client. Leftmost entries are
	// client-supplied and trivially spoofable — trusting them lets callers
	// mint fresh rate-limit identities per request.
	entries := strings.Split(request.Header.Get("X-Forwarded-For"), ",")
	for index := len(entries) - 1; index >= 0; index-- {
		ip := net.ParseIP(strings.TrimSpace(entries[index]))
		if ip == nil || r.isTrusted(ip) {
			continue
		}
		return ip.String()
	}
	if ip := net.ParseIP(strings.TrimSpace(request.Header.Get("X-Real-IP"))); ip != nil {
		return ip.String()
	}
	return peer.String()
}

func (r *ClientIPResolver) isTrusted(ip net.IP) bool {
	if r == nil {
		return false
	}
	for _, network := range r.trustedProxies {
		if network.Contains(ip) {
			return true
		}
	}
	return false
}

func remoteIP(remoteAddr string) net.IP {
	host, _, err := net.SplitHostPort(strings.TrimSpace(remoteAddr))
	if err == nil {
		return net.ParseIP(host)
	}
	return net.ParseIP(strings.Trim(strings.TrimSpace(remoteAddr), "[]"))
}
