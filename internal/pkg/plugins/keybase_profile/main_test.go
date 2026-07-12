package keybase_profile

import (
	"net/http"
	"testing"

	"github.com/smirnoffmg/deeper/internal/pkg/entities"
	"github.com/smirnoffmg/deeper/internal/pkg/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFollowTrace_WrongType(t *testing.T) {
	p := &KeybaseProfilePlugin{fetcher: &fakeProfileFetcher{}}

	traces, err := p.FollowTrace(entities.Trace{Type: entities.Domain, Value: "example.com"})
	require.NoError(t, err)
	assert.Nil(t, traces)
}

func TestFollowTrace_NonKeybaseSocialGenericIgnored(t *testing.T) {
	p := &KeybaseProfilePlugin{fetcher: &fakeProfileFetcher{}}

	traces, err := p.FollowTrace(entities.Trace{Type: entities.SocialGeneric, Value: "https://github.com/alsmirn"})
	require.NoError(t, err)
	assert.Nil(t, traces)
}

func TestFollowTrace_KeybaseURL(t *testing.T) {
	fetcher := &fakeProfileFetcher{
		responses: map[string]fakeResponse{
			lookupURL("lig"): {status: http.StatusOK, body: ligFixture},
		},
	}
	p := &KeybaseProfilePlugin{fetcher: fetcher}

	traces, err := p.FollowTrace(entities.Trace{Type: entities.SocialGeneric, Value: "https://keybase.io/lig"})
	require.NoError(t, err)
	require.NotEmpty(t, traces)
}

func TestMatches_KeybaseURL(t *testing.T) {
	p := &KeybaseProfilePlugin{}
	assert.True(t, p.Matches(entities.Trace{Type: entities.SocialGeneric, Value: "https://keybase.io/lig"}))
}

func TestMatches_OtherPlatformURL(t *testing.T) {
	p := &KeybaseProfilePlugin{}
	assert.False(t, p.Matches(entities.Trace{Type: entities.SocialGeneric, Value: "https://github.com/alsmirn"}))
}

func TestMatches_WrongTraceType(t *testing.T) {
	p := &KeybaseProfilePlugin{}
	assert.False(t, p.Matches(entities.Trace{Type: entities.Domain, Value: "keybase.io"}))
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
	assert.Equal(t, "KeybaseProfilePlugin", (&KeybaseProfilePlugin{}).String())
}
