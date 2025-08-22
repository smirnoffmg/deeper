package coderepos

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/rs/zerolog/log"
	"github.com/smirnoffmg/deeper/internal/pkg/entities"
	"github.com/smirnoffmg/deeper/internal/pkg/state"
)

const InputTraceType = entities.Username

func init() {
	p := NewPlugin()
	if err := p.Register(); err != nil {
		log.Error().Err(err).Msgf("Failed to register plugin %s", p)
	}
}

type CodeRepositoriesPlugin struct{}

func NewPlugin() *CodeRepositoriesPlugin {
	return &CodeRepositoriesPlugin{}
}

func (g *CodeRepositoriesPlugin) Register() error {
	state.RegisterPlugin(InputTraceType, g)
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

func (g *CodeRepositoriesPlugin) FollowTrace(trace entities.Trace) ([]entities.Trace, error) {
	if trace.Type != InputTraceType {
		return nil, nil
	}

	var newTraces []entities.Trace

	githubRepos, err := fetchGitHubRepos(trace.Value)
	if err == nil {
		newTraces = append(newTraces, githubRepos...)
	}

	bitbucketRepos, err := fetchBitbucketRepos(trace.Value)
	if err == nil {
		newTraces = append(newTraces, bitbucketRepos...)
	}

	gitlabRepos, err := fetchGitLabRepos(trace.Value)
	if err == nil {
		newTraces = append(newTraces, gitlabRepos...)
	}

	return newTraces, nil
}

func fetchGitHubRepos(username string) ([]entities.Trace, error) {
	url := fmt.Sprintf("https://api.github.com/users/%s/repos", username)
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

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

func fetchBitbucketRepos(username string) ([]entities.Trace, error) {
	url := fmt.Sprintf("https://api.bitbucket.org/2.0/repositories/%s", username)
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

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

func fetchGitLabRepos(username string) ([]entities.Trace, error) {
	url := fmt.Sprintf("https://gitlab.com/api/v4/users/%s/projects", username)
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

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
