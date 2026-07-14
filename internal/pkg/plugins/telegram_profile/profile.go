package telegram_profile

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

var (
	emailPattern = regexp.MustCompile(`\b[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}\b`)
	// Deliberately excludes common wrapping/trailing punctuation from the
	// match itself (rather than post-trimming), so a URL parenthesized or
	// followed by a period/comma in free text -- e.g. "(https://t.me/x)."
	// -- doesn't pull that punctuation into the captured value. Tradeoff:
	// a URL whose own query string contains one of these chars gets
	// truncated too; accepted, since free-text descriptions rarely embed
	// such URLs and the false-positive risk is worse than the truncation risk.
	urlPattern = regexp.MustCompile(`https?://[^\s"')\]}>,;:!?]+`)
)

// extractChannel returns the channel/username from a t.me profile URL, or
// "" if it doesn't match.
func extractChannel(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Hostname() != "t.me" {
		return ""
	}
	channel := strings.Trim(parsed.Path, "/")
	if channel == "" || strings.Contains(channel, "/") {
		return ""
	}
	return channel
}

func fetchProfile(ctx context.Context, fetcher pageFetcher, channel string) ([]entities.Trace, error) {
	reqURL := "https://t.me/" + channel

	resp, err := fetcher.Get(ctx, reqURL)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("telegram preview page request failed: status %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}

	// t.me always returns 200 even for nonexistent channels, rendering a
	// generic "Contact @handle" shell with an empty og:description --
	// verified live. So an empty description naturally yields no traces
	// without needing separate not-found handling.
	description, _ := doc.Find(`meta[property="og:description"]`).Attr("content")

	var traces []entities.Trace
	addTrace := func(traceType entities.TraceType, value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		traces = append(traces, entities.Trace{Type: traceType, Value: value})
	}

	if email := emailPattern.FindString(description); email != "" {
		addTrace(entities.Email, email)
	}
	if link := urlPattern.FindString(description); link != "" {
		addTrace(entities.Url, link)
	}

	return traces, nil
}
