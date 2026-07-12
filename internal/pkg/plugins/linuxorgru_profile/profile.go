package linuxorgru_profile

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/smirnoffmg/deeper/internal/pkg/entities"
)

type pageFetcher interface {
	Get(ctx context.Context, url string) (*http.Response, error)
}

// extractHandle returns the handle from a linux.org.ru profile URL
// (either host variant, /people/{handle}/profile), or "" if it doesn't match.
func extractHandle(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	host := parsed.Hostname()
	if host != "linux.org.ru" && host != "www.linux.org.ru" {
		return ""
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) != 3 || parts[0] != "people" || parts[2] != "profile" || parts[1] == "" {
		return ""
	}
	return parts[1]
}

var (
	nameFieldPattern = regexp.MustCompile(`<span class="fn">([^<]+)</span>`)
	cityFieldPattern = regexp.MustCompile(`<b>Город:</b>\s*([^<]+)<br>`)
)

func fetchProfile(ctx context.Context, fetcher pageFetcher, handle string) ([]entities.Trace, error) {
	reqURL := "https://www.linux.org.ru/people/" + handle + "/profile"

	resp, err := fetcher.Get(ctx, reqURL)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("linux.org.ru profile page request failed: status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var traces []entities.Trace
	addTrace := func(traceType entities.TraceType, value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		traces = append(traces, entities.Trace{Type: traceType, Value: value})
	}

	if match := nameFieldPattern.FindSubmatch(body); match != nil {
		fullName := strings.TrimSpace(string(match[1]))
		// A display name that's just the handle isn't a real find.
		if !strings.EqualFold(fullName, handle) {
			addTrace(entities.Name, fullName)
		}
	}

	if match := cityFieldPattern.FindSubmatch(body); match != nil {
		addTrace(entities.Address, string(match[1]))
	}

	return traces, nil
}
