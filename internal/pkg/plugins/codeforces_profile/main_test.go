package codeforces_profile

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
	p := &CodeforcesProfilePlugin{fetcher: &fakeProfileFetcher{}}

	traces, err := p.FollowTrace(context.Background(), entities.Trace{Type: entities.Domain, Value: "example.com"})
	require.NoError(t, err)
	assert.Nil(t, traces)
}

func TestFollowTrace_NonCodeforcesSocialGenericIgnored(t *testing.T) {
	p := &CodeforcesProfilePlugin{fetcher: &fakeProfileFetcher{}}

	traces, err := p.FollowTrace(context.Background(), entities.Trace{Type: entities.SocialGeneric, Value: "https://keybase.io/alsmirn"})
	require.NoError(t, err)
	assert.Nil(t, traces)
}

func TestFollowTrace_CodeforcesURL(t *testing.T) {
	fetcher := &fakeProfileFetcher{
		responses: map[string]fakeResponse{
			infoURL("tourist"): {status: http.StatusOK, body: touristFixture},
		},
	}
	p := &CodeforcesProfilePlugin{fetcher: fetcher}

	traces, err := p.FollowTrace(context.Background(), entities.Trace{Type: entities.SocialGeneric, Value: "https://codeforces.com/profile/tourist"})
	require.NoError(t, err)
	require.NotEmpty(t, traces)
}

func TestMatches_CodeforcesURL(t *testing.T) {
	p := &CodeforcesProfilePlugin{}
	assert.True(t, p.Matches(entities.Trace{Type: entities.SocialGeneric, Value: "https://codeforces.com/profile/tourist"}))
}

func TestMatches_OtherPlatformURL(t *testing.T) {
	p := &CodeforcesProfilePlugin{}
	assert.False(t, p.Matches(entities.Trace{Type: entities.SocialGeneric, Value: "https://keybase.io/alsmirn"}))
}

func TestMatches_WrongTraceType(t *testing.T) {
	p := &CodeforcesProfilePlugin{}
	assert.False(t, p.Matches(entities.Trace{Type: entities.Domain, Value: "codeforces.com"}))
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
	assert.Equal(t, "CodeforcesProfilePlugin", (&CodeforcesProfilePlugin{}).String())
}
