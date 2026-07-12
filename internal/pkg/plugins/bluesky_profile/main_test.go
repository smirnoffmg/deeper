package bluesky_profile

import (
	"net/http"
	"testing"

	"github.com/smirnoffmg/deeper/internal/pkg/entities"
	"github.com/smirnoffmg/deeper/internal/pkg/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFollowTrace_WrongType(t *testing.T) {
	p := &BlueskyProfilePlugin{fetcher: &fakeProfileFetcher{}}

	traces, err := p.FollowTrace(entities.Trace{Type: entities.Domain, Value: "example.com"})
	require.NoError(t, err)
	assert.Nil(t, traces)
}

func TestFollowTrace_NonBlueskySocialGenericIgnored(t *testing.T) {
	p := &BlueskyProfilePlugin{fetcher: &fakeProfileFetcher{}}

	traces, err := p.FollowTrace(entities.Trace{Type: entities.SocialGeneric, Value: "https://keybase.io/alsmirn"})
	require.NoError(t, err)
	assert.Nil(t, traces)
}

func TestFollowTrace_BlueskyURL(t *testing.T) {
	fetcher := &fakeProfileFetcher{
		responses: map[string]fakeResponse{
			profileURL("jay"): {status: http.StatusOK, body: `{"handle":"jay.bsky.team","displayName":"Jay"}`},
		},
	}
	p := &BlueskyProfilePlugin{fetcher: fetcher}

	traces, err := p.FollowTrace(entities.Trace{Type: entities.SocialGeneric, Value: "https://bsky.app/profile/jay.bsky.social"})
	require.NoError(t, err)
	require.NotEmpty(t, traces)
}

func TestMatches_BlueskyURL(t *testing.T) {
	p := &BlueskyProfilePlugin{}
	assert.True(t, p.Matches(entities.Trace{Type: entities.SocialGeneric, Value: "https://bsky.app/profile/lig.bsky.social"}))
}

func TestMatches_OtherPlatformURL(t *testing.T) {
	p := &BlueskyProfilePlugin{}
	assert.False(t, p.Matches(entities.Trace{Type: entities.SocialGeneric, Value: "https://keybase.io/alsmirn"}))
}

func TestMatches_WrongTraceType(t *testing.T) {
	p := &BlueskyProfilePlugin{}
	assert.False(t, p.Matches(entities.Trace{Type: entities.Domain, Value: "bsky.app"}))
}

func TestRegister_RegistersUnderSocialGeneric(t *testing.T) {
	p := NewPlugin()
	require.NoError(t, p.Register())

	found := false
	for _, registered := range state.ActivePlugins[entities.SocialGeneric] {
		if registered == p {
			found = true
		}
	}
	assert.True(t, found)
}

func TestString(t *testing.T) {
	assert.Equal(t, "BlueskyProfilePlugin", (&BlueskyProfilePlugin{}).String())
}
