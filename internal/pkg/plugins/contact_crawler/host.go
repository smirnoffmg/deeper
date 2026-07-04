package contact_crawler

import (
	"context"
	"net"
	"strings"
	"sync"

	"golang.org/x/net/publicsuffix"
)

type ipResolver interface {
	LookupIPAddr(ctx context.Context, host string) ([]net.IPAddr, error)
}

type siteChecker struct {
	resolver ipResolver
	cache    map[string][]net.IP
	mu       sync.Mutex
}

func newSiteChecker(resolver ipResolver) *siteChecker {
	return &siteChecker{
		resolver: resolver,
		cache:    make(map[string][]net.IP),
	}
}

func registrableDomain(host string) (string, bool) {
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "" {
		return "", false
	}

	domain, err := publicsuffix.EffectiveTLDPlusOne(host)
	if err != nil {
		return "", false
	}

	return domain, true
}

func (s *siteChecker) sameSite(seedHost, candidateHost string) bool {
	seedIPs := s.hostIPs(seedHost)
	if len(seedIPs) == 0 {
		return false
	}

	candidateIPs := s.hostIPs(candidateHost)
	if len(candidateIPs) == 0 {
		return false
	}

	return ipsOverlap(seedIPs, candidateIPs)
}

func (s *siteChecker) hostIPs(host string) []net.IP {
	host = normalizeHost(host)
	if host == "" {
		return nil
	}

	s.mu.Lock()
	if cached, ok := s.cache[host]; ok {
		s.mu.Unlock()
		return cached
	}
	s.mu.Unlock()

	var ips []net.IP
	if ip := net.ParseIP(host); ip != nil {
		ips = []net.IP{ip}
	} else {
		addrs, err := s.resolver.LookupIPAddr(context.Background(), host)
		if err != nil {
			return nil
		}
		ips = make([]net.IP, 0, len(addrs))
		for _, addr := range addrs {
			ips = append(ips, addr.IP)
		}
	}

	s.mu.Lock()
	s.cache[host] = ips
	s.mu.Unlock()

	return ips
}

func normalizeHost(host string) string {
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "" {
		return ""
	}
	if h, _, err := net.SplitHostPort(host); err == nil {
		return h
	}
	return host
}

func ipsOverlap(a, b []net.IP) bool {
	seen := make(map[string]struct{}, len(a))
	for _, ip := range a {
		seen[ip.String()] = struct{}{}
	}
	for _, ip := range b {
		if _, ok := seen[ip.String()]; ok {
			return true
		}
	}
	return false
}
