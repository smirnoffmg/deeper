package github_profile

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

func fetchProfile(ctx context.Context, fetcher profileFetcher, username string) ([]entities.Trace, error) {
	reqURL := "https://api.github.com/users/" + username

	resp, err := fetcher.Get(ctx, reqURL)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("github user profile request failed: status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var raw struct {
		Name            string `json:"name"`
		Company         string `json:"company"`
		Blog            string `json:"blog"`
		Location        string `json:"location"`
		Email           string `json:"email"`
		TwitterUsername string `json:"twitter_username"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
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

	addTrace(entities.Name, raw.Name)
	addTrace(entities.Company, raw.Company)
	addTrace(entities.Address, raw.Location)
	addTrace(entities.Email, raw.Email)
	addTrace(entities.Twitter, raw.TwitterUsername)

	if blog := strings.TrimSpace(raw.Blog); blog != "" {
		if parsed, err := url.Parse(blog); err == nil && parsed.Scheme != "" && parsed.Host != "" {
			addTrace(entities.Url, blog)
		}
	}

	return traces, nil
}
