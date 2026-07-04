package github_identity

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

type commitFetcher interface {
	Do(req *http.Request) (*http.Response, error)
}

type commitAuthor struct {
	Name  string
	Email string
	Login string
}

func parseOwnerRepo(githubURL string) (owner, repo string, ok bool) {
	parsed, err := url.Parse(strings.TrimSpace(githubURL))
	if err != nil || parsed.Host == "" {
		return "", "", false
	}

	host := strings.ToLower(parsed.Hostname())
	if host != "github.com" && host != "www.github.com" {
		return "", "", false
	}

	segments := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(segments) != 2 || segments[0] == "" || segments[1] == "" {
		return "", "", false
	}

	return segments[0], segments[1], true
}

func fetchCommitAuthors(ctx context.Context, fetcher commitFetcher, owner, repo, token string) ([]commitAuthor, error) {
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/commits?per_page=100", owner, repo)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := fetcher.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if isRateLimited(resp) {
		return nil, fmt.Errorf("github api rate limit exceeded for %s/%s", owner, repo)
	}
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("github commits request failed: status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return parseCommitAuthors(body), nil
}

func isRateLimited(resp *http.Response) bool {
	if resp.StatusCode != http.StatusForbidden && resp.StatusCode != http.StatusTooManyRequests {
		return false
	}
	return resp.Header.Get("X-RateLimit-Remaining") == "0"
}

func parseCommitAuthors(body []byte) []commitAuthor {
	var raw []struct {
		Author *struct {
			Login string `json:"login"`
		} `json:"author"`
		Commit struct {
			Author struct {
				Name  string `json:"name"`
				Email string `json:"email"`
			} `json:"author"`
		} `json:"commit"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil
	}

	authors := make([]commitAuthor, 0, len(raw))
	for _, item := range raw {
		author := commitAuthor{
			Name:  strings.TrimSpace(item.Commit.Author.Name),
			Email: strings.TrimSpace(item.Commit.Author.Email),
		}
		if item.Author != nil {
			author.Login = strings.TrimSpace(item.Author.Login)
		}
		if author.Name == "" && author.Email == "" && author.Login == "" {
			continue
		}
		authors = append(authors, author)
	}
	return authors
}

func authorsToTraces(authors []commitAuthor) []entities.Trace {
	seen := make(map[string]struct{})
	var traces []entities.Trace

	addTrace := func(traceType entities.TraceType, value string) {
		if value == "" {
			return
		}
		key := string(traceType) + "\x00" + value
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		traces = append(traces, entities.Trace{Type: traceType, Value: value})
	}

	for _, author := range authors {
		addTrace(entities.Name, author.Name)
		addTrace(entities.Email, author.Email)
		addTrace(entities.Username, author.Login)
	}

	return traces
}
