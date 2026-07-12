package launchpad_profile

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/smirnoffmg/deeper/internal/pkg/entities"
)

type pageFetcher interface {
	Get(ctx context.Context, url string) (*http.Response, error)
}

// extractHandle returns the handle from a launchpad.net/~{handle} URL, or
// "" if it doesn't match.
func extractHandle(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Hostname() != "launchpad.net" {
		return ""
	}
	path := strings.Trim(parsed.Path, "/")
	if !strings.HasPrefix(path, "~") {
		return ""
	}
	handle := strings.TrimPrefix(path, "~")
	if handle == "" || strings.Contains(handle, "/") {
		return ""
	}
	return handle
}

const ogTitleSuffix = " in Launchpad"

func fetchProfile(ctx context.Context, fetcher pageFetcher, handle string) ([]entities.Trace, error) {
	reqURL := "https://launchpad.net/~" + handle

	resp, err := fetcher.Get(ctx, reqURL)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("launchpad profile page request failed: status %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}

	ogTitle, _ := doc.Find(`meta[property="og:title"]`).Attr("content")
	if !strings.HasSuffix(ogTitle, ogTitleSuffix) {
		return nil, nil
	}
	fullName := strings.TrimSpace(strings.TrimSuffix(ogTitle, ogTitleSuffix))

	// A display name that's just the handle isn't a real find.
	if fullName == "" || strings.EqualFold(fullName, handle) {
		return nil, nil
	}

	return []entities.Trace{{Type: entities.Name, Value: fullName}}, nil
}
