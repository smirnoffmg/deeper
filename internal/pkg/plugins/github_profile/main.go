package github_profile

import (
	"context"

	"github.com/smirnoffmg/deeper/internal/pkg/config"
	"github.com/smirnoffmg/deeper/internal/pkg/entities"
	deeperhttp "github.com/smirnoffmg/deeper/internal/pkg/http"
	"github.com/smirnoffmg/deeper/internal/pkg/plugins"
)

type GitHubProfilePlugin struct {
	fetcher profileFetcher
	token   string
}

func NewPlugin(cfg *config.Config) *GitHubProfilePlugin {
	return &GitHubProfilePlugin{fetcher: deeperhttp.NewClient(cfg), token: cfg.GitHubToken}
}

func (p *GitHubProfilePlugin) Register(r plugins.Registry) error {
	r.Add(entities.Github, p)
	return nil
}

func (p *GitHubProfilePlugin) Matches(trace entities.Trace) bool {
	_, ok := accountLogin(trace)
	return ok
}

func accountLogin(trace entities.Trace) (string, bool) {
	if trace.Type != entities.Github {
		return "", false
	}
	return githubLogin(trace.Value)
}

func (p *GitHubProfilePlugin) FollowTrace(ctx context.Context, trace entities.Trace) ([]entities.Trace, error) {
	login, ok := accountLogin(trace)
	if !ok {
		return nil, nil
	}
	return fetchProfile(ctx, p.fetcher, login, p.token)
}

func (p GitHubProfilePlugin) String() string {
	return "GitHubProfilePlugin"
}
