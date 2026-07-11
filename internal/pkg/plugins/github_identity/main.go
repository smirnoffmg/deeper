package github_identity

import (
	"context"

	"github.com/rs/zerolog/log"
	"github.com/smirnoffmg/deeper/internal/pkg/config"
	"github.com/smirnoffmg/deeper/internal/pkg/entities"
	deeperhttp "github.com/smirnoffmg/deeper/internal/pkg/http"
	"github.com/smirnoffmg/deeper/internal/pkg/state"
)

func init() {
	p := NewPlugin()
	if err := p.Register(); err != nil {
		log.Error().Err(err).Msgf("Failed to register plugin %s", p)
	}
}

type GitHubIdentityPlugin struct {
	fetcher commitFetcher
	token   string
}

func NewPlugin() *GitHubIdentityPlugin {
	cfg := config.LoadConfig()
	return &GitHubIdentityPlugin{
		fetcher: deeperhttp.NewClient(cfg),
		token:   cfg.GitHubToken,
	}
}

func (p *GitHubIdentityPlugin) Register() error {
	state.RegisterPlugin(entities.Github, p)
	state.RegisterPlugin(entities.Repository, p)
	return nil
}

func (p *GitHubIdentityPlugin) FollowTrace(trace entities.Trace) ([]entities.Trace, error) {
	if trace.Type != entities.Github && trace.Type != entities.Repository {
		return nil, nil
	}

	owner, repo, ok := parseOwnerRepo(trace.Value)
	if !ok {
		return nil, nil
	}

	ctx := context.Background()

	if isFork(ctx, p.fetcher, owner, repo, p.token) {
		return nil, nil
	}

	authors, err := fetchCommitAuthors(ctx, p.fetcher, owner, repo, p.token)
	if err != nil {
		return nil, err
	}
	if len(authors) == 0 {
		return nil, nil
	}

	return authorsToTraces(authors), nil
}

func (p *GitHubIdentityPlugin) String() string {
	return "GitHubIdentityPlugin"
}
