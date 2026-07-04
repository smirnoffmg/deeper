package contact_crawler

import (
	"net/url"
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

// ownerMatchesTarget reports whether a GitHub repo owner plausibly belongs to
// the site being crawled, comparing against the registrable domain's own
// label (e.g. "codescoring.ru" -> "codescoring"). This exists specifically to
// stop github_identity from mining commit history (real names, personal
// emails) of unrelated third-party projects that a target's site merely
// links to or mentions — confirmed live that a "codescoring.ru" crawl
// otherwise pulled in an unrelated open-source project's contributors
// (pgbouncer) and fanned that out across dozens of social platforms.
func ownerMatchesTarget(owner, seedHost string) bool {
	owner = strings.ToLower(strings.TrimSpace(owner))
	if owner == "" {
		return false
	}

	domain, ok := registrableDomain(seedHost)
	if !ok {
		return false
	}

	label := strings.ToLower(strings.SplitN(domain, ".", 2)[0])
	if label == "" {
		return false
	}

	return label == owner || strings.Contains(owner, label) || strings.Contains(label, owner)
}

// githubRepoOwner extracts the owner/org segment from a github.com URL
// (repo URL or org-root URL) without pulling in the github_identity plugin
// as a dependency — plugins are kept independent of each other.
func githubRepoOwner(href string) (string, bool) {
	parsed, err := url.Parse(strings.TrimSpace(href))
	if err != nil || parsed.Host == "" {
		return "", false
	}

	segments := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(segments) == 0 || segments[0] == "" {
		return "", false
	}

	return segments[0], true
}
