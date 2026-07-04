package contact_crawler

import (
	"strings"

	"golang.org/x/net/publicsuffix"
)

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

// sameSite reports whether candidateHost is in-scope for a crawl seeded at seedHost,
// based purely on sharing a registrable domain. IP overlap is deliberately not used:
// shared hosting/CDN infrastructure (e.g. Cloudflare) puts many unrelated tenants
// behind the same edge IP, which would let the crawler walk onto unauthorized domains.
func sameSite(seedHost, candidateHost string) bool {
	seedDomain, ok := registrableDomain(seedHost)
	if !ok {
		return false
	}

	candidateDomain, ok := registrableDomain(candidateHost)
	if !ok {
		return false
	}

	return seedDomain == candidateDomain
}
