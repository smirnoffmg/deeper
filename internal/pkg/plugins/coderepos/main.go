package coderepos

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/smirnoffmg/deeper/internal/pkg/config"
	"github.com/smirnoffmg/deeper/internal/pkg/entities"
	deeperhttp "github.com/smirnoffmg/deeper/internal/pkg/http"
	"github.com/smirnoffmg/deeper/internal/pkg/plugins"
)

type repoFetcher interface {
	Do(req *http.Request) (*http.Response, error)
}

type CodeRepositoriesPlugin struct {
	fetcher     repoFetcher
	githubToken string
}

func NewPlugin(cfg *config.Config) *CodeRepositoriesPlugin {
	return &CodeRepositoriesPlugin{fetcher: deeperhttp.NewClient(cfg), githubToken: cfg.GitHubToken}
}

func (g *CodeRepositoriesPlugin) Register(r plugins.Registry) error {
	r.Add(entities.Github, g)
	r.Add(entities.Username, g)
	return nil
}

type GitHubRepo struct {
	URL string `json:"html_url"`
}

type BitbucketRepo struct {
	Links struct {
		HTML struct {
			Href string `json:"href"`
		} `json:"html"`
	} `json:"links"`
}

type GitLabRepo struct {
	WebURL string `json:"web_url"`
}

func (g *CodeRepositoriesPlugin) FollowTrace(ctx context.Context, trace entities.Trace) ([]entities.Trace, error) {
	findings, err := g.FollowEvidence(ctx, trace)
	if err != nil {
		return nil, err
	}
	var traces []entities.Trace
	for _, f := range findings {
		traces = append(traces, f.Trace)
	}
	return traces, nil
}

// FollowEvidence lists GitHub repos only for a Github account, where the
// account is already established. A bare handle reaches GitLab and Bitbucket
// by name alone, so those repos are a namesake guess.
func (g *CodeRepositoriesPlugin) FollowEvidence(ctx context.Context, trace entities.Trace) ([]plugins.Finding, error) {
	switch trace.Type {
	case entities.Github:
		login, ok := githubLogin(trace.Value)
		if !ok {
			return nil, nil
		}
		repos, err := fetchGitHubRepos(ctx, g.fetcher, login, g.githubToken)
		if err != nil {
			return nil, err
		}
		return withEvidence(repos, entities.StrengthStrong, "repos of github.com/"+login), nil
	case entities.Username:
		return g.followNamesake(ctx, trace.Value)
	}
	return nil, nil
}

func (g *CodeRepositoriesPlugin) followNamesake(ctx context.Context, username string) ([]plugins.Finding, error) {
	var findings []plugins.Finding
	var errs []error

	bitbucketRepos, err := fetchBitbucketRepos(ctx, g.fetcher, username)
	if err == nil {
		findings = append(findings, withEvidence(bitbucketRepos, entities.StrengthWeak, "namesake: bitbucket.org/"+username+" has repos")...)
	} else {
		errs = append(errs, err)
	}

	gitlabRepos, err := fetchGitLabRepos(ctx, g.fetcher, username)
	if err == nil {
		findings = append(findings, withEvidence(gitlabRepos, entities.StrengthWeak, "namesake: gitlab.com/"+username+" has repos")...)
	} else {
		errs = append(errs, err)
	}

	// The processor drops every trace when a plugin errors, so one host
	// rate-limiting must not discard repos the other host did return.
	if len(findings) > 0 {
		for _, err := range errs {
			log.Warn().Err(err).Str("username", username).Msg("code repository source failed, skipping")
		}
		return findings, nil
	}
	return nil, errors.Join(errs...)
}

func withEvidence(traces []entities.Trace, strength entities.Strength, basis string) []plugins.Finding {
	findings := make([]plugins.Finding, 0, len(traces))
	for _, t := range traces {
		findings = append(findings, plugins.Finding{Trace: t, Strength: strength, Basis: basis})
	}
	return findings
}

// githubLogin returns the login of a github.com account URL. Repo URLs share
// the Github trace type and are left to github_identity.
func githubLogin(value string) (string, bool) {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil {
		return "", false
	}
	host := strings.ToLower(parsed.Hostname())
	if host != "github.com" && host != "www.github.com" {
		return "", false
	}
	login := strings.Trim(parsed.Path, "/")
	if login == "" || strings.Contains(login, "/") {
		return "", false
	}
	return login, true
}

func getRepoList(ctx context.Context, fetcher repoFetcher, host, url, token string, into any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := fetcher.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("%s repositories request failed: status %d", host, resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(into)
}

func fetchGitHubRepos(ctx context.Context, fetcher repoFetcher, username, token string) ([]entities.Trace, error) {
	url := fmt.Sprintf("https://api.github.com/users/%s/repos?per_page=100", username)
	var repos []GitHubRepo
	if err := getRepoList(ctx, fetcher, "github", url, token, &repos); err != nil {
		return nil, err
	}

	var traces []entities.Trace
	for _, repo := range repos {
		traces = append(traces, entities.Trace{
			Value: repo.URL,
			Type:  entities.Repository,
		})
	}
	return traces, nil
}

func fetchBitbucketRepos(ctx context.Context, fetcher repoFetcher, username string) ([]entities.Trace, error) {
	url := fmt.Sprintf("https://api.bitbucket.org/2.0/repositories/%s?pagelen=100", username)
	var result struct {
		Repos []BitbucketRepo `json:"values"`
	}
	if err := getRepoList(ctx, fetcher, "bitbucket", url, "", &result); err != nil {
		return nil, err
	}

	var traces []entities.Trace
	for _, repo := range result.Repos {
		traces = append(traces, entities.Trace{
			Value: repo.Links.HTML.Href,
			Type:  entities.Repository,
		})
	}
	return traces, nil
}

func fetchGitLabRepos(ctx context.Context, fetcher repoFetcher, username string) ([]entities.Trace, error) {
	url := fmt.Sprintf("https://gitlab.com/api/v4/users/%s/projects?per_page=100", username)
	var repos []GitLabRepo
	if err := getRepoList(ctx, fetcher, "gitlab", url, "", &repos); err != nil {
		return nil, err
	}

	var traces []entities.Trace
	for _, repo := range repos {
		traces = append(traces, entities.Trace{
			Value: repo.WebURL,
			Type:  entities.Repository,
		})
	}
	return traces, nil
}

func (g CodeRepositoriesPlugin) String() string {
	return "CodeRepositoriesPlugin"
}
