package github_identity

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

// isFork checks whether a repo is a fork before mining its commit history,
// to avoid pulling in an unrelated upstream project's entire contributor
// history (found live: alsmirn/youtube-dl is a fork of the well-known
// yt-dlp/youtube-dl project, not his own work). Best-effort: any failure to
// determine fork status (network error, non-200, malformed JSON) is treated
// as "not a fork" rather than blocking commit mining — this is a safety
// filter on top of existing behavior, not a hard requirement, and losing a
// legitimate result to an unrelated metadata-fetch hiccup would be worse
// than the rare case of missing the filter.
func isFork(ctx context.Context, fetcher commitFetcher, owner, repo, token string) bool {
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s", owner, repo)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return false
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := fetcher.Do(req)
	if err != nil {
		return false
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return false
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false
	}

	var meta struct {
		Fork bool `json:"fork"`
	}
	if err := json.Unmarshal(body, &meta); err != nil {
		return false
	}

	return meta.Fork
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

var coAuthorTrailer = regexp.MustCompile(`(?im)^co-authored-by:\s*(.+?)\s*<([^>]+)>\s*$`)

// parseCoAuthors extracts Co-authored-by trailers from a commit message —
// a real Git/GitHub convention for crediting pair-programming collaborators
// who don't appear as the commit's author or committer at all.
func parseCoAuthors(message string) []commitAuthor {
	matches := coAuthorTrailer.FindAllStringSubmatch(message, -1)
	if len(matches) == 0 {
		return nil
	}

	authors := make([]commitAuthor, 0, len(matches))
	for _, m := range matches {
		authors = append(authors, commitAuthor{
			Name:  strings.TrimSpace(m[1]),
			Email: strings.TrimSpace(m[2]),
		})
	}
	return authors
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
			Committer struct {
				Name  string `json:"name"`
				Email string `json:"email"`
			} `json:"committer"`
			Message string `json:"message"`
		} `json:"commit"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil
	}

	var authors []commitAuthor
	for _, item := range raw {
		author := commitAuthor{
			Name:  strings.TrimSpace(item.Commit.Author.Name),
			Email: strings.TrimSpace(item.Commit.Author.Email),
		}
		if item.Author != nil {
			author.Login = strings.TrimSpace(item.Author.Login)
		}
		if author.Name != "" || author.Email != "" || author.Login != "" {
			authors = append(authors, author)
		}

		committer := commitAuthor{
			Name:  strings.TrimSpace(item.Commit.Committer.Name),
			Email: strings.TrimSpace(item.Commit.Committer.Email),
		}
		if committer.Name != "" || committer.Email != "" {
			authors = append(authors, committer)
		}

		authors = append(authors, parseCoAuthors(item.Commit.Message)...)
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
