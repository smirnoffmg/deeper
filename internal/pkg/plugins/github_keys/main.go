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
}

func NewPlugin(cfg *config.Config) *GitHubKeysPlugin {
	return &GitHubKeysPlugin{fetcher: deeperhttp.NewClient(cfg)}
}

func (p *GitHubKeysPlugin) Register(r plugins.Registry) error {
	r.Add(entities.Username, p)
	return nil
}

// FollowTrace fetches SSH and GPG keys independently — one failing (rate
// limit, network error) must not block the other, same discipline as
// dns_records' independent per-record-type lookups.
func (p *GitHubKeysPlugin) FollowTrace(ctx context.Context, trace entities.Trace) ([]entities.Trace, error) {
	if trace.Type != entities.Username {
		return nil, nil
	}

	var traces []entities.Trace

	sshTraces, err := fetchSSHKeys(ctx, p.fetcher, trace.Value)
	if err != nil {
		log.Warn().Err(err).Str("username", trace.Value).Msg("SSH keys lookup failed, skipping")
	} else {
		traces = append(traces, sshTraces...)
	}

	gpgTraces, err := fetchGPGKeys(ctx, p.fetcher, trace.Value)
	if err != nil {
		log.Warn().Err(err).Str("username", trace.Value).Msg("GPG keys lookup failed, skipping")
	} else {
		traces = append(traces, gpgTraces...)
	}

	return traces, nil
}

func (p GitHubKeysPlugin) String() string {
	return "GitHubKeysPlugin"
}
