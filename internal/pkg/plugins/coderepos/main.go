package coderepos

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/smirnoffmg/deeper/internal/pkg/config"
	"github.com/smirnoffmg/deeper/internal/pkg/entities"
	deeperhttp "github.com/smirnoffmg/deeper/internal/pkg/http"
	"github.com/smirnoffmg/deeper/internal/pkg/plugins"
)

const InputTraceType = entities.Username

type repoFetcher interface {
	Get(ctx context.Context, url string) (*http.Response, error)
}

type CodeRepositoriesPlugin struct {
	fetcher repoFetcher
}

func NewPlugin(cfg *config.Config) *CodeRepositoriesPlugin {
	return &CodeRepositoriesPlugin{fetcher: deeperhttp.NewClient(cfg)}
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

	githubRepos, err := fetchGitHubRepos(ctx, g.fetcher, trace.Value)
	if err == nil {
		newTraces = append(newTraces, githubRepos...)
	}

	bitbucketRepos, err := fetchBitbucketRepos(ctx, g.fetcher, trace.Value)
	if err == nil {
		newTraces = append(newTraces, bitbucketRepos...)
	}

	gitlabRepos, err := fetchGitLabRepos(ctx, g.fetcher, trace.Value)
	if err == nil {
		newTraces = append(newTraces, gitlabRepos...)
	}

	return newTraces, nil
}

func fetchGitHubRepos(ctx context.Context, fetcher repoFetcher, username string) ([]entities.Trace, error) {
	url := fmt.Sprintf("https://api.github.com/users/%s/repos", username)
	resp, err := fetcher.Get(ctx, url)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	var repos []GitHubRepo
	if err := json.NewDecoder(resp.Body).Decode(&repos); err != nil {
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
	url := fmt.Sprintf("https://api.bitbucket.org/2.0/repositories/%s", username)
	resp, err := fetcher.Get(ctx, url)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	var result struct {
		Repos []BitbucketRepo `json:"values"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
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
	url := fmt.Sprintf("https://gitlab.com/api/v4/users/%s/projects", username)
	resp, err := fetcher.Get(ctx, url)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	var repos []GitLabRepo
	if err := json.NewDecoder(resp.Body).Decode(&repos); err != nil {
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
