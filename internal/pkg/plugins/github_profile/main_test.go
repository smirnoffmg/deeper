package github_profile

import (
	"context"
	"net/http"
	"testing"

	"github.com/smirnoffmg/deeper/internal/pkg/config"
	"github.com/smirnoffmg/deeper/internal/pkg/entities"
	"github.com/smirnoffmg/deeper/internal/pkg/plugins"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFollowTrace_WrongType(t *testing.T) {
	p := &GitHubProfilePlugin{fetcher: &fakeProfileFetcher{}}

	traces, err := p.FollowTrace(context.Background(), entities.Trace{Type: entities.Domain, Value: "example.com"})
	require.NoError(t, err)
	assert.Nil(t, traces)
}

func TestFollowTrace_ValidUsername(t *testing.T) {
	fetcher := &fakeProfileFetcher{
		responses: map[string]fakeResponse{
			profileURL("alsmirn"): {status: http.StatusOK, body: `{"company": "CodeScoring", "blog": "https://codescoring.com"}`},
		},
	}
	p := &GitHubProfilePlugin{fetcher: fetcher}

	traces, err := p.FollowTrace(context.Background(), entities.Trace{Type: entities.Username, Value: "alsmirn"})
	require.NoError(t, err)
	require.Len(t, traces, 2)
}

func TestFollowTrace_SetsAuthorizationWhenTokenPresent(t *testing.T) {
	fetcher := &fakeProfileFetcher{}
	p := &GitHubProfilePlugin{fetcher: fetcher, token: "ghp_secret"}

	_, err := p.FollowTrace(context.Background(), entities.Trace{Type: entities.Username, Value: "alsmirn"})
	require.NoError(t, err)
	require.NotEmpty(t, fetcher.reqs)
	for _, req := range fetcher.reqs {
		assert.Equal(t, "Bearer ghp_secret", req.Header.Get("Authorization"), req.URL.String())
	}
}

func TestFollowTrace_NoAuthorizationWhenTokenAbsent(t *testing.T) {
	fetcher := &fakeProfileFetcher{}
	p := &GitHubProfilePlugin{fetcher: fetcher}

	_, err := p.FollowTrace(context.Background(), entities.Trace{Type: entities.Username, Value: "alsmirn"})
	require.NoError(t, err)
	require.NotEmpty(t, fetcher.reqs)
	for _, req := range fetcher.reqs {
		assert.Empty(t, req.Header.Get("Authorization"), req.URL.String())
	}
}

func TestRegister_RegistersUnderUsername(t *testing.T) {
	p := NewPlugin(config.DefaultConfig())
	registry := plugins.Registry{}
	require.NoError(t, p.Register(registry))

	found := false
	for _, registered := range registry[entities.Username] {
		if registered == p {
			found = true
		}
	}
	assert.True(t, found)
}

func TestString(t *testing.T) {
	assert.Equal(t, "GitHubProfilePlugin", (&GitHubProfilePlugin{}).String())
}
