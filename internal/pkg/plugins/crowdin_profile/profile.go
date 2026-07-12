package crowdin_profile

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/smirnoffmg/deeper/internal/pkg/entities"
)

type pageFetcher interface {
	Get(ctx context.Context, url string) (*http.Response, error)
}

// extractHandle returns the handle from a crowdin.com/profile/{handle} URL,
// or "" if it doesn't match.
func extractHandle(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Hostname() != "crowdin.com" {
		return ""
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) != 2 || parts[0] != "profile" || parts[1] == "" {
		return ""
	}
	return parts[1]
}

// titlePattern matches Crowdin's profile page title, e.g.
// "Alexey Smirnov (alsmirn) – Crowdin", capturing the display name and the
// crowdin handle shown alongside it.
var titlePattern = regexp.MustCompile(`^(.+?)\s*\(([^)]+)\)\s*[–-]\s*Crowdin$`)

func fetchProfile(ctx context.Context, fetcher pageFetcher, handle string) ([]entities.Trace, error) {
	reqURL := "https://crowdin.com/profile/" + handle

	resp, err := fetcher.Get(ctx, reqURL)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("crowdin profile page request failed: status %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}

	title := strings.TrimSpace(doc.Find("title").First().Text())
	match := titlePattern.FindStringSubmatch(title)
	if match == nil {
		return nil, nil
	}
	fullName, titleHandle := match[1], match[2]

	// A display name that's just the username (case-insensitively, as seen
	// for accounts that never set a real display name) isn't a real find.
	if strings.EqualFold(fullName, titleHandle) || strings.EqualFold(fullName, handle) {
		return nil, nil
	}

	return []entities.Trace{{Type: entities.Name, Value: fullName}}, nil
}
