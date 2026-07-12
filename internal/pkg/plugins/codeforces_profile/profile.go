package codeforces_profile

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/smirnoffmg/deeper/internal/pkg/entities"
)

type profileFetcher interface {
	Get(ctx context.Context, url string) (*http.Response, error)
}

// extractHandle returns the codeforces handle from a codeforces.com profile
// URL (https://codeforces.com/profile/{handle}), or "" if it doesn't match.
func extractHandle(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Hostname() != "codeforces.com" {
		return ""
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) != 2 || parts[0] != "profile" || parts[1] == "" {
		return ""
	}
	return parts[1]
}

type codeforcesResponse struct {
	Status string `json:"status"`
	Result []struct {
		FirstName    string `json:"firstName"`
		LastName     string `json:"lastName"`
		Organization string `json:"organization"`
		City         string `json:"city"`
		Country      string `json:"country"`
	} `json:"result"`
}

func fetchProfile(ctx context.Context, fetcher profileFetcher, handle string) ([]entities.Trace, error) {
	reqURL := "https://codeforces.com/api/user.info?handles=" + handle

	resp, err := fetcher.Get(ctx, reqURL)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("codeforces user.info request failed: status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var raw codeforcesResponse
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}

	// A nonexistent handle returns {"status":"FAILED",...}, verified live
	// against codeforces.com/api/user.info -- not an error, just no user.
	if raw.Status != "OK" || len(raw.Result) == 0 {
		return nil, nil
	}
	user := raw.Result[0]

	var traces []entities.Trace
	addTrace := func(traceType entities.TraceType, value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		traces = append(traces, entities.Trace{Type: traceType, Value: value})
	}

	name := strings.TrimSpace(strings.TrimSpace(user.FirstName) + " " + strings.TrimSpace(user.LastName))
	addTrace(entities.Name, name)
	addTrace(entities.Company, user.Organization)

	location := user.City
	if user.Country != "" {
		if location != "" {
			location += ", "
		}
		location += user.Country
	}
	addTrace(entities.Address, location)

	return traces, nil
}
