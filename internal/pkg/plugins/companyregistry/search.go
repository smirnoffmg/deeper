package companyregistry

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/smirnoffmg/deeper/internal/pkg/entities"
)

type searchFetcher interface {
	Get(ctx context.Context, url string) (*http.Response, error)
}

func searchCompany(ctx context.Context, fetcher searchFetcher, query string) ([]entities.Trace, error) {
	requestURL := "https://www.list-org.com/search?val=" + url.QueryEscape(query)

	resp, err := fetcher.Get(ctx, requestURL)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("list-org.com returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return parseSearchResult(string(body)), nil
}

func parseSearchResult(html string) []entities.Trace {
	if !strings.Contains(html, "<a href='/company/") {
		return nil
	}

	var traces []entities.Trace

	if fullName, ok := extractBetween(html, "</a><br><span>", "<br>"); ok {
		traces = append(traces, entities.Trace{Value: fullName, Type: entities.Company})
	}

	if director, ok := extractBetween(html, "<i>руководитель</i>: ", "<br>"); ok {
		traces = append(traces, entities.Trace{Value: director, Type: entities.Name})
	}

	if address, ok := extractBetween(html, "<i>юр.адрес</i>: ", "</span>"); ok {
		traces = append(traces, entities.Trace{Value: address, Type: entities.Address})
	}

	return traces
}

func extractBetween(html, startMarker, endMarker string) (string, bool) {
	start := strings.Index(html, startMarker)
	if start == -1 {
		return "", false
	}
	start += len(startMarker)

	end := strings.Index(html[start:], endMarker)
	if end == -1 {
		return "", false
	}

	return strings.TrimSpace(html[start : start+end]), true
}
