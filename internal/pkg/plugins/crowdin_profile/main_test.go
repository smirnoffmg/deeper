package crowdin_profile

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
	p := &CrowdinProfilePlugin{fetcher: &fakePageFetcher{}}

	traces, err := p.FollowTrace(context.Background(), entities.Trace{Type: entities.Domain, Value: "example.com"})
	require.NoError(t, err)
	assert.Nil(t, traces)
}

func TestFollowTrace_NonCrowdinSocialGenericIgnored(t *testing.T) {
	p := &CrowdinProfilePlugin{fetcher: &fakePageFetcher{}}

	traces, err := p.FollowTrace(context.Background(), entities.Trace{Type: entities.SocialGeneric, Value: "https://keybase.io/alsmirn"})
	require.NoError(t, err)
	assert.Nil(t, traces)
}

func TestFollowTrace_CrowdinURL(t *testing.T) {
	fetcher := &fakePageFetcher{
		responses: map[string]fakeResponse{
			profileURL("alsmirn"): {status: http.StatusOK, body: "<html><head><title>Alexey Smirnov (alsmirn) – Crowdin</title></head></html>"},
		},
	}
	p := &CrowdinProfilePlugin{fetcher: fetcher}

	traces, err := p.FollowTrace(context.Background(), entities.Trace{Type: entities.SocialGeneric, Value: "https://crowdin.com/profile/alsmirn"})
	require.NoError(t, err)
	require.NotEmpty(t, traces)
}

func TestMatches_CrowdinURL(t *testing.T) {
	p := &CrowdinProfilePlugin{}
	assert.True(t, p.Matches(entities.Trace{Type: entities.SocialGeneric, Value: "https://crowdin.com/profile/alsmirn"}))
}

func TestMatches_OtherPlatformURL(t *testing.T) {
	p := &CrowdinProfilePlugin{}
	assert.False(t, p.Matches(entities.Trace{Type: entities.SocialGeneric, Value: "https://keybase.io/alsmirn"}))
}

func TestMatches_WrongTraceType(t *testing.T) {
	p := &CrowdinProfilePlugin{}
	assert.False(t, p.Matches(entities.Trace{Type: entities.Domain, Value: "crowdin.com"}))
}

func TestRegister_RegistersUnderSocialGeneric(t *testing.T) {
	p := NewPlugin(config.DefaultConfig())
	registry := plugins.Registry{}
	require.NoError(t, p.Register(registry))

	found := false
	for _, registered := range registry[entities.SocialGeneric] {
		if registered == p {
			found = true
		}
	}
	assert.True(t, found)
}

func TestString(t *testing.T) {
	assert.Equal(t, "CrowdinProfilePlugin", (&CrowdinProfilePlugin{}).String())
}
