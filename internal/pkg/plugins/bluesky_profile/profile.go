package bluesky_profile

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/smirnoffmg/deeper/internal/pkg/entities"
)

type profileFetcher interface {
	Get(ctx context.Context, url string) (*http.Response, error)
}

var (
	emailPattern = regexp.MustCompile(`\b[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}\b`)
	urlPattern   = regexp.MustCompile(`https?://\S+`)
)

// extractHandle returns the bluesky handle from a bsky.app profile URL as
// written there (lig.bsky.social, alice.example.com), or "" if it doesn't
// match.
func extractHandle(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Hostname() != "bsky.app" {
		return ""
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) != 2 || parts[0] != "profile" || parts[1] == "" {
		return ""
	}
	return parts[1]
}

type blueskyProfile struct {
	DisplayName string `json:"displayName"`
	Description string `json:"description"`
}

type blueskyError struct {
	Error string `json:"error"`
}

func fetchProfile(ctx context.Context, fetcher profileFetcher, handle string) ([]entities.Trace, error) {
	if !strings.HasPrefix(handle, "did:") && !strings.Contains(handle, ".") {
		handle += ".bsky.social"
	}
	reqURL := "https://public.api.bsky.app/xrpc/app.bsky.actor.getProfile?actor=" + handle

	resp, err := fetcher.Get(ctx, reqURL)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// A nonexistent handle returns HTTP 400 with
	// {"error":"InvalidRequest","message":"Profile not found"} -- verified
	// live, not an error worth surfacing.
	if resp.StatusCode == http.StatusBadRequest {
		var apiErr blueskyError
		if err := json.Unmarshal(body, &apiErr); err == nil && apiErr.Error == "InvalidRequest" {
			return nil, nil
		}
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("bluesky getProfile request failed: status %d", resp.StatusCode)
	}

	var profile blueskyProfile
	if err := json.Unmarshal(body, &profile); err != nil {
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

	addTrace(entities.Name, profile.DisplayName)

	// The bio is free text; only worth a trace if it embeds a concrete
	// contact point, not as an unstructured blob of noise.
	if email := emailPattern.FindString(profile.Description); email != "" {
		addTrace(entities.Email, email)
	}
	if link := urlPattern.FindString(profile.Description); link != "" {
		addTrace(entities.Url, link)
	}

	return traces, nil
}
