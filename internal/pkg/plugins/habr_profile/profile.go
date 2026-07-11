package habr_profile

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/smirnoffmg/deeper/internal/pkg/entities"
)

type profileFetcher interface {
	Get(ctx context.Context, url string) (*http.Response, error)
}

func fetchProfile(ctx context.Context, fetcher profileFetcher, username string) ([]entities.Trace, error) {
	reqURL := "https://habr.com/kek/v2/users/" + username + "/card"

	resp, err := fetcher.Get(ctx, reqURL)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("habr card request failed: status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var raw struct {
		Fullname  string  `json:"fullname"`
		Location  *string `json:"location"`
		Birthday  *string `json:"birthday"`
		Workplace []struct {
			Title string `json:"title"`
		} `json:"workplace"`
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

	addTrace(entities.Name, raw.Fullname)
	if raw.Location != nil {
		addTrace(entities.Address, *raw.Location)
	}
	if raw.Birthday != nil {
		addTrace(entities.DateOfBirth, *raw.Birthday)
	}
	for _, w := range raw.Workplace {
		addTrace(entities.Company, w.Title)
	}

	return traces, nil
}
