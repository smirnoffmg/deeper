package launchpad_profile

import (
	"net/http"
	"testing"

	"github.com/smirnoffmg/deeper/internal/pkg/entities"
	"github.com/smirnoffmg/deeper/internal/pkg/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFollowTrace_WrongType(t *testing.T) {
	p := &LaunchpadProfilePlugin{fetcher: &fakePageFetcher{}}

	traces, err := p.FollowTrace(entities.Trace{Type: entities.Domain, Value: "example.com"})
	require.NoError(t, err)
	assert.Nil(t, traces)
}

func TestFollowTrace_NonLaunchpadSocialGenericIgnored(t *testing.T) {
	p := &LaunchpadProfilePlugin{fetcher: &fakePageFetcher{}}

	traces, err := p.FollowTrace(entities.Trace{Type: entities.SocialGeneric, Value: "https://keybase.io/alsmirn"})
	require.NoError(t, err)
	assert.Nil(t, traces)
}

func TestFollowTrace_LaunchpadURL(t *testing.T) {
	fetcher := &fakePageFetcher{
		responses: map[string]fakeResponse{
			profileURL("lig"): {status: http.StatusOK, body: `<html><head><meta property="og:title" content="Serge Matveenko in Launchpad" /></head></html>`},
		},
	}
	p := &LaunchpadProfilePlugin{fetcher: fetcher}

	traces, err := p.FollowTrace(entities.Trace{Type: entities.SocialGeneric, Value: "https://launchpad.net/~lig"})
	require.NoError(t, err)
	require.NotEmpty(t, traces)
}

func TestMatches_LaunchpadURL(t *testing.T) {
	p := &LaunchpadProfilePlugin{}
	assert.True(t, p.Matches(entities.Trace{Type: entities.SocialGeneric, Value: "https://launchpad.net/~lig"}))
}

func TestMatches_OtherPlatformURL(t *testing.T) {
	p := &LaunchpadProfilePlugin{}
	assert.False(t, p.Matches(entities.Trace{Type: entities.SocialGeneric, Value: "https://keybase.io/alsmirn"}))
}

func TestMatches_WrongTraceType(t *testing.T) {
	p := &LaunchpadProfilePlugin{}
	assert.False(t, p.Matches(entities.Trace{Type: entities.Domain, Value: "launchpad.net"}))
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
	assert.Equal(t, "LaunchpadProfilePlugin", (&LaunchpadProfilePlugin{}).String())
}
