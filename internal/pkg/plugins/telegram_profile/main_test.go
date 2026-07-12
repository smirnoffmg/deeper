package telegram_profile

import (
	"net/http"
	"testing"

	"github.com/smirnoffmg/deeper/internal/pkg/entities"
	"github.com/smirnoffmg/deeper/internal/pkg/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFollowTrace_WrongType(t *testing.T) {
	p := &TelegramProfilePlugin{fetcher: &fakePageFetcher{}}

	traces, err := p.FollowTrace(entities.Trace{Type: entities.Domain, Value: "example.com"})
	require.NoError(t, err)
	assert.Nil(t, traces)
}

func TestFollowTrace_NonTelegramSocialGenericIgnored(t *testing.T) {
	p := &TelegramProfilePlugin{fetcher: &fakePageFetcher{}}

	traces, err := p.FollowTrace(entities.Trace{Type: entities.SocialGeneric, Value: "https://keybase.io/alsmirn"})
	require.NoError(t, err)
	assert.Nil(t, traces)
}

func TestFollowTrace_TelegramURL(t *testing.T) {
	fetcher := &fakePageFetcher{
		responses: map[string]fakeResponse{
			channelURL("codescoring"): {status: http.StatusOK, body: realFixture},
		},
	}
	p := &TelegramProfilePlugin{fetcher: fetcher}

	traces, err := p.FollowTrace(entities.Trace{Type: entities.SocialGeneric, Value: "https://t.me/codescoring"})
	require.NoError(t, err)
	require.NotEmpty(t, traces)
}

func TestMatches_TelegramURL(t *testing.T) {
	p := &TelegramProfilePlugin{}
	assert.True(t, p.Matches(entities.Trace{Type: entities.SocialGeneric, Value: "https://t.me/codescoring"}))
}

func TestMatches_OtherPlatformURL(t *testing.T) {
	p := &TelegramProfilePlugin{}
	assert.False(t, p.Matches(entities.Trace{Type: entities.SocialGeneric, Value: "https://keybase.io/alsmirn"}))
}

func TestMatches_WrongTraceType(t *testing.T) {
	p := &TelegramProfilePlugin{}
	assert.False(t, p.Matches(entities.Trace{Type: entities.Domain, Value: "t.me"}))
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
	assert.Equal(t, "TelegramProfilePlugin", (&TelegramProfilePlugin{}).String())
}
