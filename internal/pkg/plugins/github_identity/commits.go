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
	"github.com/smirnoffmg/deeper/internal/pkg/plugins"
)

type commitFetcher interface {
	Do(req *http.Request) (*http.Response, error)
}

type commitAuthor struct {
	Name  string
	Email string
	Login string
	// CoAuthor marks an identity named only by a Co-authored-by trailer: the
	// trailer credits someone else, so it relates them to the repo without
	// showing they are the owner.
	CoAuthor bool
	Bot      bool
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
			Name:     strings.TrimSpace(m[1]),
			Email:    strings.TrimSpace(m[2]),
			CoAuthor: true,
		})
	}
	return authors
}

func parseCommitAuthors(body []byte) []commitAuthor {
	var raw []struct {
		Author *struct {
			Login string `json:"login"`
			Type  string `json:"type"`
		} `json:"author"`
		Committer *struct {
			Type string `json:"type"`
		} `json:"committer"`
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
			author.Bot = item.Author.Type == "Bot"
		}
		if author.Name != "" || author.Email != "" || author.Login != "" {
			authors = append(authors, author)
		}

		committer := commitAuthor{
			Name:  strings.TrimSpace(item.Commit.Committer.Name),
			Email: strings.TrimSpace(item.Commit.Committer.Email),
			Bot:   item.Committer != nil && item.Committer.Type == "Bot",
		}
		if committer.Name != "" || committer.Email != "" {
			authors = append(authors, committer)
		}

		authors = append(authors, parseCoAuthors(item.Commit.Message)...)
	}
	return dropNonPersons(authors)
}

var (
	botNoreplyEmail     = regexp.MustCompile(`(?i)\[bot\]@users\.noreply\.github\.com$`)
	copilotNoreplyEmail = regexp.MustCompile(`(?i)^\d+\+copilot@users\.noreply\.github\.com$`)
)

var automationEmails = map[string]bool{
	"noreply@github.com":        true,
	"action@github.com":         true,
	"github-actions@github.com": true,
}

// isNonPerson reports GitHub's own web-flow committer and app/bot accounts.
// Left in, their names and logins become Username/Name traces and get swept
// across every site Sherlock knows.
func isNonPerson(a commitAuthor) bool {
	if a.Bot || automationEmails[strings.ToLower(a.Email)] {
		return true
	}
	if botNoreplyEmail.MatchString(a.Email) || copilotNoreplyEmail.MatchString(a.Email) {
		return true
	}
	return strings.HasSuffix(strings.ToLower(a.Login), "[bot]") ||
		strings.HasSuffix(strings.ToLower(a.Name), "[bot]")
}

func dropNonPersons(authors []commitAuthor) []commitAuthor {
	var kept []commitAuthor
	for _, a := range authors {
		if !isNonPerson(a) {
			kept = append(kept, a)
		}
	}
	return kept
}

// maxDistinctContributors bounds how many distinct people a repo's commit
// history can credit before it's treated as a shared/workshop/community
// project rather than the target's own personal work.
const maxDistinctContributors = 5

// filterAuthorsForSharedRepo drops unrelated contributors when a repo has
// too many distinct people in its commit history — found live:
// iloncka/workshop-astra-tik-tok is not a fork (isFork correctly reports
// false) but has commits from several unrelated DataStax employees,
// leaking their identities purely because iloncka once contributed to a
// shared workshop repo. A small collaboration (e.g. two people on a
// personal project) is a real, meaningful signal and stays intact; a repo
// with many contributors is not, so only the repo owner's own
// login-matched commits are kept.
func filterAuthorsForSharedRepo(authors []commitAuthor, owner string) []commitAuthor {
	if distinctIdentityCount(authors) <= maxDistinctContributors {
		return authors
	}

	var filtered []commitAuthor
	for _, a := range authors {
		if a.Login != "" && strings.EqualFold(a.Login, owner) {
			filtered = append(filtered, a)
		}
	}
	return filtered
}

func distinctIdentityCount(authors []commitAuthor) int {
	seen := make(map[string]struct{})
	for _, a := range authors {
		key := identityKey(a)
		if key == "" {
			continue
		}
		seen[key] = struct{}{}
	}
	return len(seen)
}

func identityKey(a commitAuthor) string {
	switch {
	case a.Login != "":
		return "login:" + strings.ToLower(a.Login)
	case a.Email != "":
		return "email:" + strings.ToLower(a.Email)
	case a.Name != "":
		return "name:" + strings.ToLower(a.Name)
	default:
		return ""
	}
}

func authorsToFindings(authors []commitAuthor) []plugins.Finding {
	index := make(map[entities.Trace]int)
	var findings []plugins.Finding

	add := func(traceType entities.TraceType, value string, strength entities.Strength, basis string) {
		if value == "" {
			return
		}
		trace := entities.Trace{Type: traceType, Value: value}
		if i, ok := index[trace]; ok {
			if findings[i].Strength == entities.StrengthWeak && strength != entities.StrengthWeak {
				findings[i].Strength, findings[i].Basis = strength, basis
			}
			return
		}
		index[trace] = len(findings)
		findings = append(findings, plugins.Finding{Trace: trace, Strength: strength, Basis: basis})
	}

	for _, author := range authors {
		strength, basis := entities.StrengthStrong, "commit author"
		if author.CoAuthor {
			strength, basis = entities.StrengthWeak, "co-authored-by trailer"
		}
		add(entities.Name, author.Name, strength, basis)
		if entities.IsRealEmail(author.Email) {
			add(entities.Email, author.Email, strength, basis)
		}
		add(entities.Username, author.Login, strength, basis)
	}

	return findings
}
