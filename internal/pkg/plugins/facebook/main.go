package facebook

import (
	"context"

	"github.com/smirnoffmg/deeper/internal/pkg/config"
	"github.com/smirnoffmg/deeper/internal/pkg/entities"
	deeperhttp "github.com/smirnoffmg/deeper/internal/pkg/http"
	"github.com/smirnoffmg/deeper/internal/pkg/plugins"
)

type FacebookPlugin struct {
	fetcher searchFetcher
}

func NewPlugin(cfg *config.Config) *FacebookPlugin {
	return &FacebookPlugin{fetcher: deeperhttp.NewClient(cfg)}
}

func (g *FacebookPlugin) Register(r plugins.Registry) error {
	r.Add(entities.Username, g)
	r.Add(entities.Name, g)
	return nil
}

func (g *FacebookPlugin) FollowTrace(ctx context.Context, trace entities.Trace) ([]entities.Trace, error) {
	findings, err := g.FollowEvidence(ctx, trace)
	if err != nil {
		return nil, err
	}
	var traces []entities.Trace
	for _, f := range findings {
		traces = append(traces, f.Trace)
	}
	return traces, nil
}

func (g *FacebookPlugin) FollowEvidence(ctx context.Context, trace entities.Trace) ([]plugins.Finding, error) {
	if trace.Type != entities.Username && trace.Type != entities.Name {
		return nil, nil
	}

	profiles, err := searchFacebookProfiles(ctx, g.fetcher, trace.Value)
	if err != nil {
		return nil, err
	}

	var findings []plugins.Finding
	for _, profile := range profiles {
		findings = append(findings, plugins.Finding{
			Trace:    entities.Trace{Value: profile, Type: entities.Url},
			Strength: entities.StrengthWeak,
			Basis:    "search hit: " + trace.Value + " site:facebook.com",
		})
	}
	return findings, nil
}

func (g FacebookPlugin) String() string {
	return "FacebookPlugin"
}
