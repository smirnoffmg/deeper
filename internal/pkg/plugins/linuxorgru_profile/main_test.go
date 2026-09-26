package linuxorgru_profile

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
	p := &LinuxOrgRuProfilePlugin{fetcher: &fakePageFetcher{}}

	traces, err := p.FollowTrace(context.Background(), entities.Trace{Type: entities.Domain, Value: "example.com"})
	require.NoError(t, err)
	assert.Nil(t, traces)
}

func TestFollowTrace_NonMatchingSocialGenericIgnored(t *testing.T) {
	p := &LinuxOrgRuProfilePlugin{fetcher: &fakePageFetcher{}}

	traces, err := p.FollowTrace(context.Background(), entities.Trace{Type: entities.SocialGeneric, Value: "https://keybase.io/alsmirn"})
	require.NoError(t, err)
	assert.Nil(t, traces)
}

func TestFollowTrace_MatchingURL(t *testing.T) {
	fetcher := &fakePageFetcher{
		responses: map[string]fakeResponse{
			profileURL("alsmirn"): {status: http.StatusOK, body: realFixture},
		},
	}
	p := &LinuxOrgRuProfilePlugin{fetcher: fetcher}

	traces, err := p.FollowTrace(context.Background(), entities.Trace{Type: entities.SocialGeneric, Value: "https://www.linux.org.ru/people/alsmirn/profile"})
	require.NoError(t, err)
	require.NotEmpty(t, traces)
}

func TestMatches_LinuxOrgRuURL(t *testing.T) {
	p := &LinuxOrgRuProfilePlugin{}
	assert.True(t, p.Matches(entities.Trace{Type: entities.SocialGeneric, Value: "https://www.linux.org.ru/people/alsmirn/profile"}))
}

func TestMatches_OtherPlatformURL(t *testing.T) {
	p := &LinuxOrgRuProfilePlugin{}
	assert.False(t, p.Matches(entities.Trace{Type: entities.SocialGeneric, Value: "https://keybase.io/alsmirn"}))
}

func TestMatches_WrongTraceType(t *testing.T) {
	p := &LinuxOrgRuProfilePlugin{}
	assert.False(t, p.Matches(entities.Trace{Type: entities.Domain, Value: "linux.org.ru"}))
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
	assert.Equal(t, "LinuxOrgRuProfilePlugin", (&LinuxOrgRuProfilePlugin{}).String())
}
