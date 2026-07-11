package github_keys

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

type GitHubKeysPlugin struct {
	fetcher keyFetcher
}

func NewPlugin() *GitHubKeysPlugin {
	return &GitHubKeysPlugin{fetcher: deeperhttp.NewClient(config.LoadConfig())}
}

func (p *GitHubKeysPlugin) Register() error {
	state.RegisterPlugin(entities.Username, p)
	return nil
}

// FollowTrace fetches SSH and GPG keys independently — one failing (rate
// limit, network error) must not block the other, same discipline as
// dns_records' independent per-record-type lookups.
func (p *GitHubKeysPlugin) FollowTrace(trace entities.Trace) ([]entities.Trace, error) {
	if trace.Type != entities.Username {
		return nil, nil
	}

	ctx := context.Background()
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
