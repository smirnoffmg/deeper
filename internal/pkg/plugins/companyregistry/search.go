package companyregistry

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/rs/zerolog/log"
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

	traces := parseSearchResult(string(body))
	if traces == nil {
		return nil, nil
	}

	if id, ok := extractCompanyID(string(body)); ok {
		traces = append(traces, fetchNearbyCompanies(ctx, fetcher, id)...)
	}

	return traces, nil
}

// fetchNearbyCompanies follows the search match's own detail page and pulls
// its small, GPS-proximity-based "nearby enterprises" list — deliberately
// not a broader address/district search (tested live: that returns
// hundreds of unrelated results). Best-effort: a failure here must not
// lose the traces already found on the search page.
func fetchNearbyCompanies(ctx context.Context, fetcher searchFetcher, companyID string) []entities.Trace {
	detailURL := "https://www.list-org.com/company/" + companyID

	resp, err := fetcher.Get(ctx, detailURL)
	if err != nil {
		log.Warn().Err(err).Str("company_id", companyID).Msg("list-org.com detail page fetch failed, skipping nearby companies")
		return nil
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Warn().Int("status", resp.StatusCode).Str("company_id", companyID).Msg("list-org.com detail page returned non-200, skipping nearby companies")
		return nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil
	}

	var traces []entities.Trace
	for _, name := range parseNearbyCompanies(string(body)) {
		traces = append(traces, entities.Trace{Value: name, Type: entities.Company})
	}
	return traces
}

var companyIDPattern = regexp.MustCompile(`<a href='/company/(\d+)'>`)

func extractCompanyID(html string) (string, bool) {
	match := companyIDPattern.FindStringSubmatch(html)
	if match == nil {
		return "", false
	}
	return match[1], true
}

func parseNearbyCompanies(html string) []string {
	section, ok := extractBetween(html, "Предприятия рядом:</i> <span class='upper'>", "</span>")
	if !ok {
		return nil
	}

	var names []string
	for _, match := range companyLinkPattern.FindAllStringSubmatch(section, -1) {
		names = append(names, match[1])
	}
	return names
}

var companyLinkPattern = regexp.MustCompile(`<a href='/company/\d+'>([^<]+)</a>`)

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
