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
	r.Add(entities.Username, p)
	return nil
}

func (p *GitHubProfilePlugin) FollowTrace(ctx context.Context, trace entities.Trace) ([]entities.Trace, error) {
	if trace.Type != entities.Username {
		return nil, nil
	}
	return fetchProfile(ctx, p.fetcher, trace.Value, p.token)
}

func (p GitHubProfilePlugin) String() string {
	return "GitHubProfilePlugin"
}
