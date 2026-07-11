package academicpapers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/texttheater/golang-levenshtein/levenshtein"
)

type searchFetcher interface {
	Get(ctx context.Context, url string) (*http.Response, error)
}

type searchResult struct {
	Data []struct {
		URL     string `json:"url"`
		Authors []struct {
			Name string `json:"name"`
		} `json:"authors"`
	} `json:"data"`
}

func searchAuthorPapers(ctx context.Context, fetcher searchFetcher, name string) ([]string, error) {
	requestURL := "https://api.semanticscholar.org/graph/v1/author/search?query=" + url.QueryEscape(name)

	resp, err := fetcher.Get(ctx, requestURL)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("semantic scholar returned status %d", resp.StatusCode)
	}

	var result searchResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	var urls []string
	for _, paper := range result.Data {
		for _, author := range paper.Authors {
			if isCloseNameMatch(name, author.Name) {
				urls = append(urls, paper.URL)
			}
		}
	}
	return urls, nil
}

func isCloseNameMatch(queried, candidate string) bool {
	distance := levenshtein.DistanceForStrings([]rune(queried), []rune(candidate), levenshtein.DefaultOptions)
	return distance <= len(queried)/2
}
