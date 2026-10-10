package coderepos

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/rs/zerolog/log"
	"github.com/smirnoffmg/deeper/internal/pkg/config"
	"github.com/smirnoffmg/deeper/internal/pkg/entities"
	deeperhttp "github.com/smirnoffmg/deeper/internal/pkg/http"
	"github.com/smirnoffmg/deeper/internal/pkg/plugins"
)

const InputTraceType = entities.Username

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
	r.Add(InputTraceType, g)
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
	if trace.Type != InputTraceType {
		return nil, nil
	}

	var newTraces []entities.Trace
	var errs []error

	githubRepos, err := fetchGitHubRepos(ctx, g.fetcher, trace.Value, g.githubToken)
	if err == nil {
		newTraces = append(newTraces, githubRepos...)
	} else {
		errs = append(errs, err)
	}

	bitbucketRepos, err := fetchBitbucketRepos(ctx, g.fetcher, trace.Value)
	if err == nil {
		newTraces = append(newTraces, bitbucketRepos...)
	} else {
		errs = append(errs, err)
	}

	gitlabRepos, err := fetchGitLabRepos(ctx, g.fetcher, trace.Value)
	if err == nil {
		newTraces = append(newTraces, gitlabRepos...)
	} else {
		errs = append(errs, err)
	}

	// The processor drops every trace when a plugin errors, so one host
	// rate-limiting must not discard repos the other hosts did return.
	if len(newTraces) > 0 {
		for _, err := range errs {
			log.Warn().Err(err).Str("username", trace.Value).Msg("code repository source failed, skipping")
		}
		return newTraces, nil
	}
	return nil, errors.Join(errs...)
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
