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
	if !strings.HasPrefix(strings.ToLower(href), "mailto:") {
		return entities.Trace{}, false
	}

	addr := strings.TrimPrefix(href, "mailto:")
	addr = strings.TrimPrefix(addr, "MAILTO:")
	if idx := strings.IndexAny(addr, "?&"); idx >= 0 {
		addr = addr[:idx]
	}
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return entities.Trace{}, false
	}

	return entities.Trace{Value: addr, Type: entities.Email}, true
}

func telTrace(href string) (entities.Trace, bool) {
	lower := strings.ToLower(href)
	if !strings.HasPrefix(lower, "tel:") {
		return entities.Trace{}, false
	}

	number := strings.TrimSpace(href[4:])
	if idx := strings.IndexAny(number, "?&"); idx >= 0 {
		number = number[:idx]
	}
	if number == "" {
		return entities.Trace{}, false
	}

	return entities.Trace{Value: number, Type: entities.Phone}, true
}

func socialLinkTrace(href string) (entities.Trace, bool) {
	parsed, err := url.Parse(href)
	if err != nil || parsed.Host == "" {
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
