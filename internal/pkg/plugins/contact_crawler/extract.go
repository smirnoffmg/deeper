package contact_crawler

import (
	"net/url"
	"regexp"
	"strings"

	"github.com/smirnoffmg/deeper/internal/pkg/entities"
)

// Best-effort unanchored patterns for document scanning. Href-based mailto:/tel: extraction is preferred.
var (
	emailPattern = regexp.MustCompile(`\b[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}\b`)
	phonePattern = regexp.MustCompile(`(?:\+?\d{1,3}[-.\s]?)?\(?\d{3}\)?[-.\s]?\d{3}[-.\s]?\d{4}\b`)
)

type socialMatcher struct {
	hostSuffix string
	traceType  entities.TraceType
}

var socialMatchers = []socialMatcher{
	{hostSuffix: "twitter.com", traceType: entities.Twitter},
	{hostSuffix: "x.com", traceType: entities.Twitter},
	{hostSuffix: "github.com", traceType: entities.Github},
	{hostSuffix: "linkedin.com", traceType: entities.Linkedin},
	{hostSuffix: "instagram.com", traceType: entities.Instagram},
	{hostSuffix: "facebook.com", traceType: entities.Facebook},
	{hostSuffix: "tiktok.com", traceType: entities.TikTok},
	{hostSuffix: "reddit.com", traceType: entities.Reddit},
	{hostSuffix: "youtube.com", traceType: entities.YouTube},
	{hostSuffix: "pinterest.com", traceType: entities.Pinterest},
	{hostSuffix: "snapchat.com", traceType: entities.Snapchat},
	{hostSuffix: "tumblr.com", traceType: entities.Tumblr},
	{hostSuffix: "t.me", traceType: entities.SocialGeneric},
}

func extractEmails(text string) []string {
	return uniqueStrings(emailPattern.FindAllString(text, -1))
}

func extractPhones(text string) []string {
	candidates := phonePattern.FindAllString(text, -1)
	matched := make([]string, 0, len(candidates))
	for _, c := range candidates {
		if looksLikePhone(c) {
			matched = append(matched, c)
		}
	}
	return uniqueStrings(matched)
}

// looksLikePhone rejects a shape-matched candidate that's actually just a
// bare run of digits (a Telegram user ID, a tax ID, ...) — real phone
// numbers as written on contact pages are essentially always either
// "+"-prefixed or separated with spaces/dashes/dots/parens; a candidate
// with neither is far more likely to be an unrelated numeric ID that
// happens to be 10-13 digits long than an actual phone number.
func looksLikePhone(value string) bool {
	if strings.HasPrefix(value, "+") {
		return true
	}
	return strings.ContainsAny(value, " -.()")
}

func mailtoTrace(href string) (entities.Trace, bool) {
	addr, stripped := stripPrefixLoop(href, "mailto:")
	if !stripped {
		return entities.Trace{}, false
	}

	if idx := strings.IndexAny(addr, "?&"); idx >= 0 {
		addr = addr[:idx]
	}
	addr = strings.TrimSpace(addr)
	if !entities.IsEmail(addr) {
		return entities.Trace{}, false
	}

	return entities.Trace{Value: addr, Type: entities.Email}, true
}

func telTrace(href string) (entities.Trace, bool) {
	number, stripped := stripPrefixLoop(href, "tel:")
	if !stripped {
		return entities.Trace{}, false
	}

	if idx := strings.IndexAny(number, "?&"); idx >= 0 {
		number = number[:idx]
	}
	number = strings.TrimSpace(number)
	if !looksLikeTelNumber(number) {
		return entities.Trace{}, false
	}

	return entities.Trace{Value: number, Type: entities.Phone}, true
}

// stripPrefixLoop case-insensitively strips every leading occurrence of
// prefix, handling both mixed-case and doubled prefixes (e.g. a webmaster
// typo like "mailto:mailto:x@y.com" or "tel:tel:+1..."). Reports whether at
// least one prefix was actually stripped.
func stripPrefixLoop(value, prefix string) (string, bool) {
	stripped := false
	for strings.HasPrefix(strings.ToLower(value), prefix) {
		value = value[len(prefix):]
		stripped = true
	}
	return value, stripped
}

// looksLikeTelNumber validates raw `tel:` href content, which arrives
// unshaped (unlike extractPhones' free-text candidates, which are already
// pre-filtered by phonePattern before looksLikePhone ever sees them). Only
// digits and common phone punctuation are allowed, with a minimum digit
// count -- loose on purpose so real international formats (e.g. Russian
// "+7 495 123-45-67") aren't rejected the way entities.isPhone's strict
// NANP 3-3-4 grouping would.
func looksLikeTelNumber(value string) bool {
	digits := 0
	for _, r := range value {
		switch {
		case r >= '0' && r <= '9':
			digits++
		case r == '+' || r == '-' || r == '.' || r == '(' || r == ')' || r == ' ':
			// allowed separator
		default:
			return false
		}
	}
	return digits >= 7
}

// nonProfilePathSegments are share/tracking-widget paths that appear on
// essentially any modern website's footer/contact page. A hostname match
// alone can't distinguish these from the site's actual social profile link
// -- e.g. "facebook.com/sharer/sharer.php?u=..." is not the site owner's
// profile, just a "share this page" button.
var nonProfilePathSegments = map[string]bool{
	"intent":  true,
	"sharer":  true,
	"share":   true,
	"sharing": true,
	"dialog":  true,
	"plugins": true,
}

func socialLinkTrace(href string) (entities.Trace, bool) {
	parsed, err := url.Parse(href)
	if err != nil || parsed.Host == "" {
		return entities.Trace{}, false
	}

	// Only reject a known share/widget segment -- an empty path is not
	// itself disqualifying, since some matchers (e.g. tumblr) identify the
	// profile via subdomain rather than path.
	if firstSegment := strings.SplitN(strings.Trim(parsed.Path, "/"), "/", 2)[0]; nonProfilePathSegments[strings.ToLower(firstSegment)] {
		return entities.Trace{}, false
	}

	host := strings.ToLower(parsed.Hostname())
	for _, matcher := range socialMatchers {
		if host == matcher.hostSuffix || strings.HasSuffix(host, "."+matcher.hostSuffix) {
			normalized := normalizeSocialURL(parsed)
			return entities.Trace{Value: normalized, Type: matcher.traceType}, true
		}
	}

	return entities.Trace{}, false
}

func normalizeSocialURL(parsed *url.URL) string {
	parsed.Fragment = ""
	parsed.RawQuery = ""
	normalized := parsed.String()
	return strings.TrimSuffix(normalized, "/")
}

func uniqueStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}
