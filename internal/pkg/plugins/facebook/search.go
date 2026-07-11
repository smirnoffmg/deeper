package facebook

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type searchFetcher interface {
	Get(ctx context.Context, url string) (*http.Response, error)
}

func searchFacebookProfiles(ctx context.Context, fetcher searchFetcher, query string) ([]string, error) {
	searchQuery := query + " site:facebook.com"
	requestURL := "https://www.google.com/search?q=" + url.QueryEscape(searchQuery)

	resp, err := fetcher.Get(ctx, requestURL)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("google search returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return parseGoogleResults(string(body)), nil
}

func parseGoogleResults(body string) []string {
	var profiles []string
	for _, line := range strings.Split(body, "\n") {
		if !strings.Contains(line, "https://www.facebook.com/") {
			continue
		}
		start := strings.Index(line, "https://www.facebook.com/")
		end := strings.Index(line[start:], "\"")
		if end == -1 {
			continue
		}
		profiles = append(profiles, line[start:start+end])
	}
	return profiles
}
