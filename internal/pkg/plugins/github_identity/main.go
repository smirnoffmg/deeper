package github_identity

import (
	"context"

	"github.com/smirnoffmg/deeper/internal/pkg/config"
	"github.com/smirnoffmg/deeper/internal/pkg/entities"
	deeperhttp "github.com/smirnoffmg/deeper/internal/pkg/http"
	"github.com/smirnoffmg/deeper/internal/pkg/plugins"
)

type GitHubIdentityPlugin struct {
	fetcher commitFetcher
	token   string
}

func NewPlugin(cfg *config.Config) *GitHubIdentityPlugin {
	return &GitHubIdentityPlugin{
		fetcher: deeperhttp.NewClient(cfg),
		token:   cfg.GitHubToken,
	}
}

func (p *GitHubIdentityPlugin) Register(r plugins.Registry) error {
	r.Add(entities.Github, p)
	r.Add(entities.Repository, p)
	return nil
}

func (p *GitHubIdentityPlugin) FollowTrace(ctx context.Context, trace entities.Trace) ([]entities.Trace, error) {
	if trace.Type != entities.Github && trace.Type != entities.Repository {
		return nil, nil
	}

	owner, repo, ok := parseOwnerRepo(trace.Value)
	if !ok {
		return nil, nil
	}

	if isFork(ctx, p.fetcher, owner, repo, p.token) {
		return nil, nil
	}

	authors, err := fetchCommitAuthors(ctx, p.fetcher, owner, repo, p.token)
	if err != nil {
		return nil, err
	}
	authors = filterAuthorsForSharedRepo(authors, owner)
	if len(authors) == 0 {
		return nil, nil
	}

	return authorsToTraces(authors), nil
}

func (p *GitHubIdentityPlugin) String() string {
	return "GitHubIdentityPlugin"
}
