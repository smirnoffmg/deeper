// Package namesake records the guess that a bare handle names the same
// person's account on a platform. The guess is weak by construction: the
// account existing proves only that someone registered the handle there.
package namesake

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/smirnoffmg/deeper/internal/pkg/config"
	"github.com/smirnoffmg/deeper/internal/pkg/entities"
	deeperhttp "github.com/smirnoffmg/deeper/internal/pkg/http"
	"github.com/smirnoffmg/deeper/internal/pkg/plugins"
)

type accountFetcher interface {
	Do(req *http.Request) (*http.Response, error)
}

type NamesakePlugin struct {
	fetcher     accountFetcher
	githubToken string
}

func NewPlugin(cfg *config.Config) *NamesakePlugin {
	return &NamesakePlugin{fetcher: deeperhttp.NewClient(cfg), githubToken: cfg.GitHubToken}
}

func (p *NamesakePlugin) Register(r plugins.Registry) error {
	r.Add(entities.Username, p)
	return nil
}

var (
	githubHandle = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9-]{0,38}$`)
	habrHandle   = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
)

func (p *NamesakePlugin) FollowTrace(ctx context.Context, trace entities.Trace) ([]entities.Trace, error) {
	findings, err := p.FollowEvidence(ctx, trace)
	if err != nil {
		return nil, err
	}
	var traces []entities.Trace
	for _, f := range findings {
		traces = append(traces, f.Trace)
	}
	return traces, nil
}

func (p *NamesakePlugin) FollowEvidence(ctx context.Context, trace entities.Trace) ([]plugins.Finding, error) {
	if trace.Type != entities.Username {
		return nil, nil
	}

	h := trace.Value
	var findings []plugins.Finding
	var errs []error
	probe := func(name, probeURL, token string, account entities.Trace, basis string) {
		exists, err := accountExists(ctx, p.fetcher, probeURL, token)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", name, err))
			return
		}
		if exists {
			findings = append(findings, plugins.Finding{Trace: account, Strength: entities.StrengthWeak, Basis: basis})
		}
	}

	if githubHandle.MatchString(h) {
		probe("github", "https://api.github.com/users/"+h, p.githubToken,
			entities.Trace{Type: entities.Github, Value: "https://github.com/" + strings.ToLower(h)},
			"namesake: github.com/"+h+" exists")
	}
	if habrHandle.MatchString(h) {
		probe("habr", "https://habr.com/kek/v2/users/"+h+"/card", "",
			entities.Trace{Type: entities.SocialGeneric, Value: "https://habr.com/ru/users/" + h},
			"namesake: habr.com/users/"+h+" exists")
	}

	// The processor drops every trace when a plugin errors, so one platform
	// failing must not discard an account the other one confirmed.
	if len(findings) > 0 {
		for _, err := range errs {
			log.Warn().Err(err).Str("username", trace.Value).Msg("namesake account check failed, skipping")
		}
		return findings, nil
	}
	return nil, errors.Join(errs...)
}

func accountExists(ctx context.Context, fetcher accountFetcher, probeURL, token string) (bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, probeURL, nil)
	if err != nil {
		return false, err
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := fetcher.Do(req)
	if err != nil {
		return false, err
	}
	defer func() { _ = resp.Body.Close() }()

	switch {
	case resp.StatusCode == http.StatusNotFound:
		return false, nil
	case resp.StatusCode < 200 || resp.StatusCode >= 300:
		return false, fmt.Errorf("account check failed: status %d", resp.StatusCode)
	}
	return true, nil
}

func (p NamesakePlugin) String() string {
	return "NamesakePlugin"
}
