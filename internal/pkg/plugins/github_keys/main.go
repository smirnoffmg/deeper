package github_keys

import (
	"context"

	"github.com/rs/zerolog/log"
	"github.com/smirnoffmg/deeper/internal/pkg/config"
	"github.com/smirnoffmg/deeper/internal/pkg/entities"
	deeperhttp "github.com/smirnoffmg/deeper/internal/pkg/http"
	"github.com/smirnoffmg/deeper/internal/pkg/plugins"
)

type GitHubKeysPlugin struct {
	fetcher keyFetcher
	token   string
}

func NewPlugin(cfg *config.Config) *GitHubKeysPlugin {
	return &GitHubKeysPlugin{fetcher: deeperhttp.NewClient(cfg), token: cfg.GitHubToken}
}

func (p *GitHubKeysPlugin) Register(r plugins.Registry) error {
	r.Add(entities.Github, p)
	return nil
}

func (p *GitHubKeysPlugin) Matches(trace entities.Trace) bool {
	_, ok := accountLogin(trace)
	return ok
}

func accountLogin(trace entities.Trace) (string, bool) {
	if trace.Type != entities.Github {
		return "", false
	}
	return githubLogin(trace.Value)
}

// FollowTrace fetches SSH and GPG keys independently — one failing (rate
// limit, network error) must not block the other, same discipline as
// dns_records' independent per-record-type lookups.
func (p *GitHubKeysPlugin) FollowTrace(ctx context.Context, trace entities.Trace) ([]entities.Trace, error) {
	login, ok := accountLogin(trace)
	if !ok {
		return nil, nil
	}

	var traces []entities.Trace

	sshTraces, err := fetchSSHKeys(ctx, p.fetcher, login, p.token)
	if err != nil {
		log.Warn().Err(err).Str("username", login).Msg("SSH keys lookup failed, skipping")
	} else {
		traces = append(traces, sshTraces...)
	}

	gpgTraces, err := fetchGPGKeys(ctx, p.fetcher, login, p.token)
	if err != nil {
		log.Warn().Err(err).Str("username", login).Msg("GPG keys lookup failed, skipping")
	} else {
		traces = append(traces, gpgTraces...)
	}

	return traces, nil
}

func (p GitHubKeysPlugin) String() string {
	return "GitHubKeysPlugin"
}
